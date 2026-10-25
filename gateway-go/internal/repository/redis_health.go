package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/domain"
	"github.com/redis/go-redis/v9"
)

const (
	healthScoreMin     = 0.0
	healthScoreMax     = 100.0
	healthCircuitFloor = 20.0
)

// RedisSourceHealth implements domain.SourceHealthRepository on health:sources:{media_type}.
type RedisSourceHealth struct {
	rdb *redis.Client
}

// NewRedisSourceHealth injects a go-redis client.
func NewRedisSourceHealth(rdb *redis.Client) *RedisSourceHealth {
	if rdb == nil {
		panic("repository: NewRedisSourceHealth: rdb must not be nil")
	}
	return &RedisSourceHealth{rdb: rdb}
}

// IncrBy ZINCRBY then clamps score into [0, 100] per SPEC §5.5.
func (h *RedisSourceHealth) IncrBy(ctx context.Context, mediaType domain.MediaType, sourceID string, delta float64) (float64, error) {
	if !mediaType.Valid() {
		return 0, domain.BadRequest("invalid media_type", domain.ErrInvalidInput)
	}
	if sourceID == "" {
		return 0, domain.BadRequest("source_id is empty", domain.ErrInvalidInput)
	}
	key := keyHealthSources(mediaType)
	score, err := h.rdb.ZIncrBy(ctx, key, delta, sourceID).Result()
	if err != nil {
		return 0, domain.Internal(fmt.Sprintf("ZINCRBY %s", key), err)
	}
	clamped := score
	if clamped > healthScoreMax {
		clamped = healthScoreMax
	}
	if clamped < healthScoreMin {
		clamped = healthScoreMin
	}
	if clamped != score {
		if err := h.rdb.ZAdd(ctx, key, redis.Z{Score: clamped, Member: sourceID}).Err(); err != nil {
			return 0, domain.Internal(fmt.Sprintf("ZADD clamp %s", key), err)
		}
		score = clamped
	}
	return score, nil
}

// Score returns the current health score, or ErrNotFound if the member is absent.
func (h *RedisSourceHealth) Score(ctx context.Context, mediaType domain.MediaType, sourceID string) (float64, error) {
	if !mediaType.Valid() {
		return 0, domain.BadRequest("invalid media_type", domain.ErrInvalidInput)
	}
	if sourceID == "" {
		return 0, domain.BadRequest("source_id is empty", domain.ErrInvalidInput)
	}
	key := keyHealthSources(mediaType)
	score, err := h.rdb.ZScore(ctx, key, sourceID).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, domain.ErrNotFound
		}
		return 0, domain.Internal(fmt.Sprintf("ZSCORE %s", key), err)
	}
	return score, nil
}

// Top returns up to n source_ids ordered by score descending (highest health first).
func (h *RedisSourceHealth) Top(ctx context.Context, mediaType domain.MediaType, n int64) ([]string, error) {
	if !mediaType.Valid() {
		return nil, domain.BadRequest("invalid media_type", domain.ErrInvalidInput)
	}
	if n <= 0 {
		return nil, domain.BadRequest("n must be > 0", domain.ErrInvalidInput)
	}
	key := keyHealthSources(mediaType)
	members, err := h.rdb.ZRevRange(ctx, key, 0, n-1).Result()
	if err != nil {
		return nil, domain.Internal(fmt.Sprintf("ZREVRANGE %s", key), err)
	}
	return members, nil
}

// BelowCircuitFloor reports whether score is under the SPEC §5.5 circuit threshold (20).
func BelowCircuitFloor(score float64) bool {
	return score < healthCircuitFloor
}
