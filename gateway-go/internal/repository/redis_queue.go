package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/domain"
	"github.com/redis/go-redis/v9"
)

// RedisScrapeQueue implements domain.ScrapeQueueRepository via LPUSH.
type RedisScrapeQueue struct {
	rdb *redis.Client
}

// NewRedisScrapeQueue injects a go-redis client (no package-level globals).
func NewRedisScrapeQueue(rdb *redis.Client) *RedisScrapeQueue {
	if rdb == nil {
		panic("repository: NewRedisScrapeQueue: rdb must not be nil")
	}
	return &RedisScrapeQueue{rdb: rdb}
}

// Enqueue JSON-marshals the task and LPUSHes onto queue:scrape:realtime|cron.
func (q *RedisScrapeQueue) Enqueue(ctx context.Context, priority domain.ScrapeQueuePriority, task *domain.ScrapeTask) error {
	if task == nil {
		return domain.BadRequest("scrape task is nil", domain.ErrInvalidInput)
	}
	if task.TaskID == "" || task.ChapterID == "" || task.RemoteURL == "" || task.SourceID == "" {
		return domain.BadRequest("scrape task missing required fields", domain.ErrInvalidInput)
	}
	key, err := queueKey(priority)
	if err != nil {
		return domain.BadRequest(err.Error(), domain.ErrInvalidInput)
	}
	payload, err := json.Marshal(task)
	if err != nil {
		return domain.Internal("marshal scrape task", err)
	}
	if err := q.rdb.LPush(ctx, key, payload).Err(); err != nil {
		return domain.Internal(fmt.Sprintf("LPUSH %s", key), err)
	}
	return nil
}
