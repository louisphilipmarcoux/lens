package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"time"

	"go.uber.org/zap"
)

// Notifier sends alert notifications.
type Notifier interface {
	Notify(ctx context.Context, alert Alert) error
}

// MultiNotifier fans out notifications to multiple notifiers.
type MultiNotifier struct {
	notifiers []Notifier
	logger    *zap.Logger
}

// NewMultiNotifier creates a notifier that sends to all configured backends.
func NewMultiNotifier(notifiers []Notifier, logger *zap.Logger) *MultiNotifier {
	return &MultiNotifier{notifiers: notifiers, logger: logger}
}

// Notify sends the alert to all notifiers, logging errors but not failing.
func (m *MultiNotifier) Notify(ctx context.Context, alert Alert) error {
	for _, n := range m.notifiers {
		if err := n.Notify(ctx, alert); err != nil {
			m.logger.Error("notifier failed", zap.Error(err))
		}
	}
	return nil
}

// WebhookNotifier sends alerts to a webhook URL.
type WebhookNotifier struct {
	URL    string
	client *http.Client
}

// NewWebhookNotifier creates a webhook notifier.
func NewWebhookNotifier(url string) *WebhookNotifier {
	return &WebhookNotifier{
		URL:    url,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Notify sends the alert as a JSON POST to the webhook URL.
func (w *WebhookNotifier) Notify(ctx context.Context, alert Alert) error {
	payload := map[string]any{
		"rule":        alert.Rule.Name,
		"state":       alert.State,
		"severity":    alert.Rule.Severity,
		"value":       alert.Value,
		"fired_at":    alert.FiredAt,
		"resolved_at": alert.ResolvedAt,
		"labels":      alert.Labels,
		"annotations": alert.Rule.Annotations,
		"fingerprint": alert.Fingerprint,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned %d", resp.StatusCode)
	}
	return nil
}

// EmailNotifier sends alert emails via SMTP.
type EmailNotifier struct {
	SMTPAddr string
	From     string
	To       []string
}

// Notify sends an alert email.
func (e *EmailNotifier) Notify(ctx context.Context, alert Alert) error {
	subject := fmt.Sprintf("[Lens %s] %s - %s", alert.Rule.Severity, alert.Rule.Name, alert.State)
	body := fmt.Sprintf(
		"Alert: %s\nState: %s\nSeverity: %s\nValue: %.2f\nFired At: %s\n\nQuery: %s\nCondition: %s %v",
		alert.Rule.Name, alert.State, alert.Rule.Severity,
		alert.Value, alert.FiredAt.Format(time.RFC3339),
		alert.Rule.Query, alert.Rule.Condition, alert.Rule.Threshold,
	)

	msg := fmt.Sprintf("Subject: %s\r\nFrom: %s\r\nTo: %s\r\n\r\n%s",
		subject, e.From, e.To[0], body)

	return smtp.SendMail(e.SMTPAddr, nil, e.From, e.To, []byte(msg))
}

// LogNotifier logs alerts (useful for testing).
type LogNotifier struct {
	Logger *zap.Logger
}

// Notify logs the alert.
func (l *LogNotifier) Notify(_ context.Context, alert Alert) error {
	l.Logger.Info("alert notification",
		zap.String("rule", alert.Rule.Name),
		zap.String("state", string(alert.State)),
		zap.String("severity", string(alert.Rule.Severity)),
		zap.Float64("value", alert.Value),
	)
	return nil
}
