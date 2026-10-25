package repository

import (
	"context"
	"testing"
	"time"

	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/domain"
)

func TestRedisStream_BlockingPop(t *testing.T) {
	mr, rdb := newTestRedis(t)
	s := NewRedisStream(rdb)
	taskID := "task-stream-1"
	key := keyStream(taskID)
	if key != "stream:task-stream-1" {
		t.Fatalf("key=%q", key)
	}
	mr.RPush(key, "READY")

	val, err := s.BlockingPop(context.Background(), taskID, time.Second)
	if err != nil {
		t.Fatalf("BlockingPop: %v", err)
	}
	if val != "READY" {
		t.Fatalf("val=%q want READY", val)
	}
}

func TestRedisChapterCache_SetGet(t *testing.T) {
	_, rdb := newTestRedis(t)
	c := NewRedisChapterCache(rdb, 0)
	ctx := context.Background()
	chapterID := "chap-1"
	if err := c.Set(ctx, chapterID, "hello body", 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := c.Get(ctx, chapterID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "hello body" {
		t.Fatalf("got=%q", got)
	}
	if keyChapterCache(chapterID) != "cache:chapter:chap-1" {
		t.Fatalf("unexpected cache key")
	}
}

func TestRedisProxyTicket_HGetAll(t *testing.T) {
	mr, rdb := newTestRedis(t)
	p := NewRedisProxyTicket(rdb)
	ticket := "a1b2c3d4e5f67890"
	key := keyProxyTicket(ticket)
	if key != "proxy:ticket:a1b2c3d4e5f67890" {
		t.Fatalf("key=%q", key)
	}
	mr.HSet(key, "url", "https://cdn.example/img.jpg")
	mr.HSet(key, "referer", "https://site.example/ch/1")
	mr.HSet(key, "ua", "Mozilla/5.0")
	mr.HSet(key, "xor_key", "12")

	got, err := p.Get(context.Background(), ticket)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.URL != "https://cdn.example/img.jpg" || got.XORKey != "12" {
		t.Fatalf("got=%+v", got)
	}
	_, err = p.Get(context.Background(), "missing")
	if err != domain.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
