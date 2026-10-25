package service

import (
	"context"
	"errors"

	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/domain"
	"github.com/google/uuid"
)

// WorkService exposes work listing and chapter catalogs.
type WorkService struct {
	works       domain.WorkRepository
	workSources domain.WorkSourceRepository
	chapters    domain.ChapterRepository
}

// NewWorkService injects repositories.
func NewWorkService(works domain.WorkRepository, workSources domain.WorkSourceRepository, chapters domain.ChapterRepository) *WorkService {
	if works == nil || workSources == nil || chapters == nil {
		panic("service: NewWorkService: repositories must not be nil")
	}
	return &WorkService{works: works, workSources: workSources, chapters: chapters}
}

// ListWorks returns a page of works, optionally filtered by media_type.
func (s *WorkService) ListWorks(ctx context.Context, mediaType *domain.MediaType, limit, offset int) ([]domain.Work, error) {
	if mediaType != nil && !mediaType.Valid() {
		return nil, domain.BadRequest("invalid media_type", domain.ErrInvalidInput)
	}
	return s.works.List(ctx, mediaType, limit, offset)
}

// GetWork returns one work by id.
func (s *WorkService) GetWork(ctx context.Context, id uuid.UUID) (*domain.Work, error) {
	w, err := s.works.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NotFound("work not found", err)
		}
		return nil, err
	}
	return w, nil
}

// ChapterCatalogItem is a chapter row enriched for API responses.
type ChapterCatalogItem struct {
	ID         uuid.UUID `json:"id"`
	ChapterNum float64   `json:"chapter_num"`
	Title      string    `json:"title"`
	SourceID   string    `json:"source_id"`
	HasStorage bool      `json:"has_storage"`
}

// ListChapters returns chapter indexes across all sources bound to the work.
// Prefer the highest-listed source order from work_sources; duplicates by chapter_num
// keep the first occurrence (caller may later refine by health).
func (s *WorkService) ListChapters(ctx context.Context, workID uuid.UUID) ([]ChapterCatalogItem, error) {
	if _, err := s.works.GetByID(ctx, workID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NotFound("work not found", err)
		}
		return nil, err
	}
	mappings, err := s.workSources.ListByWorkID(ctx, workID)
	if err != nil {
		return nil, err
	}
	seen := make(map[float64]struct{})
	out := make([]ChapterCatalogItem, 0)
	for _, m := range mappings {
		chs, err := s.chapters.ListByWorkSource(ctx, m.ID)
		if err != nil {
			return nil, err
		}
		for _, ch := range chs {
			if _, ok := seen[ch.ChapterNum]; ok {
				continue
			}
			seen[ch.ChapterNum] = struct{}{}
			out = append(out, ChapterCatalogItem{
				ID:         ch.ID,
				ChapterNum: ch.ChapterNum,
				Title:      ch.Title,
				SourceID:   m.SourceID,
				HasStorage: ch.StorageKey != nil && *ch.StorageKey != "",
			})
		}
	}
	// Sort by chapter_num ascending (insertion may interleave sources).
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].ChapterNum < out[i].ChapterNum {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}
