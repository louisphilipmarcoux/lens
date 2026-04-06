package proc

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/louispm/lens/internal/common/model"
)

// MemoryCollector reads /proc/meminfo and produces memory metrics.
type MemoryCollector struct {
	procRoot string
	hostname string
}

func NewMemoryCollector(procRoot, hostname string) *MemoryCollector {
	return &MemoryCollector{procRoot: procRoot, hostname: hostname}
}

func (c *MemoryCollector) Name() string { return "memory" }

func (c *MemoryCollector) Collect(ctx context.Context) ([]model.Metric, error) {
	f, err := openProcFile(c.procRoot, "meminfo")
	if err != nil {
		return nil, fmt.Errorf("open /proc/meminfo: %w", err)
	}
	defer f.Close()

	return c.parse(f, time.Now())
}

func (c *MemoryCollector) parse(r io.Reader, ts time.Time) ([]model.Metric, error) {
	info, err := parseMeminfo(r)
	if err != nil {
		return nil, err
	}

	tags := map[string]string{"host": c.hostname}
	metrics := []model.Metric{
		{Name: "memory.total_bytes", Tags: tags, Value: float64(info["MemTotal"]) * 1024, Timestamp: ts, Type: model.MetricGauge},
		{Name: "memory.free_bytes", Tags: tags, Value: float64(info["MemFree"]) * 1024, Timestamp: ts, Type: model.MetricGauge},
		{Name: "memory.available_bytes", Tags: tags, Value: float64(info["MemAvailable"]) * 1024, Timestamp: ts, Type: model.MetricGauge},
		{Name: "memory.buffers_bytes", Tags: tags, Value: float64(info["Buffers"]) * 1024, Timestamp: ts, Type: model.MetricGauge},
		{Name: "memory.cached_bytes", Tags: tags, Value: float64(info["Cached"]) * 1024, Timestamp: ts, Type: model.MetricGauge},
		{Name: "memory.swap_total_bytes", Tags: tags, Value: float64(info["SwapTotal"]) * 1024, Timestamp: ts, Type: model.MetricGauge},
		{Name: "memory.swap_free_bytes", Tags: tags, Value: float64(info["SwapFree"]) * 1024, Timestamp: ts, Type: model.MetricGauge},
	}

	if total := info["MemTotal"]; total > 0 {
		used := total - info["MemAvailable"]
		metrics = append(metrics, model.Metric{
			Name: "memory.used_percent", Tags: tags,
			Value: float64(used) / float64(total) * 100, Timestamp: ts, Type: model.MetricGauge,
		})
	}
	if swapTotal := info["SwapTotal"]; swapTotal > 0 {
		swapUsed := swapTotal - info["SwapFree"]
		metrics = append(metrics, model.Metric{
			Name: "memory.swap_used_percent", Tags: tags,
			Value: float64(swapUsed) / float64(swapTotal) * 100, Timestamp: ts, Type: model.MetricGauge,
		})
	}

	return metrics, nil
}

// parseMeminfo parses /proc/meminfo into a map of field name -> value in kB.
func parseMeminfo(r io.Reader) (map[string]uint64, error) {
	result := make(map[string]uint64)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		valStr := strings.TrimSpace(parts[1])
		valStr = strings.TrimSuffix(valStr, " kB")
		valStr = strings.TrimSpace(valStr)
		v, err := strconv.ParseUint(valStr, 10, 64)
		if err != nil {
			continue
		}
		result[key] = v
	}
	return result, scanner.Err()
}
