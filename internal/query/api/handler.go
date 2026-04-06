// Package api provides the HTTP API for the Lens query layer.
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/louispm/lens/internal/query/cache"
	"github.com/louispm/lens/internal/query/engine"
)

// Handler implements the query API endpoints.
type Handler struct {
	metrics *engine.MetricEngine
	logs    *engine.LogEngine
	traces  *engine.TraceEngine
	cache   *cache.Cache
	logger  *zap.Logger
}

// NewHandler creates a query API handler.
func NewHandler(metrics *engine.MetricEngine, logs *engine.LogEngine, traces *engine.TraceEngine, c *cache.Cache, logger *zap.Logger) *Handler {
	return &Handler{
		metrics: metrics,
		logs:    logs,
		traces:  traces,
		cache:   c,
		logger:  logger,
	}
}

// RegisterRoutes sets up the HTTP routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/query", h.handleMetricQuery)
	mux.HandleFunc("GET /api/v1/query_range", h.handleMetricRangeQuery)
	mux.HandleFunc("GET /api/v1/logs", h.handleLogSearch)
	mux.HandleFunc("GET /api/v1/logs/aggregate", h.handleLogAggregate)
	mux.HandleFunc("GET /api/v1/traces/{traceID}", h.handleGetTrace)
	mux.HandleFunc("GET /api/v1/traces", h.handleSearchTraces)
	mux.HandleFunc("GET /health", h.handleHealth)
	mux.HandleFunc("GET /ready", h.handleReady)
}

func (h *Handler) handleMetricQuery(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		writeError(w, http.StatusBadRequest, "missing query parameter")
		return
	}

	pq, err := engine.ParsePromQL(query)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid PromQL: "+err.Error())
		return
	}

	timeStr := r.URL.Query().Get("time")
	at := time.Now()
	if timeStr != "" {
		if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
			at = t
		}
	}

	// Check cache.
	cacheKey := cache.Key("instant", query, at.Truncate(15*time.Second).String())
	if cached, ok := h.cache.Get(cacheKey); ok {
		writeJSON(w, cached)
		return
	}

	samples, err := h.metrics.InstantQuery(r.Context(), pq.MetricName, pq.Filters, at)
	if err != nil {
		h.logger.Error("metric query failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}

	h.cache.Set(cacheKey, samples)
	writeJSON(w, samples)
}

func (h *Handler) handleMetricRangeQuery(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		writeError(w, http.StatusBadRequest, "missing query parameter")
		return
	}

	pq, err := engine.ParsePromQL(query)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid PromQL: "+err.Error())
		return
	}

	start, end, step := parseTimeRange(r)
	pq.Start = start
	pq.End = end
	pq.Step = step

	cacheKey := cache.Key("range", query, start.String(), end.String(), step.String())
	if cached, ok := h.cache.Get(cacheKey); ok {
		writeJSON(w, cached)
		return
	}

	results, err := h.metrics.Query(r.Context(), pq)
	if err != nil {
		h.logger.Error("range query failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}

	h.cache.Set(cacheKey, results)
	writeJSON(w, results)
}

func (h *Handler) handleLogSearch(w http.ResponseWriter, r *http.Request) {
	q := &engine.LogQuery{
		Service:   r.URL.Query().Get("service"),
		Level:     r.URL.Query().Get("level"),
		Search:    r.URL.Query().Get("search"),
		OrderDesc: r.URL.Query().Get("order") == "desc",
	}

	if v := r.URL.Query().Get("limit"); v != "" {
		q.Limit, _ = strconv.Atoi(v)
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		q.Offset, _ = strconv.Atoi(v)
	}

	start, end, _ := parseTimeRange(r)
	q.Start = start
	q.End = end

	records, total, err := h.logs.Search(r.Context(), q)
	if err != nil {
		h.logger.Error("log search failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}

	writeJSON(w, map[string]any{
		"records": records,
		"total":   total,
	})
}

func (h *Handler) handleLogAggregate(w http.ResponseWriter, r *http.Request) {
	groupBy := r.URL.Query().Get("group_by")
	if groupBy == "" {
		writeError(w, http.StatusBadRequest, "missing group_by parameter")
		return
	}

	q := &engine.LogQuery{
		Service: r.URL.Query().Get("service"),
		Level:   r.URL.Query().Get("level"),
		Search:  r.URL.Query().Get("search"),
	}
	start, end, _ := parseTimeRange(r)
	q.Start = start
	q.End = end

	aggs, err := h.logs.Aggregate(r.Context(), q, groupBy)
	if err != nil {
		h.logger.Error("log aggregate failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "aggregation failed")
		return
	}

	writeJSON(w, aggs)
}

func (h *Handler) handleGetTrace(w http.ResponseWriter, r *http.Request) {
	traceID := r.PathValue("traceID")
	if traceID == "" {
		writeError(w, http.StatusBadRequest, "missing traceID")
		return
	}

	cacheKey := cache.Key("trace", traceID)
	if cached, ok := h.cache.Get(cacheKey); ok {
		writeJSON(w, cached)
		return
	}

	result, err := h.traces.GetTrace(r.Context(), traceID)
	if err != nil {
		h.logger.Error("get trace failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "trace lookup failed")
		return
	}
	if result == nil {
		writeError(w, http.StatusNotFound, "trace not found")
		return
	}

	h.cache.Set(cacheKey, result)
	writeJSON(w, result)
}

func (h *Handler) handleSearchTraces(w http.ResponseWriter, r *http.Request) {
	q := &engine.TraceSearchQuery{
		Service:   r.URL.Query().Get("service"),
		Operation: r.URL.Query().Get("operation"),
	}

	if v := r.URL.Query().Get("min_duration"); v != "" {
		q.MinDuration, _ = time.ParseDuration(v)
	}
	if v := r.URL.Query().Get("max_duration"); v != "" {
		q.MaxDuration, _ = time.ParseDuration(v)
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		q.Limit, _ = strconv.Atoi(v)
	}

	start, end, _ := parseTimeRange(r)
	q.Start = start
	q.End = end

	results, err := h.traces.SearchTraces(r.Context(), q)
	if err != nil {
		h.logger.Error("search traces failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "trace search failed")
		return
	}

	writeJSON(w, results)
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleReady(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ready"})
}

func parseTimeRange(r *http.Request) (start, end time.Time, step time.Duration) {
	end = time.Now()
	start = end.Add(-1 * time.Hour)

	if v := r.URL.Query().Get("start"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			start = t
		}
	}
	if v := r.URL.Query().Get("end"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			end = t
		}
	}
	if v := r.URL.Query().Get("step"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			step = d
		}
	}
	if step == 0 {
		step = 60 * time.Second
	}
	return
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
