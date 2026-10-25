package handler

import (
	"strconv"

	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/domain"
	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/service"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// WorkHandler serves work listing, chapter catalog, and reading progress.
type WorkHandler struct {
	works    *service.WorkService
	progress *service.ProgressService
}

// NewWorkHandler injects WorkService and ProgressService.
func NewWorkHandler(works *service.WorkService, progress *service.ProgressService) *WorkHandler {
	if works == nil || progress == nil {
		panic("handler: NewWorkHandler: works and progress must not be nil")
	}
	return &WorkHandler{works: works, progress: progress}
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

type putProgressBody struct {
	ChapterID      string `json:"chapter_id"`
	PositionOffset int    `json:"position_offset"`
	UserID         string `json:"user_id"`
}

// PutProgress PUT /api/v1/works/:workID/progress
// user_id may come from JSON body, query, or X-User-ID header (demo auth stub).
func (h *WorkHandler) PutProgress(c *fiber.Ctx) error {
	workID, err := uuid.Parse(c.Params("workID"))
	if err != nil {
		return domain.BadRequest("invalid workID", err)
	}
	var body putProgressBody
	if err := c.BodyParser(&body); err != nil {
		return domain.BadRequest("invalid JSON body", err)
	}
	userRaw := firstNonEmpty(body.UserID, c.Query("user_id"), c.Get("X-User-ID"))
	userID, err := uuid.Parse(userRaw)
	if err != nil {
		return domain.BadRequest("user_id must be a UUID (body/query/X-User-ID)", err)
	}
	chapterID, err := uuid.Parse(body.ChapterID)
	if err != nil {
		return domain.BadRequest("chapter_id must be a UUID", err)
	}
	rp := &domain.ReadingProgress{
		UserID:         userID,
		WorkID:         workID,
		ChapterID:      chapterID,
		PositionOffset: body.PositionOffset,
	}
	out, err := h.progress.Upsert(c.UserContext(), rp)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// GetProgress GET /api/v1/works/:workID/progress?user_id=
func (h *WorkHandler) GetProgress(c *fiber.Ctx) error {
	workID, err := uuid.Parse(c.Params("workID"))
	if err != nil {
		return domain.BadRequest("invalid workID", err)
	}
	userRaw := firstNonEmpty(c.Query("user_id"), c.Get("X-User-ID"))
	userID, err := uuid.Parse(userRaw)
	if err != nil {
		return domain.BadRequest("user_id query/header must be a UUID", err)
	}
	out, err := h.progress.Get(c.UserContext(), userID, workID)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
