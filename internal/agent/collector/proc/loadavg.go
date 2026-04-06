package proc

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/louispm/lens/internal/common/model"
)

// LoadAvgCollector reads /proc/loadavg.
type LoadAvgCollector struct {
	procRoot string
	hostname string
}

func NewLoadAvgCollector(procRoot, hostname string) *LoadAvgCollector {
	return &LoadAvgCollector{procRoot: procRoot, hostname: hostname}
}

func (c *LoadAvgCollector) Name() string { return "loadavg" }

func (c *LoadAvgCollector) Collect(ctx context.Context) ([]model.Metric, error) {
	f, err := openProcFile(c.procRoot, "loadavg")
	if err != nil {
		return nil, fmt.Errorf("open /proc/loadavg: %w", err)
	}
	defer f.Close()

	return c.parse(f, time.Now())
}

func (c *LoadAvgCollector) parse(r io.Reader, ts time.Time) ([]model.Metric, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	fields := strings.Fields(string(data))
	if len(fields) < 5 {
		return nil, fmt.Errorf("unexpected /proc/loadavg format: %q", string(data))
	}

	load1, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return nil, fmt.Errorf("parse load1: %w", err)
	}
	load5, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return nil, fmt.Errorf("parse load5: %w", err)
	}
	load15, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return nil, fmt.Errorf("parse load15: %w", err)
	}

	// fields[3] is "running/total" processes
	procParts := strings.Split(fields[3], "/")
	var running, total float64
	if len(procParts) == 2 {
		running, _ = strconv.ParseFloat(procParts[0], 64)
		total, _ = strconv.ParseFloat(procParts[1], 64)
	}

	tags := map[string]string{"host": c.hostname}
	return []model.Metric{
		{Name: "loadavg.1min", Tags: tags, Value: load1, Timestamp: ts, Type: model.MetricGauge},
		{Name: "loadavg.5min", Tags: tags, Value: load5, Timestamp: ts, Type: model.MetricGauge},
		{Name: "loadavg.15min", Tags: tags, Value: load15, Timestamp: ts, Type: model.MetricGauge},
		{Name: "loadavg.running_processes", Tags: tags, Value: running, Timestamp: ts, Type: model.MetricGauge},
		{Name: "loadavg.total_processes", Tags: tags, Value: total, Timestamp: ts, Type: model.MetricGauge},
	}, nil
}
