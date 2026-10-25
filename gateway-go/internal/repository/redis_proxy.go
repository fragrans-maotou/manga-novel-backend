package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/domain"
	"github.com/redis/go-redis/v9"
)

// RedisProxyTicket implements domain.ProxyTicketRepository via HGETALL.
type RedisProxyTicket struct {
	rdb *redis.Client
}

// NewRedisProxyTicket injects a go-redis client.
func NewRedisProxyTicket(rdb *redis.Client) *RedisProxyTicket {
	if rdb == nil {
		panic("repository: NewRedisProxyTicket: rdb must not be nil")
	}
	return &RedisProxyTicket{rdb: rdb}
}

// Get loads proxy:ticket:{ticket} hash fields: url, referer, ua, xor_key.
func (p *RedisProxyTicket) Get(ctx context.Context, ticket string) (*domain.ProxyTicket, error) {
	if ticket == "" {
		return nil, domain.BadRequest("ticket is empty", domain.ErrInvalidInput)
	}
	key := keyProxyTicket(ticket)
	fields, err := p.rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, domain.Internal(fmt.Sprintf("HGETALL %s", key), err)
	}
	if len(fields) == 0 {
		return nil, domain.ErrNotFound
	}
	url := fields["url"]
	if url == "" {
		return nil, domain.Internal(fmt.Sprintf("%s missing url field", key), errors.New("incomplete proxy ticket"))
	}
	return &domain.ProxyTicket{
		URL:     url,
		Referer: fields["referer"],
		UA:      fields["ua"],
		XORKey:  fields["xor_key"],
	}, nil
}
