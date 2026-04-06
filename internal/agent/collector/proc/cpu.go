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

// CPUStats holds raw values from /proc/stat for a single CPU line.
type CPUStats struct {
	User    uint64
	Nice    uint64
	System  uint64
	Idle    uint64
	IOWait  uint64
	IRQ     uint64
	SoftIRQ uint64
	Steal   uint64
}

func (s CPUStats) total() uint64 {
	return s.User + s.Nice + s.System + s.Idle + s.IOWait + s.IRQ + s.SoftIRQ + s.Steal
}

// CPUCollector reads /proc/stat and computes CPU utilization percentages.
type CPUCollector struct {
	procRoot string
	hostname string
	mu       sync.Mutex
	prev     map[string]CPUStats
}

func NewCPUCollector(procRoot, hostname string) *CPUCollector {
	return &CPUCollector{
		procRoot: procRoot,
		hostname: hostname,
		prev:     make(map[string]CPUStats),
	}
}

func (c *CPUCollector) Name() string { return "cpu" }

func (c *CPUCollector) Collect(ctx context.Context) ([]model.Metric, error) {
	f, err := openProcFile(c.procRoot, "stat")
	if err != nil {
		return nil, fmt.Errorf("open /proc/stat: %w", err)
	}
	defer f.Close()

	return c.parse(f, time.Now())
}

func (c *CPUCollector) parse(r io.Reader, ts time.Time) ([]model.Metric, error) {
	current, err := parseProcStat(r)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	prev := c.prev
	c.prev = current
	c.mu.Unlock()

	if len(prev) == 0 {
		return nil, nil
	}

	var metrics []model.Metric
	for name, cur := range current {
		old, ok := prev[name]
		if !ok {
			continue
		}

		totalDelta := cur.total() - old.total()
		if totalDelta == 0 {
			continue
		}

		fd := float64(totalDelta)
		tags := map[string]string{"host": c.hostname, "cpu": name}

		metrics = append(metrics,
			model.Metric{Name: "cpu.user_percent", Tags: tags, Value: float64(cur.User-old.User) / fd * 100, Timestamp: ts, Type: model.MetricGauge},
			model.Metric{Name: "cpu.system_percent", Tags: tags, Value: float64(cur.System-old.System) / fd * 100, Timestamp: ts, Type: model.MetricGauge},
			model.Metric{Name: "cpu.idle_percent", Tags: tags, Value: float64(cur.Idle-old.Idle) / fd * 100, Timestamp: ts, Type: model.MetricGauge},
			model.Metric{Name: "cpu.iowait_percent", Tags: tags, Value: float64(cur.IOWait-old.IOWait) / fd * 100, Timestamp: ts, Type: model.MetricGauge},
			model.Metric{Name: "cpu.steal_percent", Tags: tags, Value: float64(cur.Steal-old.Steal) / fd * 100, Timestamp: ts, Type: model.MetricGauge},
		)
	}

	return metrics, nil
}

func parseProcStat(r io.Reader) (map[string]CPUStats, error) {
	result := make(map[string]CPUStats)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		name := fields[0]
		vals := make([]uint64, 8)
		for i := 0; i < 8; i++ {
			v, err := strconv.ParseUint(fields[i+1], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parse %s field %d: %w", name, i, err)
			}
			vals[i] = v
		}
		result[name] = CPUStats{
			User:    vals[0],
			Nice:    vals[1],
			System:  vals[2],
			Idle:    vals[3],
			IOWait:  vals[4],
			IRQ:     vals[5],
			SoftIRQ: vals[6],
			Steal:   vals[7],
		}
	}
	return result, scanner.Err()
}
