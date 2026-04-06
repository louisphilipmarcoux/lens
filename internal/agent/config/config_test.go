package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()
	assert.Equal(t, "/proc", cfg.ProcRoot)
	assert.Equal(t, 10*time.Second, cfg.CollectInterval)
	assert.Equal(t, ":4317", cfg.OTelGRPCAddr)
	assert.Equal(t, "http://localhost:8080", cfg.Backend.URL)
	assert.Equal(t, int64(64*1024*1024), cfg.Buffer.MaxSegmentSize)
	assert.Equal(t, 1000, cfg.Batch.MaxSize)
	assert.Equal(t, 5*time.Second, cfg.Batch.MaxWait)
	assert.Equal(t, ":9090", cfg.HTTPAddr)
}

func TestLoadFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "agent.yml")
	err := os.WriteFile(cfgFile, []byte(`
proc_root: /host/proc
collect_interval: 5s
otel_grpc_addr: ":5317"
backend:
  url: http://ingest:8080
  timeout: 10s
buffer:
  dir: /tmp/lens-buffer
  max_segment_size: 33554432
  max_total_size: 536870912
batch:
  max_size: 500
  max_wait: 2s
http_addr: ":8090"
log_files:
  - path: /var/log/app.log
    service: myapp
    format: json
`), 0644)
	require.NoError(t, err)

	t.Setenv("LENS_CONFIG", cfgFile)

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "/host/proc", cfg.ProcRoot)
	assert.Equal(t, 5*time.Second, cfg.CollectInterval)
	assert.Equal(t, ":5317", cfg.OTelGRPCAddr)
	assert.Equal(t, "http://ingest:8080", cfg.Backend.URL)
	assert.Equal(t, 10*time.Second, cfg.Backend.Timeout)
	assert.Equal(t, "/tmp/lens-buffer", cfg.Buffer.Dir)
	assert.Equal(t, int64(33554432), cfg.Buffer.MaxSegmentSize)
	assert.Equal(t, 500, cfg.Batch.MaxSize)
	assert.Equal(t, ":8090", cfg.HTTPAddr)
	assert.Len(t, cfg.LogFiles, 1)
	assert.Equal(t, "myapp", cfg.LogFiles[0].Service)
}

func TestEnvOverrides(t *testing.T) {
	t.Setenv("LENS_CONFIG", "/nonexistent/path.yml")
	t.Setenv("LENS_PROC_ROOT", "/custom/proc")
	t.Setenv("LENS_BACKEND_URL", "http://custom:9999")
	t.Setenv("LENS_BATCH_MAX_SIZE", "2000")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "/custom/proc", cfg.ProcRoot)
	assert.Equal(t, "http://custom:9999", cfg.Backend.URL)
	assert.Equal(t, 2000, cfg.Batch.MaxSize)
}
