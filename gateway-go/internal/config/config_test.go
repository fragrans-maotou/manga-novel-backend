package config

import (
	"testing"
	"time"
)

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_ADDR", "127.0.0.1:6379")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when DATABASE_URL is empty")
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/manga?sslmode=disable")
	t.Setenv("REDIS_ADDR", "127.0.0.1:6379")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("REDIS_DB", "")
	t.Setenv("STREAM_BLPOP_TIMEOUT", "")
	t.Setenv("CHAPTER_CACHE_TTL", "")
	t.Setenv("REDIS_PASSWORD", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.RedisDB != 0 {
		t.Fatalf("RedisDB = %d, want 0", cfg.RedisDB)
	}
	if cfg.ChapterCacheTTL != 86400*time.Second {
		t.Fatalf("ChapterCacheTTL = %v, want 86400s", cfg.ChapterCacheTTL)
	}
	if cfg.StreamBLPopTimeout != 30*time.Second {
		t.Fatalf("StreamBLPopTimeout = %v, want 30s", cfg.StreamBLPopTimeout)
	}
}

func TestLoadDurationAsSeconds(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	t.Setenv("REDIS_ADDR", "localhost:6379")
	t.Setenv("CHAPTER_CACHE_TTL", "3600")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ChapterCacheTTL != 3600*time.Second {
		t.Fatalf("ChapterCacheTTL = %v, want 3600s", cfg.ChapterCacheTTL)
	}
}
