package repository

import (
	"context"
	"errors"
	"time"

	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresWorkSourceRepository implements domain.WorkSourceRepository.
type PostgresWorkSourceRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresWorkSourceRepository(pool *pgxpool.Pool) *PostgresWorkSourceRepository {
	if pool == nil {
		panic("repository: NewPostgresWorkSourceRepository: pool must not be nil")
	}
	return &PostgresWorkSourceRepository{pool: pool}
}

func (r *PostgresWorkSourceRepository) Create(ctx context.Context, ws *domain.WorkSource) error {
	if ws == nil {
		return domain.BadRequest("work_source is nil", domain.ErrInvalidInput)
	}
	const q = `
INSERT INTO work_sources (id, work_id, source_id, remote_work_url, latest_chapter_title, last_synced_at)
VALUES (
  COALESCE(NULLIF($1, '00000000-0000-0000-0000-000000000000'::uuid), gen_random_uuid()),
  $2, $3, $4, $5, $6
)
RETURNING id, work_id, source_id, remote_work_url, latest_chapter_title, last_synced_at`
	err := r.pool.QueryRow(ctx, q,
		ws.ID, ws.WorkID, ws.SourceID, ws.RemoteWorkURL, ws.LatestChapterTitle, ws.LastSyncedAt,
	).Scan(
		&ws.ID, &ws.WorkID, &ws.SourceID, &ws.RemoteWorkURL,
		&ws.LatestChapterTitle, &ws.LastSyncedAt,
	)
	if err != nil {
		return domain.Internal("insert work_source", err)
	}
	return nil
}

func (r *PostgresWorkSourceRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.WorkSource, error) {
	const q = `
SELECT id, work_id, source_id, remote_work_url, latest_chapter_title, last_synced_at
FROM work_sources WHERE id = $1`
	var ws domain.WorkSource
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&ws.ID, &ws.WorkID, &ws.SourceID, &ws.RemoteWorkURL,
		&ws.LatestChapterTitle, &ws.LastSyncedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, domain.Internal("get work_source", err)
	}
	return &ws, nil
}

func (r *PostgresWorkSourceRepository) ListByWorkID(ctx context.Context, workID uuid.UUID) ([]domain.WorkSource, error) {
	const q = `
SELECT id, work_id, source_id, remote_work_url, latest_chapter_title, last_synced_at
FROM work_sources WHERE work_id = $1 ORDER BY source_id`
	rows, err := r.pool.Query(ctx, q, workID)
	if err != nil {
		return nil, domain.Internal("list work_sources", err)
	}
	defer rows.Close()
	out := make([]domain.WorkSource, 0)
	for rows.Next() {
		var ws domain.WorkSource
		if err := rows.Scan(
			&ws.ID, &ws.WorkID, &ws.SourceID, &ws.RemoteWorkURL,
			&ws.LatestChapterTitle, &ws.LastSyncedAt,
		); err != nil {
			return nil, domain.Internal("scan work_source", err)
		}
		out = append(out, ws)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal("iterate work_sources", err)
	}
	return out, nil
}

func (r *PostgresWorkSourceRepository) UpdateSyncMeta(ctx context.Context, id uuid.UUID, latestTitle *string, syncedAt time.Time) error {
	const q = `
UPDATE work_sources
SET latest_chapter_title = $2, last_synced_at = $3
WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id, latestTitle, syncedAt)
	if err != nil {
		return domain.Internal("update work_source sync meta", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

var _ domain.WorkSourceRepository = (*PostgresWorkSourceRepository)(nil)
