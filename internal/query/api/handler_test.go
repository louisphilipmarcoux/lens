package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/louispm/lens/internal/query/cache"
)

func TestHealthEndpoint(t *testing.T) {
	h := NewHandler(nil, nil, nil, cache.New(time.Minute, 100), zap.NewNop())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "ok")
}

func TestMissingQueryParam(t *testing.T) {
	h := NewHandler(nil, nil, nil, cache.New(time.Minute, 100), zap.NewNop())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/query", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "missing query")
}

func TestParseTimeRange(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?start=2026-01-01T00:00:00Z&end=2026-01-01T01:00:00Z&step=5m", nil)
	start, end, step := parseTimeRange(req)

	assert.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), start)
	assert.Equal(t, time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC), end)
	assert.Equal(t, 5*time.Minute, step)
}

func TestParseTimeRangeDefaults(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	start, end, step := parseTimeRange(req)

	assert.WithinDuration(t, time.Now().Add(-1*time.Hour), start, 5*time.Second)
	assert.WithinDuration(t, time.Now(), end, 5*time.Second)
	assert.Equal(t, 60*time.Second, step)
}
