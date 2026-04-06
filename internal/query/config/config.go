package config

import (
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the query layer configuration.
type Config struct {
	HTTPAddr       string        `yaml:"http_addr"`
	ClickHouseDSN  string        `yaml:"clickhouse_dsn"`
	MetricsAddr    string        `yaml:"metrics_addr"`
	CacheTTL       time.Duration `yaml:"cache_ttl"`
	CacheMaxSize   int           `yaml:"cache_max_size"`
	RateLimit      int           `yaml:"rate_limit"`
	QueryTimeout   time.Duration `yaml:"query_timeout"`
}

// Defaults returns sensible defaults.
func Defaults() *Config {
	return &Config{
		HTTPAddr:      ":8081",
		ClickHouseDSN: "clickhouse://localhost:9000/default",
		MetricsAddr:   ":9092",
		CacheTTL:      30 * time.Second,
		CacheMaxSize:  1000,
		RateLimit:     100,
		QueryTimeout:  30 * time.Second,
	}
}

// Load reads configuration from file and environment overrides.
func Load() (*Config, error) {
	cfg := Defaults()
	configPath := envOrDefault("LENS_QUERY_CONFIG", "/etc/lens/query.yml")
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
	if v := os.Getenv("LENS_QUERY_HTTP_ADDR"); v != "" {
		cfg.HTTPAddr = v
	}
	if v := os.Getenv("LENS_CLICKHOUSE_DSN"); v != "" {
		cfg.ClickHouseDSN = v
	}
	if v := os.Getenv("LENS_QUERY_CACHE_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.CacheTTL = d
		}
	}
	if v := os.Getenv("LENS_QUERY_RATE_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.RateLimit = n
		}
	}
	if v := os.Getenv("LENS_QUERY_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.QueryTimeout = d
		}
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
