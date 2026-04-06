package selfmon

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// Monitor exposes Prometheus metrics and health endpoints for the agent.
type Monitor struct {
	addr   string
	logger *zap.Logger
	server *http.Server
	ready  atomic.Bool

	MetricsCollected prometheus.Counter
	LogsCollected    prometheus.Counter
	SpansReceived    prometheus.Counter
	BatchesShipped   prometheus.Counter
	BatchesFailed    prometheus.Counter
	BufferBytes      prometheus.Gauge
	BufferSegments   prometheus.Gauge
	DataLossTotal    prometheus.Counter
	CollectDuration  prometheus.Histogram
}

// New creates a self-monitoring instance.
func New(addr string, logger *zap.Logger) *Monitor {
	m := &Monitor{
		addr:   addr,
		logger: logger,
		MetricsCollected: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "lens_agent_metrics_collected_total",
			Help: "Total number of metrics collected",
		}),
		LogsCollected: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "lens_agent_logs_collected_total",
			Help: "Total number of log entries collected",
		}),
		SpansReceived: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "lens_agent_spans_received_total",
			Help: "Total number of spans received via OTel",
		}),
		BatchesShipped: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "lens_agent_batches_shipped_total",
			Help: "Total batches successfully shipped to backend",
		}),
		BatchesFailed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "lens_agent_batches_failed_total",
			Help: "Total batches that failed to ship",
		}),
		BufferBytes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "lens_agent_buffer_bytes",
			Help: "Current disk buffer usage in bytes",
		}),
		BufferSegments: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "lens_agent_buffer_segments",
			Help: "Current number of buffer segments",
		}),
		DataLossTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "lens_agent_data_loss_total",
			Help: "Total entries dropped due to buffer overflow",
		}),
		CollectDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "lens_agent_collect_duration_seconds",
			Help:    "Duration of metric collection cycles",
			Buckets: prometheus.DefBuckets,
		}),
	}

	return m
}

// Start begins serving health and metrics endpoints.
func (m *Monitor) Start() error {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		m.MetricsCollected,
		m.LogsCollected,
		m.SpansReceived,
		m.BatchesShipped,
		m.BatchesFailed,
		m.BufferBytes,
		m.BufferSegments,
		m.DataLossTotal,
		m.CollectDuration,
	)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		if m.ready.Load() {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ready"}`))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"not ready"}`))
		}
	})

	m.server = &http.Server{
		Addr:         m.addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		m.logger.Info("self-monitoring started", zap.String("addr", m.addr))
		if err := m.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			m.logger.Error("self-monitoring server error", zap.Error(err))
		}
	}()

	return nil
}

// SetReady marks the agent as ready.
func (m *Monitor) SetReady() {
	m.ready.Store(true)
}

// Stop gracefully shuts down the HTTP server.
func (m *Monitor) Stop(ctx context.Context) {
	if m.server != nil {
		m.server.Shutdown(ctx)
	}
}
