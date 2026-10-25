package middleware

import (
	"errors"
	"log/slog"

	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/domain"
	"github.com/gofiber/fiber/v2"
)

// NewErrorHandler returns Fiber's ErrorHandler that maps domain.AppError to JSON.
func NewErrorHandler(logger *slog.Logger) fiber.ErrorHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(c *fiber.Ctx, err error) error {
		code := fiber.StatusInternalServerError
		msg := "internal server error"

		var ae *domain.AppError
		var fe *fiber.Error
		switch {
		case errors.As(err, &ae):
			code = ae.Code
			msg = ae.Message
		case errors.As(err, &fe):
			code = fe.Code
			msg = fe.Message
		case errors.Is(err, domain.ErrNotFound):
			code = fiber.StatusNotFound
			msg = "not found"
		case errors.Is(err, domain.ErrInvalidInput):
			code = fiber.StatusBadRequest
			msg = "invalid input"
		case errors.Is(err, domain.ErrTimeout):
			code = fiber.StatusGatewayTimeout
			msg = "timeout"
		case errors.Is(err, domain.ErrUnavailable), errors.Is(err, domain.ErrCircuitOpen):
			code = fiber.StatusServiceUnavailable
			msg = "service unavailable"
		}

		if code >= 500 {
			logger.Error("request failed",
				"request_id", RequestIDFromCtx(c),
				"status", code,
				"err", err,
			)
		}

		return c.Status(code).JSON(fiber.Map{
			"error":      msg,
			"request_id": RequestIDFromCtx(c),
		})
	}
}
