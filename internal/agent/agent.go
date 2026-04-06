package agent

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/louispm/lens/internal/agent/batcher"
	"github.com/louispm/lens/internal/agent/buffer"
	"github.com/louispm/lens/internal/agent/collector/logs"
	"github.com/louispm/lens/internal/agent/collector/otel"
	"github.com/louispm/lens/internal/agent/collector/proc"
	"github.com/louispm/lens/internal/agent/config"
	"github.com/louispm/lens/internal/agent/selfmon"
	"github.com/louispm/lens/internal/agent/shipper"
	"github.com/louispm/lens/internal/common/model"
)

// Agent is the top-level orchestrator for the Lens collector agent.
type Agent struct {
	cfg    *config.Config
	logger *zap.Logger
}

// New creates an Agent.
func New(cfg *config.Config, logger *zap.Logger) *Agent {
	return &Agent{cfg: cfg, logger: logger}
}

// Run starts all agent subsystems and blocks until the context is cancelled.
func (a *Agent) Run(ctx context.Context) error {
	a.logger.Info("starting lens agent",
		zap.String("proc_root", a.cfg.ProcRoot),
		zap.Duration("collect_interval", a.cfg.CollectInterval),
		zap.String("backend_url", a.cfg.Backend.URL),
	)

	// Initialize self-monitoring.
	mon := selfmon.New(a.cfg.HTTPAddr, a.logger)
	if err := mon.Start(); err != nil {
		return err
	}
	defer mon.Stop(context.Background())

	// Initialize disk-backed buffer.
	buf, err := buffer.NewDiskBuffer(a.cfg.Buffer.Dir, a.cfg.Buffer.MaxSegmentSize, a.cfg.Buffer.MaxTotalSize)
	if err != nil {
		return err
	}
	defer buf.Close()

	// Initialize batcher.
	bat := batcher.New(a.cfg.Batch.MaxSize, a.cfg.Batch.MaxWait, func(data []byte) error {
		return buf.Write(data)
	}, a.logger)
	defer bat.Stop()

	// Initialize /proc collectors.
	registry := proc.NewRegistry(a.cfg.ProcRoot, a.logger)

	// Initialize log tailer.
	var tailerConfigs []logs.TailerConfig
	for _, lf := range a.cfg.LogFiles {
		tailerConfigs = append(tailerConfigs, logs.TailerConfig{
			Path:    lf.Path,
			Service: lf.Service,
			Format:  lf.Format,
		})
	}
	tailer := logs.NewTailer(tailerConfigs, func(entry model.LogEntry) {
		bat.AddLogs([]model.LogEntry{entry})
		mon.LogsCollected.Inc()
	}, a.cfg.Buffer.Dir, a.logger)
	tailer.Start(ctx)
	defer tailer.Stop()

	// Initialize OTel span receiver.
	otelRecv := otel.NewReceiver(a.cfg.OTelGRPCAddr, func(spans []model.Span) {
		bat.AddSpans(spans)
		mon.SpansReceived.Add(float64(len(spans)))
	}, a.logger)
	if err := otelRecv.Start(); err != nil {
		a.logger.Warn("failed to start OTel receiver", zap.Error(err))
	} else {
		defer otelRecv.Stop()
	}

	// Start shipper.
	ship := shipper.New(buf, a.cfg.Backend.URL, a.cfg.Backend.Timeout, a.logger)
	go ship.Run(ctx)

	// Mark agent as ready after first collection.
	firstCollection := true

	// Start collection loop.
	ticker := time.NewTicker(a.cfg.CollectInterval)
	defer ticker.Stop()

	// Collect immediately on start.
	a.collect(ctx, registry, bat, mon, &firstCollection)

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("agent shutting down")
			return nil
		case <-ticker.C:
			a.collect(ctx, registry, bat, mon, &firstCollection)
			// Update buffer metrics.
			mon.BufferBytes.Set(float64(buf.TotalSize()))
			mon.BufferSegments.Set(float64(buf.SegmentCount()))
			if loss := buf.DataLossCount(); loss > 0 {
				mon.DataLossTotal.Add(float64(loss))
			}
		}
	}
}

func (a *Agent) collect(ctx context.Context, registry *proc.Registry, bat *batcher.Batcher, mon *selfmon.Monitor, firstCollection *bool) {
	start := time.Now()
	metrics := registry.CollectAll(ctx)
	elapsed := time.Since(start)

	mon.CollectDuration.Observe(elapsed.Seconds())
	mon.MetricsCollected.Add(float64(len(metrics)))

	if len(metrics) > 0 {
		bat.AddMetrics(metrics)
	}

	a.logger.Debug("collection cycle complete",
		zap.Int("metrics", len(metrics)),
		zap.Duration("elapsed", elapsed),
	)

	if *firstCollection {
		*firstCollection = false
		mon.SetReady()
		a.logger.Info("agent ready")
	}
}
