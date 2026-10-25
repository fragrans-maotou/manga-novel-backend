package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const requestIDLocalsKey = "request_id"
const requestIDHeader = "X-Request-ID"

// RequestID ensures every request has an X-Request-ID (generated if absent).
func RequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Get(requestIDHeader)
		if id == "" {
			id = uuid.New().String()
		}
		c.Locals(requestIDLocalsKey, id)
		c.Set(requestIDHeader, id)
		return c.Next()
	}
}

// RequestIDFromCtx returns the request id stored by RequestID middleware.
func RequestIDFromCtx(c *fiber.Ctx) string {
	if v, ok := c.Locals(requestIDLocalsKey).(string); ok {
		return v
	}
	return ""
}
