package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/louispm/lens/internal/query/engine"
)

// WebSocketHandler provides real-time metric streaming.
// Uses server-sent events (SSE) as a simpler alternative to WebSockets
// that works without additional dependencies.
type WebSocketHandler struct {
	metrics *engine.MetricEngine
	logger  *zap.Logger
}

// NewWebSocketHandler creates a real-time streaming handler.
func NewWebSocketHandler(metrics *engine.MetricEngine, logger *zap.Logger) *WebSocketHandler {
	return &WebSocketHandler{metrics: metrics, logger: logger}
}

// RegisterRoutes adds the streaming endpoint.
func (h *WebSocketHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/stream", h.handleStream)
}

// handleStream provides SSE streaming of metric data.
func (h *WebSocketHandler) handleStream(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		writeError(w, http.StatusBadRequest, "missing query parameter")
		return
	}

	pq, err := engine.ParsePromQL(query)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid query")
		return
	}

	intervalStr := r.URL.Query().Get("interval")
	interval := 10 * time.Second
	if intervalStr != "" {
		if d, err := time.ParseDuration(intervalStr); err == nil && d >= time.Second {
			interval = d
		}
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx := r.Context()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	h.logger.Debug("SSE stream started", zap.String("query", query), zap.Duration("interval", interval))

	for {
		select {
		case <-ctx.Done():
			h.logger.Debug("SSE stream closed by client")
			return
		case <-ticker.C:
			samples := h.queryLatest(ctx, pq)
			data, _ := json.Marshal(samples)
			_, _ = w.Write([]byte("data: "))
			_, _ = w.Write(data)
			_, _ = w.Write([]byte("\n\n"))
			flusher.Flush()
		}
	}
}

func (h *WebSocketHandler) queryLatest(ctx context.Context, pq *engine.PromQLQuery) []engine.MetricSample {
	samples, err := h.metrics.InstantQuery(ctx, pq.MetricName, pq.Filters, time.Now())
	if err != nil {
		h.logger.Warn("stream query failed", zap.Error(err))
		return nil
	}
	return samples
}
