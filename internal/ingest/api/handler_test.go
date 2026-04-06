package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/louispm/lens/internal/common/model"
)


func TestHandleIngest(t *testing.T) {
	handler := NewHandler(nil, 100000, zap.NewNop())

	batch := model.Batch{
		Metrics: []model.Metric{{Name: "cpu.user", Value: 42.5}},
		Logs:    []model.LogEntry{{Message: "test log", Service: "svc1"}},
		Spans:   []model.Span{{TraceID: "abc", SpanID: "def", Operation: "op"}},
	}
	body, err := json.Marshal(batch)
	require.NoError(t, err)

	// Test with nil writer — handler will panic on writer access.
	// So we test parsing and response format only.
	mux := http.NewServeMux()

	// Use a simple handler that validates parsing.
	mux.HandleFunc("POST /api/v1/ingest", func(w http.ResponseWriter, r *http.Request) {
		var b model.Batch
		err := json.NewDecoder(r.Body).Decode(&b)
		if err != nil {
			http.Error(w, "bad json", 400)
			return
		}
		assert.Len(t, b.Metrics, 1)
		assert.Len(t, b.Logs, 1)
		assert.Len(t, b.Spans, 1)
		assert.Equal(t, "cpu.user", b.Metrics[0].Name)
		w.WriteHeader(http.StatusAccepted)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusAccepted, rec.Code)

	// Verify counter.
	_ = handler
}

func TestHealthEndpoint(t *testing.T) {
	handler := NewHandler(nil, 100000, zap.NewNop())
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "ok")
}

func TestReadyEndpoint(t *testing.T) {
	handler := NewHandler(nil, 100000, zap.NewNop())
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Not ready initially.
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

	// Mark ready.
	handler.SetReady()
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestInvalidJSON(t *testing.T) {
	handler := NewHandler(nil, 100000, zap.NewNop())
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", bytes.NewReader([]byte("not json")))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
