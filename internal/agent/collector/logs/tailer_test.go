package logs

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/louispm/lens/internal/common/model"
)

func TestTailerReadsNewLines(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "app.log")

	require.NoError(t, os.WriteFile(logFile, []byte(""), 0644))

	var mu sync.Mutex
	var entries []model.LogEntry
	handler := func(entry model.LogEntry) {
		mu.Lock()
		entries = append(entries, entry)
		mu.Unlock()
	}

	tailer := NewTailer([]TailerConfig{
		{Path: logFile, Service: "testapp", Format: "raw"},
	}, handler, dir, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	tailer.Start(ctx)

	// Write lines to the log file.
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, 0644)
	require.NoError(t, err)
	_, _ = f.WriteString("line one\nline two\n")
	f.Close()

	// Wait for tailer to pick up lines.
	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(entries) >= 2
	}, 5*time.Second, 100*time.Millisecond)

	cancel()
	tailer.Stop()

	mu.Lock()
	assert.Equal(t, "line one", entries[0].Message)
	assert.Equal(t, "line two", entries[1].Message)
	assert.Equal(t, "testapp", entries[0].Service)
	mu.Unlock()
}

func TestTailerJSONParsing(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "app.log")

	jsonLine := `{"level":"error","message":"something broke","request_id":"abc123"}` + "\n"
	require.NoError(t, os.WriteFile(logFile, []byte(jsonLine), 0644))

	var mu sync.Mutex
	var entries []model.LogEntry
	handler := func(entry model.LogEntry) {
		mu.Lock()
		entries = append(entries, entry)
		mu.Unlock()
	}

	tailer := NewTailer([]TailerConfig{
		{Path: logFile, Service: "testapp", Format: "json"},
	}, handler, dir, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	tailer.Start(ctx)

	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(entries) >= 1
	}, 5*time.Second, 100*time.Millisecond)

	cancel()
	tailer.Stop()

	mu.Lock()
	assert.Equal(t, "something broke", entries[0].Message)
	assert.Equal(t, "error", entries[0].Level)
	assert.Equal(t, "abc123", entries[0].Fields["request_id"])
	mu.Unlock()
}
