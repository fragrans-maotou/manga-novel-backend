package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/domain"
	"github.com/redis/go-redis/v9"
)

// RedisStream implements domain.StreamRepository via BLPOP on stream:{task_id}.
type RedisStream struct {
	rdb *redis.Client
}

// NewRedisStream injects a go-redis client.
func NewRedisStream(rdb *redis.Client) *RedisStream {
	if rdb == nil {
		panic("repository: NewRedisStream: rdb must not be nil")
	}
	return &RedisStream{rdb: rdb}
}

// BlockingPop BLPOP from stream:{task_id}. Returns domain.ErrTimeout on wait expiry.
func (s *RedisStream) BlockingPop(ctx context.Context, taskID string, timeout time.Duration) (string, error) {
	if taskID == "" {
		return "", domain.BadRequest("task_id is empty", domain.ErrInvalidInput)
	}
	if timeout < 0 {
		return "", domain.BadRequest("timeout must be >= 0", domain.ErrInvalidInput)
	}
	key := keyStream(taskID)
	res, err := s.rdb.BLPop(ctx, timeout, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", domain.ErrTimeout
		}
		return "", domain.Internal(fmt.Sprintf("BLPOP %s", key), err)
	}
	// BLPop returns [key, value]
	if len(res) < 2 {
		return "", domain.Internal(fmt.Sprintf("BLPOP %s: unexpected reply length %d", key, len(res)), nil)
	}
	return res[1], nil
}
