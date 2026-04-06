// Package ingest provides the ingestion backend service.
package ingest

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/louispm/lens/internal/ingest/api"
	"github.com/louispm/lens/internal/ingest/clickhouse"
	"github.com/louispm/lens/internal/ingest/config"
)

// Server is the top-level ingestion backend.
type Server struct {
	cfg    *config.Config
	logger *zap.Logger
}

// NewServer creates an ingestion server.
func NewServer(cfg *config.Config, logger *zap.Logger) *Server {
	return &Server{cfg: cfg, logger: logger}
}

// Run starts the ingestion backend and blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	s.logger.Info("starting ingestion backend",
		zap.String("http_addr", s.cfg.HTTPAddr),
		zap.String("clickhouse_dsn", s.cfg.ClickHouseDSN),
	)

	// Connect to ClickHouse.
	ch, err := clickhouse.New(s.cfg.ClickHouseDSN, s.logger)
	if err != nil {
		return err
	}
	defer ch.Close()

	// Run migrations.
	if err := ch.Migrate(ctx); err != nil {
		return err
	}

	// Start batch writer.
	writer := clickhouse.NewBatchWriter(ch, s.cfg.FlushInterval, s.cfg.BatchMaxSize, s.logger)
	writer.Start()
	defer writer.Stop()

	// Create API handler.
	handler := api.NewHandler(writer, s.cfg.RateLimit, s.logger)

	// Set up HTTP server.
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	httpServer := &http.Server{
		Addr:         s.cfg.HTTPAddr,
		Handler:      handler.Middleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start metrics server on separate port.
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{}))
	metricsServer := &http.Server{
		Addr:    s.cfg.MetricsAddr,
		Handler: metricsMux,
	}
	go func() {
		s.logger.Info("metrics server started", zap.String("addr", s.cfg.MetricsAddr))
		metricsServer.ListenAndServe()
	}()

	// Mark ready.
	handler.SetReady()
	s.logger.Info("ingestion backend ready")

	// Start HTTP server.
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server error", zap.Error(err))
		}
	}()

	// Wait for shutdown.
	<-ctx.Done()
	s.logger.Info("ingestion backend shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	httpServer.Shutdown(shutdownCtx)
	metricsServer.Shutdown(shutdownCtx)

	return nil
}
