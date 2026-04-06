package selfmon

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestHealthEndpoint(t *testing.T) {
	m := New("127.0.0.1:0", zap.NewNop())
	// Use a random port.
	m.addr = "127.0.0.1:19090"
	require.NoError(t, m.Start())
	defer m.Stop(context.Background())

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://127.0.0.1:19090/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestReadyEndpoint(t *testing.T) {
	m := New("127.0.0.1:19091", zap.NewNop())
	require.NoError(t, m.Start())
	defer m.Stop(context.Background())

	time.Sleep(100 * time.Millisecond)

	// Not ready initially.
	resp, err := http.Get("http://127.0.0.1:19091/ready")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

	// Mark ready.
	m.SetReady()
	resp, err = http.Get("http://127.0.0.1:19091/ready")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestMetricsEndpoint(t *testing.T) {
	m := New("127.0.0.1:19092", zap.NewNop())
	require.NoError(t, m.Start())
	defer m.Stop(context.Background())

	time.Sleep(100 * time.Millisecond)

	m.MetricsCollected.Add(42)

	resp, err := http.Get("http://127.0.0.1:19092/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
