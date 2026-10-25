package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/domain"
	"github.com/redis/go-redis/v9"
)

// RedisChapterCache implements domain.ChapterCacheRepository on cache:chapter:{id}.
type RedisChapterCache struct {
	rdb        *redis.Client
	defaultTTL time.Duration
}

// NewRedisChapterCache injects redis client and default TTL (SPEC §5.3: 86400s).
func NewRedisChapterCache(rdb *redis.Client, defaultTTL time.Duration) *RedisChapterCache {
	if rdb == nil {
		panic("repository: NewRedisChapterCache: rdb must not be nil")
	}
	if defaultTTL <= 0 {
		defaultTTL = time.Duration(chapterCacheTTLSeconds) * time.Second
	}
	return &RedisChapterCache{rdb: rdb, defaultTTL: defaultTTL}
}

func (c *RedisChapterCache) Get(ctx context.Context, chapterID string) (string, error) {
	if chapterID == "" {
		return "", domain.BadRequest("chapter_id is empty", domain.ErrInvalidInput)
	}
	key := keyChapterCache(chapterID)
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", domain.ErrNotFound
		}
		return "", domain.Internal(fmt.Sprintf("GET %s", key), err)
	}
	return val, nil
}

func (c *RedisChapterCache) Set(ctx context.Context, chapterID, value string, ttl time.Duration) error {
	if chapterID == "" {
		return domain.BadRequest("chapter_id is empty", domain.ErrInvalidInput)
	}
	if ttl <= 0 {
		ttl = c.defaultTTL
	}
	key := keyChapterCache(chapterID)
	if err := c.rdb.Set(ctx, key, value, ttl).Err(); err != nil {
		return domain.Internal(fmt.Sprintf("SET %s", key), err)
	}
	return nil
}

func (c *RedisChapterCache) Delete(ctx context.Context, chapterID string) error {
	if chapterID == "" {
		return domain.BadRequest("chapter_id is empty", domain.ErrInvalidInput)
	}
	key := keyChapterCache(chapterID)
	if err := c.rdb.Del(ctx, key).Err(); err != nil {
		return domain.Internal(fmt.Sprintf("DEL %s", key), err)
	}
	return nil
}
