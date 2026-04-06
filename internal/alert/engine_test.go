package alert

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type mockNotifier struct {
	mu     sync.Mutex
	alerts []Alert
}

func (m *mockNotifier) Notify(_ context.Context, alert Alert) error {
	m.mu.Lock()
	m.alerts = append(m.alerts, alert)
	m.mu.Unlock()
	return nil
}

func (m *mockNotifier) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.alerts)
}

func TestEvaluateCondition(t *testing.T) {
	assert.True(t, evaluateCondition(95, ">", 90))
	assert.False(t, evaluateCondition(85, ">", 90))
	assert.True(t, evaluateCondition(90, ">=", 90))
	assert.True(t, evaluateCondition(5, "<", 10))
	assert.True(t, evaluateCondition(0, "==", 0))
	assert.True(t, evaluateCondition(1, "!=", 0))
}

func TestEngineFiresAlert(t *testing.T) {
	notifier := &mockNotifier{}
	queryValue := 95.0

	rules := []Rule{
		{
			Name:      "high_cpu",
			Query:     "cpu.user_percent",
			Condition: ">",
			Threshold: 90,
			For:       0, // fire immediately
			Severity:  SeverityCritical,
			Interval:  100 * time.Millisecond,
			Labels:    map[string]string{"team": "infra"},
		},
	}

	engine := NewEngine(rules, func(_ context.Context, _ string) (float64, error) {
		return queryValue, nil
	}, notifier, zap.NewNop())

	// First eval sets pending, second fires.
	engine.evaluateAll(context.Background())
	engine.evaluateAll(context.Background())

	active := engine.ActiveAlerts()
	require.Len(t, active, 1)
	assert.Equal(t, "high_cpu", active[0].Rule.Name)
	assert.Equal(t, StateFiring, active[0].State)
	assert.Equal(t, 95.0, active[0].Value)

	// Resolve.
	queryValue = 50.0
	engine.evaluateAll(context.Background())

	active = engine.ActiveAlerts()
	assert.Empty(t, active)

	history := engine.AlertHistory()
	assert.Len(t, history, 2) // fired + resolved
}

func TestEngineForDuration(t *testing.T) {
	notifier := &mockNotifier{}

	rules := []Rule{
		{
			Name:      "sustained_high",
			Query:     "cpu.user_percent",
			Condition: ">",
			Threshold: 90,
			For:       200 * time.Millisecond,
			Severity:  SeverityWarning,
			Interval:  50 * time.Millisecond,
		},
	}

	engine := NewEngine(rules, func(_ context.Context, _ string) (float64, error) {
		return 95.0, nil
	}, notifier, zap.NewNop())

	// First eval: starts pending.
	engine.evaluateAll(context.Background())
	assert.Empty(t, engine.ActiveAlerts())

	// Still within For duration.
	engine.evaluateAll(context.Background())
	assert.Empty(t, engine.ActiveAlerts())

	// Wait past For duration.
	time.Sleep(250 * time.Millisecond)
	engine.evaluateAll(context.Background())
	assert.Len(t, engine.ActiveAlerts(), 1)
}

func TestFingerprint(t *testing.T) {
	r1 := Rule{Name: "test", Query: "q", Condition: ">", Threshold: 90}
	r2 := Rule{Name: "test", Query: "q", Condition: ">", Threshold: 90}
	r3 := Rule{Name: "other", Query: "q", Condition: ">", Threshold: 90}

	assert.Equal(t, fingerprint(r1), fingerprint(r2))
	assert.NotEqual(t, fingerprint(r1), fingerprint(r3))
}
