package logs

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/louispm/lens/internal/common/model"
)

// LogHandler is called for each new log entry.
type LogHandler func(entry model.LogEntry)

// TailerConfig holds the configuration for a single log file.
type TailerConfig struct {
	Path    string
	Service string
	Format  string // "json", "raw"
}

// Tailer watches log files for new lines and parses them.
type Tailer struct {
	configs  []TailerConfig
	handler  LogHandler
	logger   *zap.Logger
	hostname string
	stateDir string

	mu      sync.Mutex
	offsets map[string]int64
	cancel  context.CancelFunc
}

// NewTailer creates a log tailer for the given files.
func NewTailer(configs []TailerConfig, handler LogHandler, stateDir string, logger *zap.Logger) *Tailer {
	hostname, _ := os.Hostname()
	return &Tailer{
		configs:  configs,
		handler:  handler,
		logger:   logger,
		hostname: hostname,
		stateDir: stateDir,
		offsets:  make(map[string]int64),
	}
}

// Start begins tailing all configured log files.
func (t *Tailer) Start(ctx context.Context) {
	ctx, t.cancel = context.WithCancel(ctx)
	t.loadOffsets()

	for _, cfg := range t.configs {
		go t.tailFile(ctx, cfg)
	}
}

// Stop stops the tailer and saves offsets.
func (t *Tailer) Stop() {
	if t.cancel != nil {
		t.cancel()
	}
	t.saveOffsets()
}

func (t *Tailer) tailFile(ctx context.Context, cfg TailerConfig) {
	t.mu.Lock()
	offset := t.offsets[cfg.Path]
	t.mu.Unlock()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		f, err := os.Open(cfg.Path)
		if err != nil {
			t.logger.Warn("cannot open log file", zap.String("path", cfg.Path), zap.Error(err))
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				continue
			}
		}

		// Check for truncation (log rotation).
		info, err := f.Stat()
		if err == nil && info.Size() < offset {
			offset = 0
		}

		if offset > 0 {
			f.Seek(offset, io.SeekStart)
		}

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			entry := t.parseLine(line, cfg)
			t.handler(entry)
		}

		newOffset, _ := f.Seek(0, io.SeekCurrent)
		t.mu.Lock()
		t.offsets[cfg.Path] = newOffset
		t.mu.Unlock()
		offset = newOffset

		f.Close()

		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Second):
		}
	}
}

func (t *Tailer) parseLine(line string, cfg TailerConfig) model.LogEntry {
	entry := model.LogEntry{
		Timestamp: time.Now(),
		Service:   cfg.Service,
		Host:      t.hostname,
		Source:    cfg.Path,
		Message:   line,
	}

	if cfg.Format == "json" {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(line), &parsed); err == nil {
			if msg, ok := parsed["message"].(string); ok {
				entry.Message = msg
				delete(parsed, "message")
			}
			if msg, ok := parsed["msg"].(string); ok {
				entry.Message = msg
				delete(parsed, "msg")
			}
			if lvl, ok := parsed["level"].(string); ok {
				entry.Level = lvl
				delete(parsed, "level")
			}
			if ts, ok := parsed["timestamp"].(string); ok {
				if t, err := time.Parse(time.RFC3339, ts); err == nil {
					entry.Timestamp = t
				}
				delete(parsed, "timestamp")
			}
			if ts, ok := parsed["ts"].(string); ok {
				if t, err := time.Parse(time.RFC3339, ts); err == nil {
					entry.Timestamp = t
				}
				delete(parsed, "ts")
			}
			entry.Fields = parsed
		}
	}

	return entry
}

func (t *Tailer) loadOffsets() {
	path := filepath.Join(t.stateDir, "tailer-state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	json.Unmarshal(data, &t.offsets)
}

func (t *Tailer) saveOffsets() {
	if t.stateDir == "" {
		return
	}
	os.MkdirAll(t.stateDir, 0755)
	t.mu.Lock()
	data, _ := json.Marshal(t.offsets)
	t.mu.Unlock()
	path := filepath.Join(t.stateDir, "tailer-state.json")
	os.WriteFile(path, data, 0644)
}
