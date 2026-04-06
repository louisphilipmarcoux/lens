package shipper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/louispm/lens/internal/agent/buffer"
)

func TestShipperSendsData(t *testing.T) {
	var receivedCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dir := t.TempDir()
	buf, err := buffer.NewDiskBuffer(dir, 1024, 1024*1024)
	require.NoError(t, err)

	// Write data and force a segment rotation.
	for i := 0; i < 3; i++ {
		require.NoError(t, buf.Write([]byte(`{"metrics":[{"name":"test"}]}`)))
	}
	require.NoError(t, buf.Close())

	// Reopen buffer to create a new active segment (old one becomes shippable).
	buf, err = buffer.NewDiskBuffer(dir, 1024, 1024*1024)
	require.NoError(t, err)
	defer buf.Close()

	s := New(buf, server.URL, 5*time.Second, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.shipOnce(ctx)

	assert.Equal(t, int32(3), receivedCount.Load())
	assert.Equal(t, int64(3), s.ShippedCount())
}

func TestBackoff(t *testing.T) {
	b0 := Backoff(0, 1*time.Second, 30*time.Second)
	assert.LessOrEqual(t, b0, 1*time.Second)
	assert.GreaterOrEqual(t, b0, 500*time.Millisecond)

	b3 := Backoff(3, 1*time.Second, 30*time.Second)
	assert.LessOrEqual(t, b3, 8*time.Second)

	b10 := Backoff(10, 1*time.Second, 30*time.Second)
	assert.LessOrEqual(t, b10, 30*time.Second)
}
