package handler

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// HealthHandler serves /healthz.
type HealthHandler struct {
	pool *pgxpool.Pool
	rdb  *redis.Client
}

// NewHealthHandler injects optional deps for liveness checks.
func NewHealthHandler(pool *pgxpool.Pool, rdb *redis.Client) *HealthHandler {
	return &HealthHandler{pool: pool, rdb: rdb}
}

// Healthz returns 200 when process is up; includes ping results when deps set.
func (h *HealthHandler) Healthz(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), 2*time.Second)
	defer cancel()

	status := fiber.Map{"status": "ok"}
	ok := true
	if h.pool != nil {
		if err := h.pool.Ping(ctx); err != nil {
			status["postgres"] = "down"
			ok = false
		} else {
			status["postgres"] = "up"
		}
	}
	if h.rdb != nil {
		if err := h.rdb.Ping(ctx).Err(); err != nil {
			status["redis"] = "down"
			ok = false
		} else {
			status["redis"] = "up"
		}
	}
	if !ok {
		status["status"] = "degraded"
		return c.Status(fiber.StatusServiceUnavailable).JSON(status)
	}
	return c.JSON(status)
}
