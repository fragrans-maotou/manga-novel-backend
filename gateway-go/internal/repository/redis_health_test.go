package repository

import (
	"context"
	"testing"

	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/domain"
)

func TestRedisSourceHealth_ZIncrByAndClamp(t *testing.T) {
	_, rdb := newTestRedis(t)
	h := NewRedisSourceHealth(rdb)
	ctx := context.Background()

	score, err := h.IncrBy(ctx, domain.MediaTypeNovel, "src_a", 100)
	if err != nil {
		t.Fatalf("IncrBy init: %v", err)
	}
	if score != 100 {
		t.Fatalf("score=%v want 100", score)
	}

	// success +1 but max 100
	score, err = h.IncrBy(ctx, domain.MediaTypeNovel, "src_a", 1)
	if err != nil {
		t.Fatalf("IncrBy +1: %v", err)
	}
	if score != 100 {
		t.Fatalf("score after +1 clamp=%v want 100", score)
	}

	// failure -20
	score, err = h.IncrBy(ctx, domain.MediaTypeNovel, "src_a", -20)
	if err != nil {
		t.Fatalf("IncrBy -20: %v", err)
	}
	if score != 80 {
		t.Fatalf("score=%v want 80", score)
	}

	got, err := h.Score(ctx, domain.MediaTypeNovel, "src_a")
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if got != 80 {
		t.Fatalf("Score=%v want 80", got)
	}

	key := keyHealthSources(domain.MediaTypeNovel)
	if key != "health:sources:novel" {
		t.Fatalf("key=%q want health:sources:novel", key)
	}
}

func TestRedisSourceHealth_TopAndCircuit(t *testing.T) {
	_, rdb := newTestRedis(t)
	h := NewRedisSourceHealth(rdb)
	ctx := context.Background()

	_, _ = h.IncrBy(ctx, domain.MediaTypeManga, "low", 10)
	_, _ = h.IncrBy(ctx, domain.MediaTypeManga, "high", 90)

	top, err := h.Top(ctx, domain.MediaTypeManga, 2)
	if err != nil {
		t.Fatalf("Top: %v", err)
	}
	if len(top) != 2 || top[0] != "high" || top[1] != "low" {
		t.Fatalf("Top=%v want [high low]", top)
	}
	if !BelowCircuitFloor(10) {
		t.Fatal("expected score 10 below circuit floor")
	}
	if BelowCircuitFloor(20) {
		t.Fatal("score 20 should not be below floor (<20)")
	}
}
