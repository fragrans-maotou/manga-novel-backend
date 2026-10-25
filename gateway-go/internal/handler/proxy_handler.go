package handler

import (
	"bufio"
	"io"

	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/domain"
	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/service"
	"github.com/gofiber/fiber/v2"
)

// ProxyHandler streams manga images with optional XOR (SPEC §6.3).
type ProxyHandler struct {
	proxy *service.ProxyService
}

// NewProxyHandler injects ProxyService.
func NewProxyHandler(proxy *service.ProxyService) *ProxyHandler {
	if proxy == nil {
		panic("handler: NewProxyHandler: proxy must not be nil")
	}
	return &ProxyHandler{proxy: proxy}
}

// ProxyImage GET /api/v1/proxy/image/:ticket
// Uses SetBodyStreamWriter + fixed 32KiB buffer. Never io.ReadAll on image body.
func (h *ProxyHandler) ProxyImage(c *fiber.Ctx) error {
	ticket := c.Params("ticket")
	if ticket == "" {
		return domain.BadRequest("ticket is required", domain.ErrInvalidInput)
	}

	stream, err := h.proxy.OpenImage(c.UserContext(), ticket)
	if err != nil {
		return err
	}

	c.Set("Content-Type", stream.ContentType)
	c.Set("Cache-Control", "public, max-age=3600")

	xorKey := stream.XORKey
	xorOn := stream.XOREnabled
	body := stream.Body

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer func() { _ = body.Close() }()
		buf := make([]byte, service.ImageProxyBufferSize) // 32 * 1024
		for {
			n, readErr := body.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				if xorOn {
					service.ApplyXORInPlace(chunk, xorKey)
				}
				if _, err := w.Write(chunk); err != nil {
					return
				}
				if err := w.Flush(); err != nil {
					return
				}
			}
			if readErr != nil {
				if readErr == io.EOF {
					return
				}
				return
			}
		}
	})
	return nil
}
