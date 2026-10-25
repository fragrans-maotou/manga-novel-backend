package handler

import (
	"strconv"

	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/domain"
	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/service"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// WorkHandler serves work listing and chapter catalog endpoints.
type WorkHandler struct {
	works *service.WorkService
}

// NewWorkHandler injects WorkService.
func NewWorkHandler(works *service.WorkService) *WorkHandler {
	if works == nil {
		panic("handler: NewWorkHandler: works must not be nil")
	}
	return &WorkHandler{works: works}
}

// ListWorks GET /api/v1/works?media_type=&limit=&offset=
func (h *WorkHandler) ListWorks(c *fiber.Ctx) error {
	var media *domain.MediaType
	if raw := c.Query("media_type"); raw != "" {
		mt := domain.MediaType(raw)
		if !mt.Valid() {
			return domain.BadRequest("invalid media_type", domain.ErrInvalidInput)
		}
		media = &mt
	}
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	items, err := h.works.ListWorks(c.UserContext(), media, limit, offset)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"items": items})
}

// GetWork GET /api/v1/works/:workID
func (h *WorkHandler) GetWork(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("workID"))
	if err != nil {
		return domain.BadRequest("invalid workID", err)
	}
	w, err := h.works.GetWork(c.UserContext(), id)
	if err != nil {
		return err
	}
	return c.JSON(w)
}

// ListChapters GET /api/v1/works/:workID/chapters
func (h *WorkHandler) ListChapters(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("workID"))
	if err != nil {
		return domain.BadRequest("invalid workID", err)
	}
	items, err := h.works.ListChapters(c.UserContext(), id)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"items": items})
}
