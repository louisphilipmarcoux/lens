package proc

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/louispm/lens/internal/common/model"
)

// Collector gathers metrics from a specific source.
type Collector interface {
	Name() string
	Collect(ctx context.Context) ([]model.Metric, error)
}

// Registry manages a set of collectors and runs them on a schedule.
type Registry struct {
	collectors []Collector
	logger     *zap.Logger
	procRoot   string
	hostname   string
}

// NewRegistry creates a Registry with all standard /proc collectors.
func NewRegistry(procRoot string, logger *zap.Logger) *Registry {
	hostname, _ := os.Hostname()

	r := &Registry{
		logger:   logger,
		procRoot: procRoot,
		hostname: hostname,
	}

	r.collectors = []Collector{
		NewCPUCollector(procRoot, hostname),
		NewMemoryCollector(procRoot, hostname),
		NewDiskCollector(procRoot, hostname),
		NewNetworkCollector(procRoot, hostname),
		NewLoadAvgCollector(procRoot, hostname),
	}

	return r
}

// CollectAll runs all collectors and returns aggregated metrics.
func (r *Registry) CollectAll(ctx context.Context) []model.Metric {
	var (
		mu      sync.Mutex
		metrics []model.Metric
	)

	var wg sync.WaitGroup
	for _, c := range r.collectors {
		wg.Add(1)
		go func(c Collector) {
			defer wg.Done()
			start := time.Now()
			m, err := c.Collect(ctx)
			elapsed := time.Since(start)
			if err != nil {
				r.logger.Warn("collector failed",
					zap.String("collector", c.Name()),
					zap.Duration("elapsed", elapsed),
					zap.Error(err),
				)
				return
			}
			r.logger.Debug("collector succeeded",
				zap.String("collector", c.Name()),
				zap.Int("metrics", len(m)),
				zap.Duration("elapsed", elapsed),
			)
			mu.Lock()
			metrics = append(metrics, m...)
			mu.Unlock()
		}(c)
	}
	wg.Wait()
	return metrics
}

// openProcFile opens a file under the configured /proc root.
func openProcFile(procRoot, path string) (*os.File, error) {
	full := fmt.Sprintf("%s/%s", procRoot, path)
	return os.Open(full)
}
