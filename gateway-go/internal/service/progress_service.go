package service

import (
	"context"
	"errors"

	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/domain"
	"github.com/google/uuid"
)

// ProgressService manages reading_progress rows.
type ProgressService struct {
	progress domain.ReadingProgressRepository
	works    domain.WorkRepository
	chapters domain.ChapterRepository
}

// NewProgressService injects repositories.
func NewProgressService(
	progress domain.ReadingProgressRepository,
	works domain.WorkRepository,
	chapters domain.ChapterRepository,
) *ProgressService {
	if progress == nil || works == nil || chapters == nil {
		panic("service: NewProgressService: repositories must not be nil")
	}
	return &ProgressService{progress: progress, works: works, chapters: chapters}
}

// Get returns progress for (userID, workID).
func (s *ProgressService) Get(ctx context.Context, userID, workID uuid.UUID) (*domain.ReadingProgress, error) {
	if _, err := s.works.GetByID(ctx, workID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NotFound("work not found", err)
		}
		return nil, err
	}
	rp, err := s.progress.Get(ctx, userID, workID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NotFound("reading progress not found", err)
		}
		return nil, err
	}
	return rp, nil
}

// Upsert stores reading progress after validating work + chapter exist.
func (s *ProgressService) Upsert(ctx context.Context, rp *domain.ReadingProgress) (*domain.ReadingProgress, error) {
	if rp == nil {
		return nil, domain.BadRequest("progress is nil", domain.ErrInvalidInput)
	}
	if rp.UserID == uuid.Nil || rp.WorkID == uuid.Nil || rp.ChapterID == uuid.Nil {
		return nil, domain.BadRequest("user_id, work_id, chapter_id are required", domain.ErrInvalidInput)
	}
	if rp.PositionOffset < 0 {
		return nil, domain.BadRequest("position_offset must be >= 0", domain.ErrInvalidInput)
	}
	if _, err := s.works.GetByID(ctx, rp.WorkID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NotFound("work not found", err)
		}
		return nil, err
	}
	if _, err := s.chapters.GetByID(ctx, rp.ChapterID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NotFound("chapter not found", err)
		}
		return nil, err
	}
	if err := s.progress.Upsert(ctx, rp); err != nil {
		return nil, err
	}
	return rp, nil
}
