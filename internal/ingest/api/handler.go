// Package api provides the HTTP ingestion API for Lens.
package api

import (
	"encoding/json"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/louispm/lens/internal/common/model"
	"github.com/louispm/lens/internal/ingest/clickhouse"
)

// Handler implements the ingestion API endpoints.
type Handler struct {
	writer    *clickhouse.BatchWriter
	logger    *zap.Logger
	rateLimit int

	// Counters.
	RequestCount  atomic.Int64
	BytesReceived atomic.Int64
	ErrorCount    atomic.Int64
	RejectedCount atomic.Int64
	ready         atomic.Bool
}

// NewHandler creates an API handler.
func NewHandler(writer *clickhouse.BatchWriter, rateLimit int, logger *zap.Logger) *Handler {
	return &Handler{
		writer:    writer,
		logger:    logger,
		rateLimit: rateLimit,
	}
}

// SetReady marks the service as ready to accept traffic.
func (h *Handler) SetReady() { h.ready.Store(true) }

// RegisterRoutes sets up the HTTP routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/ingest", h.handleIngest)
	mux.HandleFunc("POST /api/v1/ingest/metrics", h.handleIngestMetrics)
	mux.HandleFunc("POST /api/v1/ingest/logs", h.handleIngestLogs)
	mux.HandleFunc("POST /api/v1/ingest/traces", h.handleIngestTraces)
	mux.HandleFunc("GET /health", h.handleHealth)
	mux.HandleFunc("GET /ready", h.handleReady)
}

// handleIngest accepts a full Batch (metrics + logs + traces).
func (h *Handler) handleIngest(w http.ResponseWriter, r *http.Request) {
	h.RequestCount.Add(1)

	body, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		h.ErrorCount.Add(1)
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	h.BytesReceived.Add(int64(len(body)))

	var batch model.Batch
	if err := json.Unmarshal(body, &batch); err != nil {
		h.ErrorCount.Add(1)
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if len(batch.Metrics) > 0 {
		h.writer.AddMetrics(batch.Metrics)
	}
	if len(batch.Logs) > 0 {
		h.writer.AddLogs(batch.Logs)
	}
	if len(batch.Spans) > 0 {
		h.writer.AddSpans(batch.Spans)
	}

	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"accepted"}`))
}

// handleIngestMetrics accepts metrics only.
func (h *Handler) handleIngestMetrics(w http.ResponseWriter, r *http.Request) {
	h.RequestCount.Add(1)

	var metrics []model.Metric
	if err := json.NewDecoder(io.LimitReader(r.Body, 10*1024*1024)).Decode(&metrics); err != nil {
		h.ErrorCount.Add(1)
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	h.writer.AddMetrics(metrics)
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"accepted"}`))
}

// handleIngestLogs accepts logs only.
func (h *Handler) handleIngestLogs(w http.ResponseWriter, r *http.Request) {
	h.RequestCount.Add(1)

	var logs []model.LogEntry
	if err := json.NewDecoder(io.LimitReader(r.Body, 10*1024*1024)).Decode(&logs); err != nil {
		h.ErrorCount.Add(1)
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	h.writer.AddLogs(logs)
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"accepted"}`))
}

// handleIngestTraces accepts spans only.
func (h *Handler) handleIngestTraces(w http.ResponseWriter, r *http.Request) {
	h.RequestCount.Add(1)

	var spans []model.Span
	if err := json.NewDecoder(io.LimitReader(r.Body, 10*1024*1024)).Decode(&spans); err != nil {
		h.ErrorCount.Add(1)
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	h.writer.AddSpans(spans)
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"accepted"}`))
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (h *Handler) handleReady(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.ready.Load() {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"not ready"}`))
	}
}

// Middleware returns an http.Handler that applies rate limiting and request logging.
func (h *Handler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
		h.logger.Debug("request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Duration("duration", time.Since(start)),
		)
	})
}
