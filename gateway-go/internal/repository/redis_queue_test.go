package repository

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/domain"
	"github.com/redis/go-redis/v9"
)

func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

func TestRedisScrapeQueue_EnqueueRealtimeLPUSH(t *testing.T) {
	mr, rdb := newTestRedis(t)
	q := NewRedisScrapeQueue(rdb)

	task := &domain.ScrapeTask{
		TaskID:    "11111111-1111-1111-1111-111111111111",
		MediaType: "novel",
		SourceID:  "example_source",
		ChapterID: "22222222-2222-2222-2222-222222222222",
		RemoteURL: "https://target-site.com/read/1001.html",
		TimeoutMS: 8000,
	}

	if err := q.Enqueue(context.Background(), domain.ScrapeQueueRealtime, task); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	raw, err := mr.Lpop(KeyQueueScrapeRealtime)
	if err != nil {
		t.Fatalf("Lpop realtime queue: %v", err)
	}
	var got domain.ScrapeTask
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.TaskID != task.TaskID || got.SourceID != task.SourceID || got.RemoteURL != task.RemoteURL {
		t.Fatalf("got %+v, want %+v", got, task)
	}
}

func TestRedisScrapeQueue_EnqueueCronLPUSH(t *testing.T) {
	mr, rdb := newTestRedis(t)
	q := NewRedisScrapeQueue(rdb)

	task := &domain.ScrapeTask{
		TaskID:    "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		MediaType: "manga",
		SourceID:  "manga_src",
		ChapterID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		RemoteURL: "https://example.com/ch/1",
		TimeoutMS: 5000,
	}
	if err := q.Enqueue(context.Background(), domain.ScrapeQueueCron, task); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if n, err := mr.List(KeyQueueScrapeCron); err != nil || len(n) != 1 {
		t.Fatalf("cron queue len=%v err=%v", n, err)
	}
}

func TestRedisScrapeQueue_RejectsNilTask(t *testing.T) {
	_, rdb := newTestRedis(t)
	q := NewRedisScrapeQueue(rdb)
	err := q.Enqueue(context.Background(), domain.ScrapeQueueRealtime, nil)
	if err == nil {
		t.Fatal("expected error for nil task")
	}
}
