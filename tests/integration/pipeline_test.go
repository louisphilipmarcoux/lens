//go:build integration

// Package integration contains end-to-end pipeline tests for Lens.
//
// These tests require a running ClickHouse instance and are executed
// in CI via Docker Compose.
package integration

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/louispm/lens/internal/common/model"
)

func getIngestURL() string {
	if v := os.Getenv("LENS_INGEST_URL"); v != "" {
		return v
	}
	return "http://localhost:8080"
}

func getQueryURL() string {
	if v := os.Getenv("LENS_QUERY_URL"); v != "" {
		return v
	}
	return "http://localhost:8081"
}

// requireServices skips the test if ingest or query services are not reachable.
func requireServices(t *testing.T, ingestURL, queryURL string) {
	t.Helper()
	for _, addr := range []string{ingestURL, queryURL} {
		conn, err := net.DialTimeout("tcp", addr[len("http://"):], 2*time.Second)
		if err != nil {
			t.Skipf("skipping: service at %s not reachable: %v", addr, err)
		}
		conn.Close()
	}
}

func TestPipelineE2E_MetricIngestAndQuery(t *testing.T) {
	ingestURL := getIngestURL()
	queryURL := getQueryURL()
	requireServices(t, ingestURL, queryURL)

	// Inject known metrics.
	batch := model.Batch{
		Metrics: []model.Metric{
			{Name: "test.e2e.cpu", Tags: map[string]string{"host": "e2e-host"}, Value: 42.5, Timestamp: time.Now(), Type: model.MetricGauge},
			{Name: "test.e2e.cpu", Tags: map[string]string{"host": "e2e-host"}, Value: 43.0, Timestamp: time.Now(), Type: model.MetricGauge},
		},
	}
	body, err := json.Marshal(batch)
	require.NoError(t, err)

	resp, err := http.Post(ingestURL+"/api/v1/ingest", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	resp.Body.Close()

	// Wait for flush.
	time.Sleep(10 * time.Second)

	// Query the metrics back.
	resp, err = http.Get(queryURL + "/api/v1/query?query=test.e2e.cpu")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestPipelineE2E_LogIngestAndSearch(t *testing.T) {
	ingestURL := getIngestURL()
	queryURL := getQueryURL()
	requireServices(t, ingestURL, queryURL)

	batch := model.Batch{
		Logs: []model.LogEntry{
			{Timestamp: time.Now(), Service: "e2e-svc", Host: "e2e-host", Level: "error", Message: "e2e test error message"},
		},
	}
	body, err := json.Marshal(batch)
	require.NoError(t, err)

	resp, err := http.Post(ingestURL+"/api/v1/ingest", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	resp.Body.Close()

	time.Sleep(10 * time.Second)

	resp, err = http.Get(queryURL + "/api/v1/logs?service=e2e-svc&search=e2e+test")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestPipelineE2E_TraceIngestAndLookup(t *testing.T) {
	ingestURL := getIngestURL()
	queryURL := getQueryURL()
	requireServices(t, ingestURL, queryURL)

	batch := model.Batch{
		Spans: []model.Span{
			{
				TraceID: "e2e-trace-001", SpanID: "span-root", Service: "e2e-svc",
				Operation: "GET /test", StartTime: time.Now(), Duration: 100 * time.Millisecond,
				Status: model.SpanStatusOK, Tags: map[string]string{"env": "test"},
			},
			{
				TraceID: "e2e-trace-001", SpanID: "span-child", ParentID: "span-root",
				Service: "e2e-db", Operation: "SELECT", StartTime: time.Now(),
				Duration: 50 * time.Millisecond, Status: model.SpanStatusOK,
			},
		},
	}
	body, err := json.Marshal(batch)
	require.NoError(t, err)

	resp, err := http.Post(ingestURL+"/api/v1/ingest", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	resp.Body.Close()

	time.Sleep(10 * time.Second)

	resp, err = http.Get(queryURL + "/api/v1/traces/e2e-trace-001")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}
