package proc

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/louispm/lens/internal/common/model"
)

// NetDevStats holds counters from /proc/net/dev for a single interface.
type NetDevStats struct {
	RxBytes   uint64
	RxPackets uint64
	RxErrors  uint64
	TxBytes   uint64
	TxPackets uint64
	TxErrors  uint64
}

// NetworkCollector reads /proc/net/dev and computes per-interface network rates.
type NetworkCollector struct {
	procRoot string
	hostname string
	mu       sync.Mutex
	prev     map[string]NetDevStats
	prevTime time.Time
}

func NewNetworkCollector(procRoot, hostname string) *NetworkCollector {
	return &NetworkCollector{
		procRoot: procRoot,
		hostname: hostname,
		prev:     make(map[string]NetDevStats),
	}
}

func (c *NetworkCollector) Name() string { return "network" }

func (c *NetworkCollector) Collect(ctx context.Context) ([]model.Metric, error) {
	f, err := openProcFile(c.procRoot, "net/dev")
	if err != nil {
		return nil, fmt.Errorf("open /proc/net/dev: %w", err)
	}
	defer f.Close()

	now := time.Now()
	return c.parse(f, now)
}

func (c *NetworkCollector) parse(r io.Reader, ts time.Time) ([]model.Metric, error) {
	current, err := parseNetDev(r)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	prev := c.prev
	prevTime := c.prevTime
	c.prev = current
	c.prevTime = ts
	c.mu.Unlock()

	if len(prev) == 0 {
		return nil, nil
	}

	elapsed := ts.Sub(prevTime).Seconds()
	if elapsed <= 0 {
		return nil, nil
	}

	var metrics []model.Metric
	for iface, cur := range current {
		old, ok := prev[iface]
		if !ok {
			continue
		}

		tags := map[string]string{"host": c.hostname, "interface": iface}
		metrics = append(metrics,
			model.Metric{Name: "net.rx_bytes_per_sec", Tags: tags, Value: float64(cur.RxBytes-old.RxBytes) / elapsed, Timestamp: ts, Type: model.MetricGauge},
			model.Metric{Name: "net.tx_bytes_per_sec", Tags: tags, Value: float64(cur.TxBytes-old.TxBytes) / elapsed, Timestamp: ts, Type: model.MetricGauge},
			model.Metric{Name: "net.rx_packets_per_sec", Tags: tags, Value: float64(cur.RxPackets-old.RxPackets) / elapsed, Timestamp: ts, Type: model.MetricGauge},
			model.Metric{Name: "net.tx_packets_per_sec", Tags: tags, Value: float64(cur.TxPackets-old.TxPackets) / elapsed, Timestamp: ts, Type: model.MetricGauge},
			model.Metric{Name: "net.rx_errors", Tags: tags, Value: float64(cur.RxErrors), Timestamp: ts, Type: model.MetricCounter},
			model.Metric{Name: "net.tx_errors", Tags: tags, Value: float64(cur.TxErrors), Timestamp: ts, Type: model.MetricCounter},
		)
	}

	return metrics, nil
}

func parseNetDev(r io.Reader) (map[string]NetDevStats, error) {
	result := make(map[string]NetDevStats)
	scanner := bufio.NewScanner(r)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		// Skip the two header lines.
		if lineNum <= 2 {
			continue
		}
		line := scanner.Text()
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}
		iface := strings.TrimSpace(line[:colonIdx])
		rest := strings.Fields(line[colonIdx+1:])
		if len(rest) < 16 {
			continue
		}

		parse := func(idx int) uint64 {
			v, _ := strconv.ParseUint(rest[idx], 10, 64)
			return v
		}

		result[iface] = NetDevStats{
			RxBytes:   parse(0),
			RxPackets: parse(1),
			RxErrors:  parse(2),
			TxBytes:   parse(8),
			TxPackets: parse(9),
			TxErrors:  parse(10),
		}
	}
	return result, scanner.Err()
}
