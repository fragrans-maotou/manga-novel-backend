package middleware

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
)

// AccessLog writes a structured slog line after each request.
func AccessLog(logger *slog.Logger) fiber.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		logger.Info("http",
			"request_id", RequestIDFromCtx(c),
			"method", c.Method(),
			"path", c.Path(),
			"status", c.Response().StatusCode(),
			"latency_ms", time.Since(start).Milliseconds(),
			"ip", c.IP(),
		)
		return err
	}
}
