package config

import (
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the ingestion backend configuration.
type Config struct {
	HTTPAddr       string        `yaml:"http_addr"`
	ClickHouseDSN  string        `yaml:"clickhouse_dsn"`
	FlushInterval  time.Duration `yaml:"flush_interval"`
	BatchMaxSize   int           `yaml:"batch_max_size"`
	RateLimit      int           `yaml:"rate_limit"`
	MetricsAddr    string        `yaml:"metrics_addr"`
}

// Defaults returns a Config with sensible defaults.
func Defaults() *Config {
	return &Config{
		HTTPAddr:      ":8080",
		ClickHouseDSN: "clickhouse://localhost:9000/default",
		FlushInterval: 5 * time.Second,
		BatchMaxSize:  10000,
		RateLimit:     100000,
		MetricsAddr:   ":9091",
	}
}

// Load reads configuration from file and environment overrides.
func Load() (*Config, error) {
	cfg := Defaults()

	configPath := envOrDefault("LENS_INGEST_CONFIG", "/etc/lens/ingest.yml")
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
	if v := os.Getenv("LENS_INGEST_HTTP_ADDR"); v != "" {
		cfg.HTTPAddr = v
	}
	if v := os.Getenv("LENS_CLICKHOUSE_DSN"); v != "" {
		cfg.ClickHouseDSN = v
	}
	if v := os.Getenv("LENS_INGEST_FLUSH_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.FlushInterval = d
		}
	}
	if v := os.Getenv("LENS_INGEST_BATCH_MAX_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.BatchMaxSize = n
		}
	}
	if v := os.Getenv("LENS_INGEST_RATE_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.RateLimit = n
		}
	}
	if v := os.Getenv("LENS_INGEST_METRICS_ADDR"); v != "" {
		cfg.MetricsAddr = v
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
