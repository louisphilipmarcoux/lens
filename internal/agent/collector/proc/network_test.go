package proc

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNetDev(t *testing.T) {
	f, err := os.Open("testdata/net_dev")
	require.NoError(t, err)
	defer f.Close()

	stats, err := parseNetDev(f)
	require.NoError(t, err)

	assert.Len(t, stats, 3)
	assert.Contains(t, stats, "lo")
	assert.Contains(t, stats, "eth0")
	assert.Contains(t, stats, "docker0")

	eth0 := stats["eth0"]
	assert.Equal(t, uint64(9876543210), eth0.RxBytes)
	assert.Equal(t, uint64(654321), eth0.RxPackets)
	assert.Equal(t, uint64(12), eth0.RxErrors)
	assert.Equal(t, uint64(5432109876), eth0.TxBytes)
	assert.Equal(t, uint64(432109), eth0.TxPackets)
	assert.Equal(t, uint64(3), eth0.TxErrors)
}

func TestNetworkCollectorDelta(t *testing.T) {
	c := NewNetworkCollector("/unused", "testhost")
	ts := time.Now()

	data1 := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
  eth0: 1000000 10000 0 0 0 0 0 0 500000 5000 0 0 0 0 0 0`

	data2 := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
  eth0: 1100000 11000 0 0 0 0 0 0 600000 6000 0 0 0 0 0 0`

	m1, err := c.parse(strings.NewReader(data1), ts)
	require.NoError(t, err)
	assert.Empty(t, m1)

	m2, err := c.parse(strings.NewReader(data2), ts.Add(10*time.Second))
	require.NoError(t, err)
	require.NotEmpty(t, m2)

	metricMap := make(map[string]float64)
	for _, m := range m2 {
		metricMap[m.Name] = m.Value
	}

	// rx_bytes_per_sec: 100000 / 10 = 10000
	assert.InDelta(t, 10000, metricMap["net.rx_bytes_per_sec"], 0.1)
	// tx_bytes_per_sec: 100000 / 10 = 10000
	assert.InDelta(t, 10000, metricMap["net.tx_bytes_per_sec"], 0.1)
}
