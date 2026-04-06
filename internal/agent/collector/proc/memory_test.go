package proc

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMeminfo(t *testing.T) {
	f, err := os.Open("testdata/meminfo")
	require.NoError(t, err)
	defer f.Close()

	info, err := parseMeminfo(f)
	require.NoError(t, err)

	assert.Equal(t, uint64(16384000), info["MemTotal"])
	assert.Equal(t, uint64(2048000), info["MemFree"])
	assert.Equal(t, uint64(8192000), info["MemAvailable"])
	assert.Equal(t, uint64(512000), info["Buffers"])
	assert.Equal(t, uint64(4096000), info["Cached"])
	assert.Equal(t, uint64(2048000), info["SwapTotal"])
	assert.Equal(t, uint64(1024000), info["SwapFree"])
}

func TestMemoryCollector(t *testing.T) {
	c := NewMemoryCollector("/unused", "testhost")
	f, err := os.Open("testdata/meminfo")
	require.NoError(t, err)
	defer f.Close()

	metrics, err := c.parse(f, time.Now())
	require.NoError(t, err)

	metricMap := make(map[string]float64)
	for _, m := range metrics {
		metricMap[m.Name] = m.Value
		assert.Equal(t, "testhost", m.Tags["host"])
	}

	assert.Equal(t, float64(16384000)*1024, metricMap["memory.total_bytes"])
	assert.Equal(t, float64(2048000)*1024, metricMap["memory.free_bytes"])
	assert.Equal(t, float64(8192000)*1024, metricMap["memory.available_bytes"])

	// used_percent = (16384000 - 8192000) / 16384000 * 100 = 50%
	assert.InDelta(t, 50.0, metricMap["memory.used_percent"], 0.01)

	// swap_used_percent = (2048000 - 1024000) / 2048000 * 100 = 50%
	assert.InDelta(t, 50.0, metricMap["memory.swap_used_percent"], 0.01)
}
