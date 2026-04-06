package proc

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAvgCollector(t *testing.T) {
	c := NewLoadAvgCollector("/unused", "testhost")

	f, err := os.Open("testdata/loadavg")
	require.NoError(t, err)
	defer f.Close()

	metrics, err := c.parse(f, time.Now())
	require.NoError(t, err)
	require.Len(t, metrics, 5)

	metricMap := make(map[string]float64)
	for _, m := range metrics {
		metricMap[m.Name] = m.Value
		assert.Equal(t, "testhost", m.Tags["host"])
	}

	assert.InDelta(t, 0.72, metricMap["loadavg.1min"], 0.001)
	assert.InDelta(t, 1.23, metricMap["loadavg.5min"], 0.001)
	assert.InDelta(t, 0.95, metricMap["loadavg.15min"], 0.001)
	assert.InDelta(t, 3, metricMap["loadavg.running_processes"], 0.001)
	assert.InDelta(t, 512, metricMap["loadavg.total_processes"], 0.001)
}
