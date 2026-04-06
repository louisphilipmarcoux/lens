// Package alert implements the alerting engine for Lens.
package alert

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Severity represents alert severity levels.
type Severity string

const (
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// State represents the current state of an alert.
type State string

const (
	StatePending  State = "pending"
	StateFiring   State = "firing"
	StateResolved State = "resolved"
)

// Rule defines an alert rule.
type Rule struct {
	Name        string            `json:"name" yaml:"name"`
	Query       string            `json:"query" yaml:"query"`
	Condition   string            `json:"condition" yaml:"condition"` // "> 90", "< 10", "== 0"
	Threshold   float64           `json:"threshold" yaml:"threshold"`
	For         time.Duration     `json:"for" yaml:"for"` // must be true for this duration
	Severity    Severity          `json:"severity" yaml:"severity"`
	Labels      map[string]string `json:"labels" yaml:"labels"`
	Annotations map[string]string `json:"annotations" yaml:"annotations"`
	Interval    time.Duration     `json:"interval" yaml:"interval"` // evaluation interval
}

// Alert represents a fired alert instance.
type Alert struct {
	Rule        Rule              `json:"rule"`
	State       State             `json:"state"`
	Value       float64           `json:"value"`
	FiredAt     time.Time         `json:"fired_at"`
	ResolvedAt  *time.Time        `json:"resolved_at,omitempty"`
	Labels      map[string]string `json:"labels"`
	Fingerprint string            `json:"fingerprint"`
}

// QueryFunc executes a PromQL query and returns the current value.
type QueryFunc func(ctx context.Context, query string) (float64, error)

// Engine evaluates alert rules on a schedule.
type Engine struct {
	rules    []Rule
	query    QueryFunc
	notifier Notifier
	logger   *zap.Logger

	mu       sync.RWMutex
	active   map[string]*Alert // fingerprint -> alert
	history  []Alert
	pending  map[string]time.Time // fingerprint -> first pending time
}

// NewEngine creates an alerting engine.
func NewEngine(rules []Rule, query QueryFunc, notifier Notifier, logger *zap.Logger) *Engine {
	return &Engine{
		rules:    rules,
		query:    query,
		notifier: notifier,
		logger:   logger,
		active:   make(map[string]*Alert),
		pending:  make(map[string]time.Time),
	}
}

// Run starts the evaluation loop. Blocks until ctx is cancelled.
func (e *Engine) Run(ctx context.Context) {
	// Use the smallest rule interval as the tick.
	interval := 15 * time.Second
	for _, r := range e.rules {
		if r.Interval > 0 && r.Interval < interval {
			interval = r.Interval
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	e.logger.Info("alerting engine started", zap.Int("rules", len(e.rules)), zap.Duration("interval", interval))

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.evaluateAll(ctx)
		}
	}
}

// ActiveAlerts returns currently firing alerts.
func (e *Engine) ActiveAlerts() []Alert {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]Alert, 0, len(e.active))
	for _, a := range e.active {
		result = append(result, *a)
	}
	return result
}

// AlertHistory returns recent alert history.
func (e *Engine) AlertHistory() []Alert {
	e.mu.RLock()
	defer e.mu.RUnlock()
	h := make([]Alert, len(e.history))
	copy(h, e.history)
	return h
}

func (e *Engine) evaluateAll(ctx context.Context) {
	for _, rule := range e.rules {
		e.evaluateRule(ctx, rule)
	}
}

func (e *Engine) evaluateRule(ctx context.Context, rule Rule) {
	value, err := e.query(ctx, rule.Query)
	if err != nil {
		e.logger.Warn("alert query failed", zap.String("rule", rule.Name), zap.Error(err))
		return
	}

	firing := evaluateCondition(value, rule.Condition, rule.Threshold)
	fp := fingerprint(rule)

	e.mu.Lock()
	defer e.mu.Unlock()

	if firing {
		pendingStart, isPending := e.pending[fp]
		if !isPending {
			e.pending[fp] = time.Now()
			return
		}

		// Check if it's been pending long enough.
		if time.Since(pendingStart) < rule.For {
			return
		}

		// Fire or update.
		if existing, ok := e.active[fp]; ok {
			existing.Value = value
			return
		}

		alert := &Alert{
			Rule:        rule,
			State:       StateFiring,
			Value:       value,
			FiredAt:     time.Now(),
			Labels:      rule.Labels,
			Fingerprint: fp,
		}
		e.active[fp] = alert
		alertCopy := *alert
		e.history = append(e.history, alertCopy)

		e.logger.Warn("alert firing",
			zap.String("rule", rule.Name),
			zap.Float64("value", value),
			zap.String("severity", string(rule.Severity)),
		)

		go func(a Alert) { _ = e.notifier.Notify(context.Background(), a) }(alertCopy)

	} else {
		delete(e.pending, fp)

		if existing, ok := e.active[fp]; ok {
			now := time.Now()
			existing.State = StateResolved
			existing.ResolvedAt = &now
			resolvedCopy := *existing
			e.history = append(e.history, resolvedCopy)
			delete(e.active, fp)

			e.logger.Info("alert resolved", zap.String("rule", rule.Name))
			go func(a Alert) { _ = e.notifier.Notify(context.Background(), a) }(resolvedCopy)
		}
	}
}

func evaluateCondition(value float64, condition string, threshold float64) bool {
	switch condition {
	case ">":
		return value > threshold
	case ">=":
		return value >= threshold
	case "<":
		return value < threshold
	case "<=":
		return value <= threshold
	case "==":
		return value == threshold
	case "!=":
		return value != threshold
	default:
		return value > threshold
	}
}

func fingerprint(rule Rule) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%v", rule.Name, rule.Query, rule.Condition, rule.Threshold)))
	return fmt.Sprintf("%x", h[:8])
}

// TrimHistory keeps only the last N entries.
func (e *Engine) TrimHistory(maxEntries int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.history) > maxEntries {
		e.history = e.history[len(e.history)-maxEntries:]
	}
}
