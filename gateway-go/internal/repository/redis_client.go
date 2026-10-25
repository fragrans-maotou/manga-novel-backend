package repository

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// RedisOptions holds connection settings for NewRedisClient.
type RedisOptions struct {
	Addr     string
	Password string
	DB       int
}

// NewRedisClient constructs a go-redis client and verifies connectivity with PING.
// Caller owns Close(); no package-level Client/Redis globals.
func NewRedisClient(ctx context.Context, opts RedisOptions) (*redis.Client, error) {
	if opts.Addr == "" {
		return nil, fmt.Errorf("repository: redis addr is empty")
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     opts.Addr,
		Password: opts.Password,
		DB:       opts.DB,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("repository: ping redis: %w", err)
	}
	return rdb, nil
}
