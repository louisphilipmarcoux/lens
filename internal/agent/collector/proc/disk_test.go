package proc

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDiskStats(t *testing.T) {
	data := `   8       0 sda 55847 2523 2621962 27820 43673 20674 1714872 36880 0 29540 64700 0 0 0 0
   7       0 loop0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0`

	stats, err := parseDiskStats(strings.NewReader(data))
	require.NoError(t, err)

	// loop0 should be filtered out.
	assert.Len(t, stats, 1)
	assert.Contains(t, stats, "sda")

	sda := stats["sda"]
	assert.Equal(t, uint64(55847), sda.ReadsCompleted)
	assert.Equal(t, uint64(2621962), sda.SectorsRead)
	assert.Equal(t, uint64(43673), sda.WritesCompleted)
	assert.Equal(t, uint64(1714872), sda.SectorsWritten)
}

func TestDiskCollectorDelta(t *testing.T) {
	c := NewDiskCollector("/unused", "testhost")
	ts := time.Now()

	r1 := strings.NewReader(`   8       0 sda 1000 0 100000 0 500 0 50000 0 0 1000 2000 0 0 0 0`)
	m1, err := c.parse(r1, ts)
	require.NoError(t, err)
	assert.Empty(t, m1)

	r2 := strings.NewReader(`   8       0 sda 1010 0 102000 0 505 0 51000 0 0 1100 2200 0 0 0 0`)
	m2, err := c.parse(r2, ts.Add(10*time.Second))
	require.NoError(t, err)
	require.NotEmpty(t, m2)

	metricMap := make(map[string]float64)
	for _, m := range m2 {
		metricMap[m.Name] = m.Value
	}

	// read: (102000-100000) * 512 / 10 = 102400 bytes/sec
	assert.InDelta(t, 102400, metricMap["disk.read_bytes_per_sec"], 0.1)
	// reads_per_sec: 10/10 = 1
	assert.InDelta(t, 1.0, metricMap["disk.reads_per_sec"], 0.01)
}
