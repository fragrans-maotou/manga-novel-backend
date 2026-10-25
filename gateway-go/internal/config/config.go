package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds process configuration loaded from environment variables.
// All dependencies are constructed from this value in cmd/api/main.go (DI).
type Config struct {
	HTTPAddr string

	DatabaseURL string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// StreamBLPopTimeout is the default BLPOP wait used by stream consumers.
	StreamBLPopTimeout time.Duration

	// ChapterCacheTTL matches SPEC §5.3 (86400s) unless overridden.
	ChapterCacheTTL time.Duration

	// ScrapeTimeoutMS is written into scrape task payloads (SPEC §5.1 example: 8000).
	ScrapeTimeoutMS int

	// ShutdownTimeout bounds graceful shutdown after SIGINT/SIGTERM.
	ShutdownTimeout time.Duration

	// ProxyHTTPTimeout bounds upstream image fetches in the proxy path.
	ProxyHTTPTimeout time.Duration
}

// Load reads configuration from environment variables.
// Required: DATABASE_URL, REDIS_ADDR.
func Load() (*Config, error) {
	cfg := &Config{
		HTTPAddr:           getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:        strings.TrimSpace(os.Getenv("DATABASE_URL")),
		RedisAddr:          getenv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:      os.Getenv("REDIS_PASSWORD"),
		RedisDB:            getenvInt("REDIS_DB", 0),
		StreamBLPopTimeout: getenvDuration("STREAM_BLPOP_TIMEOUT", 30*time.Second),
		ChapterCacheTTL:    getenvDuration("CHAPTER_CACHE_TTL", 86400*time.Second),
		ScrapeTimeoutMS:    getenvInt("SCRAPE_TIMEOUT_MS", 8000),
		ShutdownTimeout:    getenvDuration("SHUTDOWN_TIMEOUT", 15*time.Second),
		ProxyHTTPTimeout:   getenvDuration("PROXY_HTTP_TIMEOUT", 30*time.Second),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("config: DATABASE_URL is required")
	}
	if cfg.RedisAddr == "" {
		return nil, fmt.Errorf("config: REDIS_ADDR is required")
	}
	if cfg.RedisDB < 0 {
		return nil, fmt.Errorf("config: REDIS_DB must be >= 0")
	}
	if cfg.ScrapeTimeoutMS <= 0 {
		return nil, fmt.Errorf("config: SCRAPE_TIMEOUT_MS must be > 0")
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	sec, err := strconv.Atoi(v)
	if err != nil || sec < 0 {
		return fallback
	}
	return time.Duration(sec) * time.Second
}
