// Package query provides the query layer service.
package query

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/louispm/lens/internal/query/api"
	"github.com/louispm/lens/internal/query/cache"
	"github.com/louispm/lens/internal/query/config"
	"github.com/louispm/lens/internal/query/engine"
	"github.com/louispm/lens/internal/query/middleware"
)

// Server is the query layer service.
type Server struct {
	cfg    *config.Config
	logger *zap.Logger
}

// NewServer creates a query server.
func NewServer(cfg *config.Config, logger *zap.Logger) *Server {
	return &Server{cfg: cfg, logger: logger}
}

// Run starts the query server and blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	s.logger.Info("starting query layer",
		zap.String("http_addr", s.cfg.HTTPAddr),
		zap.String("clickhouse_dsn", s.cfg.ClickHouseDSN),
	)

	// Connect to ClickHouse.
	db, err := sql.Open("clickhouse", s.cfg.ClickHouseDSN)
	if err != nil {
		return err
	}
	defer db.Close()

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return err
	}

	// Create engines.
	metricEngine := engine.NewMetricEngine(db)
	logEngine := engine.NewLogEngine(db)
	traceEngine := engine.NewTraceEngine(db)

	// Create cache.
	queryCache := cache.New(s.cfg.CacheTTL, s.cfg.CacheMaxSize)

	// Create API handler.
	handler := api.NewHandler(metricEngine, logEngine, traceEngine, queryCache, s.logger)

	// Create WebSocket handler.
	wsHandler := api.NewWebSocketHandler(metricEngine, s.logger)

	// Set up routes.
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	wsHandler.RegisterRoutes(mux)

	// Apply middleware — skip timeout for SSE stream (it strips http.Flusher).
	limiter := middleware.NewRateLimiter(s.cfg.RateLimit)
	queryTimeout := s.cfg.QueryTimeout
	httpHandler := middleware.RateLimit(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/stream" {
			mux.ServeHTTP(w, r)
			return
		}
		middleware.Timeout(queryTimeout)(mux).ServeHTTP(w, r)
	}))

	// Start HTTP server.
	httpServer := &http.Server{
		Addr:         s.cfg.HTTPAddr,
		Handler:      httpHandler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // disabled — SSE streams are long-lived
		IdleTimeout:  120 * time.Second,
	}

	// Start metrics server.
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{}))
	metricsServer := &http.Server{Addr: s.cfg.MetricsAddr, Handler: metricsMux}
	go func() {
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("metrics server error", zap.Error(err))
		}
	}()

	s.logger.Info("query layer ready")

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	s.logger.Info("query layer shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)
	_ = metricsServer.Shutdown(shutdownCtx)

	return nil
}
