package handler

import (
	"bufio"
	"fmt"

	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/domain"
	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/service"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// ChapterHandler serves novel SSE and manga chapter JSON.
type ChapterHandler struct {
	chapters *service.ChapterService
}

// NewChapterHandler injects ChapterService.
func NewChapterHandler(chapters *service.ChapterService) *ChapterHandler {
	if chapters == nil {
		panic("handler: NewChapterHandler: chapters must not be nil")
	}
	return &ChapterHandler{chapters: chapters}
}

// GetMangaChapter GET /api/v1/chapters/:chapterID/manga
func (h *ChapterHandler) GetMangaChapter(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("chapterID"))
	if err != nil {
		return domain.BadRequest("invalid chapterID", err)
	}
	out, err := h.chapters.GetMangaChapter(c.UserContext(), id)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// StreamNovel GET /api/v1/chapters/:chapterID/novel  (text/event-stream)
func (h *ChapterHandler) StreamNovel(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("chapterID"))
	if err != nil {
		return domain.BadRequest("invalid chapterID", err)
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	ctx := c.UserContext()
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		emitErr := h.chapters.StreamNovel(ctx, id, func(chunk string) error {
			if _, err := fmt.Fprintf(w, "data: %s\n\n", escapeSSE(chunk)); err != nil {
				return err
			}
			return w.Flush()
		})
		if emitErr != nil {
			_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", escapeSSE(emitErr.Error()))
			_ = w.Flush()
			return
		}
		_, _ = fmt.Fprintf(w, "event: done\ndata: %s\n\n", streamDoneData)
		_ = w.Flush()
	})
	return nil
}

const streamDoneData = "[DONE]"

func escapeSSE(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, '\\', 'n')
			continue
		}
		if s[i] == '\r' {
			continue
		}
		out = append(out, s[i])
	}
	return string(out)
}
