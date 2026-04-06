package config

import (
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all agent configuration.
type Config struct {
	ProcRoot        string          `yaml:"proc_root"`
	CollectInterval time.Duration   `yaml:"collect_interval"`
	LogFiles        []LogFileConfig `yaml:"log_files"`
	OTelGRPCAddr    string          `yaml:"otel_grpc_addr"`
	Backend         BackendConfig   `yaml:"backend"`
	Buffer          BufferConfig    `yaml:"buffer"`
	Batch           BatchConfig     `yaml:"batch"`
	HTTPAddr        string          `yaml:"http_addr"`
}

// LogFileConfig defines a log file to tail.
type LogFileConfig struct {
	Path    string `yaml:"path"`
	Service string `yaml:"service"`
	Format  string `yaml:"format"`
	Pattern string `yaml:"pattern,omitempty"`
}

// BackendConfig defines the ingestion backend connection.
type BackendConfig struct {
	URL     string        `yaml:"url"`
	Timeout time.Duration `yaml:"timeout"`
}

// BufferConfig defines the disk-backed buffer settings.
type BufferConfig struct {
	Dir            string `yaml:"dir"`
	MaxSegmentSize int64  `yaml:"max_segment_size"`
	MaxTotalSize   int64  `yaml:"max_total_size"`
}

// BatchConfig defines batching behavior.
type BatchConfig struct {
	MaxSize int           `yaml:"max_size"`
	MaxWait time.Duration `yaml:"max_wait"`
}

// Defaults returns a Config with sensible default values.
func Defaults() *Config {
	return &Config{
		ProcRoot:        "/proc",
		CollectInterval: 10 * time.Second,
		OTelGRPCAddr:    ":4317",
		Backend: BackendConfig{
			URL:     "http://localhost:8080",
			Timeout: 30 * time.Second,
		},
		Buffer: BufferConfig{
			Dir:            "/var/lib/lens/buffer",
			MaxSegmentSize: 64 * 1024 * 1024,
			MaxTotalSize:   1024 * 1024 * 1024,
		},
		Batch: BatchConfig{
			MaxSize: 1000,
			MaxWait: 5 * time.Second,
		},
		HTTPAddr: ":9090",
	}
}

// Load reads configuration from the default config file path and
// environment variable overrides. If no config file is found, defaults are used.
func Load() (*Config, error) {
	cfg := Defaults()

	configPath := envOrDefault("LENS_CONFIG", "/etc/lens/agent.yml")
	data, err := os.ReadFile(configPath)
	if err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	applyEnvOverrides(cfg)
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("LENS_PROC_ROOT"); v != "" {
		cfg.ProcRoot = v
	}
	if v := os.Getenv("LENS_COLLECT_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.CollectInterval = d
		}
	}
	if v := os.Getenv("LENS_OTEL_GRPC_ADDR"); v != "" {
		cfg.OTelGRPCAddr = v
	}
	if v := os.Getenv("LENS_BACKEND_URL"); v != "" {
		cfg.Backend.URL = v
	}
	if v := os.Getenv("LENS_BACKEND_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Backend.Timeout = d
		}
	}
	if v := os.Getenv("LENS_BUFFER_DIR"); v != "" {
		cfg.Buffer.Dir = v
	}
	if v := os.Getenv("LENS_BUFFER_MAX_SEGMENT_SIZE"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.Buffer.MaxSegmentSize = n
		}
	}
	if v := os.Getenv("LENS_BUFFER_MAX_TOTAL_SIZE"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.Buffer.MaxTotalSize = n
		}
	}
	if v := os.Getenv("LENS_BATCH_MAX_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Batch.MaxSize = n
		}
	}
	if v := os.Getenv("LENS_BATCH_MAX_WAIT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Batch.MaxWait = d
		}
	}
	if v := os.Getenv("LENS_HTTP_ADDR"); v != "" {
		cfg.HTTPAddr = v
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
