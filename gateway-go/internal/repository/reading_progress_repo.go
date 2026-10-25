package repository

import (
	"context"
	"errors"

	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresReadingProgressRepository implements domain.ReadingProgressRepository.
type PostgresReadingProgressRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresReadingProgressRepository(pool *pgxpool.Pool) *PostgresReadingProgressRepository {
	if pool == nil {
		panic("repository: NewPostgresReadingProgressRepository: pool must not be nil")
	}
	return &PostgresReadingProgressRepository{pool: pool}
}

func (r *PostgresReadingProgressRepository) Upsert(ctx context.Context, rp *domain.ReadingProgress) error {
	if rp == nil {
		return domain.BadRequest("reading_progress is nil", domain.ErrInvalidInput)
	}
	const q = `
INSERT INTO reading_progress (user_id, work_id, chapter_id, position_offset, updated_at)
VALUES ($1, $2, $3, $4, NOW())
ON CONFLICT (user_id, work_id) DO UPDATE SET
  chapter_id = EXCLUDED.chapter_id,
  position_offset = EXCLUDED.position_offset,
  updated_at = NOW()
RETURNING user_id, work_id, chapter_id, position_offset, updated_at`
	err := r.pool.QueryRow(ctx, q,
		rp.UserID, rp.WorkID, rp.ChapterID, rp.PositionOffset,
	).Scan(
		&rp.UserID, &rp.WorkID, &rp.ChapterID, &rp.PositionOffset, &rp.UpdatedAt,
	)
	if err != nil {
		return domain.Internal("upsert reading_progress", err)
	}
	return nil
}

func (r *PostgresReadingProgressRepository) Get(ctx context.Context, userID, workID uuid.UUID) (*domain.ReadingProgress, error) {
	const q = `
SELECT user_id, work_id, chapter_id, position_offset, updated_at
FROM reading_progress WHERE user_id = $1 AND work_id = $2`
	var rp domain.ReadingProgress
	err := r.pool.QueryRow(ctx, q, userID, workID).Scan(
		&rp.UserID, &rp.WorkID, &rp.ChapterID, &rp.PositionOffset, &rp.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, domain.Internal("get reading_progress", err)
	}
	return &rp, nil
}

var _ domain.ReadingProgressRepository = (*PostgresReadingProgressRepository)(nil)
