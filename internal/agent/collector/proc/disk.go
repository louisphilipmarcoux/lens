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

// DiskStats holds raw counters from /proc/diskstats for one device.
type DiskStats struct {
	ReadsCompleted  uint64
	SectorsRead     uint64
	WritesCompleted uint64
	SectorsWritten  uint64
	IOInProgress    uint64
	IOTime          uint64
	WeightedIOTime  uint64
}

// DiskCollector reads /proc/diskstats and computes per-device I/O rates.
type DiskCollector struct {
	procRoot string
	hostname string
	mu       sync.Mutex
	prev     map[string]DiskStats
	prevTime time.Time
}

func NewDiskCollector(procRoot, hostname string) *DiskCollector {
	return &DiskCollector{
		procRoot: procRoot,
		hostname: hostname,
		prev:     make(map[string]DiskStats),
	}
}

func (c *DiskCollector) Name() string { return "disk" }

func (c *DiskCollector) Collect(ctx context.Context) ([]model.Metric, error) {
	f, err := openProcFile(c.procRoot, "diskstats")
	if err != nil {
		return nil, fmt.Errorf("open /proc/diskstats: %w", err)
	}
	defer f.Close()

	now := time.Now()
	return c.parse(f, now)
}

func (c *DiskCollector) parse(r io.Reader, ts time.Time) ([]model.Metric, error) {
	current, err := parseDiskStats(r)
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
	for dev, cur := range current {
		old, ok := prev[dev]
		if !ok {
			continue
		}

		tags := map[string]string{"host": c.hostname, "device": dev}
		// Sectors are typically 512 bytes.
		readBytes := float64(cur.SectorsRead-old.SectorsRead) * 512 / elapsed
		writeBytes := float64(cur.SectorsWritten-old.SectorsWritten) * 512 / elapsed

		metrics = append(metrics,
			model.Metric{Name: "disk.read_bytes_per_sec", Tags: tags, Value: readBytes, Timestamp: ts, Type: model.MetricGauge},
			model.Metric{Name: "disk.write_bytes_per_sec", Tags: tags, Value: writeBytes, Timestamp: ts, Type: model.MetricGauge},
			model.Metric{Name: "disk.reads_per_sec", Tags: tags, Value: float64(cur.ReadsCompleted-old.ReadsCompleted) / elapsed, Timestamp: ts, Type: model.MetricGauge},
			model.Metric{Name: "disk.writes_per_sec", Tags: tags, Value: float64(cur.WritesCompleted-old.WritesCompleted) / elapsed, Timestamp: ts, Type: model.MetricGauge},
			model.Metric{Name: "disk.io_in_progress", Tags: tags, Value: float64(cur.IOInProgress), Timestamp: ts, Type: model.MetricGauge},
			model.Metric{Name: "disk.io_time_ms", Tags: tags, Value: float64(cur.IOTime - old.IOTime), Timestamp: ts, Type: model.MetricCounter},
		)
	}

	return metrics, nil
}

func parseDiskStats(r io.Reader) (map[string]DiskStats, error) {
	result := make(map[string]DiskStats)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 14 {
			continue
		}
		dev := fields[2]

		// Skip loop, ram, and dm- devices that have no I/O.
		if strings.HasPrefix(dev, "loop") || strings.HasPrefix(dev, "ram") {
			continue
		}

		parse := func(idx int) uint64 {
			v, _ := strconv.ParseUint(fields[idx], 10, 64)
			return v
		}

		stats := DiskStats{
			ReadsCompleted:  parse(3),
			SectorsRead:     parse(5),
			WritesCompleted: parse(7),
			SectorsWritten:  parse(9),
			IOInProgress:    parse(11),
			IOTime:          parse(12),
			WeightedIOTime:  parse(13),
		}

		// Skip devices with zero total I/O.
		if stats.ReadsCompleted == 0 && stats.WritesCompleted == 0 {
			continue
		}

		result[dev] = stats
	}
	return result, scanner.Err()
}
