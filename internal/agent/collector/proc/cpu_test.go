package proc

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseProcStat(t *testing.T) {
	data := `cpu  10132153 290696 3084719 46828483 16683 0 25195 0 0 0
cpu0 1393280 32966 572056 13343292 6130 0 17875 0 0 0
intr 1462898
ctxt 3921655`

	stats, err := parseProcStat(strings.NewReader(data))
	require.NoError(t, err)

	assert.Len(t, stats, 2)

	cpu := stats["cpu"]
	assert.Equal(t, uint64(10132153), cpu.User)
	assert.Equal(t, uint64(290696), cpu.Nice)
	assert.Equal(t, uint64(3084719), cpu.System)
	assert.Equal(t, uint64(46828483), cpu.Idle)
	assert.Equal(t, uint64(16683), cpu.IOWait)
	assert.Equal(t, uint64(0), cpu.Steal)

	cpu0 := stats["cpu0"]
	assert.Equal(t, uint64(1393280), cpu0.User)
}

func TestCPUCollectorDelta(t *testing.T) {
	c := NewCPUCollector("/unused", "testhost")
	ts := time.Now()

	// First read populates prev, returns no metrics.
	r1 := strings.NewReader(`cpu  1000 100 200 5000 50 0 10 0 0 0`)
	m1, err := c.parse(r1, ts)
	require.NoError(t, err)
	assert.Empty(t, m1)

	// Second read computes deltas.
	r2 := strings.NewReader(`cpu  1100 100 300 5400 50 0 10 0 0 0`)
	m2, err := c.parse(r2, ts.Add(10*time.Second))
	require.NoError(t, err)

	require.NotEmpty(t, m2)

	// Total delta = (1100-1000) + (300-200) + (5400-5000) = 100 + 100 + 400 = 600
	metricMap := make(map[string]float64)
	for _, m := range m2 {
		metricMap[m.Name] = m.Value
		assert.Equal(t, "testhost", m.Tags["host"])
		assert.Equal(t, "cpu", m.Tags["cpu"])
	}

	// user: 100/600 * 100 = 16.666...
	assert.InDelta(t, 16.67, metricMap["cpu.user_percent"], 0.1)
	// system: 100/600 * 100 = 16.666...
	assert.InDelta(t, 16.67, metricMap["cpu.system_percent"], 0.1)
	// idle: 400/600 * 100 = 66.666...
	assert.InDelta(t, 66.67, metricMap["cpu.idle_percent"], 0.1)
}
