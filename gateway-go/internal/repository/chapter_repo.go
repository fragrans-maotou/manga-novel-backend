package repository

import (
	"context"
	"errors"

	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresChapterRepository implements domain.ChapterRepository.
type PostgresChapterRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresChapterRepository(pool *pgxpool.Pool) *PostgresChapterRepository {
	if pool == nil {
		panic("repository: NewPostgresChapterRepository: pool must not be nil")
	}
	return &PostgresChapterRepository{pool: pool}
}

func (r *PostgresChapterRepository) Create(ctx context.Context, ch *domain.Chapter) error {
	if ch == nil {
		return domain.BadRequest("chapter is nil", domain.ErrInvalidInput)
	}
	const q = `
INSERT INTO chapters (id, work_source_id, chapter_num, title, remote_chapter_url, storage_key, created_at)
VALUES (
  COALESCE(NULLIF($1, '00000000-0000-0000-0000-000000000000'::uuid), gen_random_uuid()),
  $2, $3, $4, $5, $6, NOW()
)
RETURNING id, work_source_id, chapter_num, title, remote_chapter_url, storage_key, created_at`
	err := r.pool.QueryRow(ctx, q,
		ch.ID, ch.WorkSourceID, ch.ChapterNum, ch.Title, ch.RemoteChapterURL, ch.StorageKey,
	).Scan(
		&ch.ID, &ch.WorkSourceID, &ch.ChapterNum, &ch.Title,
		&ch.RemoteChapterURL, &ch.StorageKey, &ch.CreatedAt,
	)
	if err != nil {
		return domain.Internal("insert chapter", err)
	}
	return nil
}

func (r *PostgresChapterRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Chapter, error) {
	const q = `
SELECT id, work_source_id, chapter_num, title, remote_chapter_url, storage_key, created_at
FROM chapters WHERE id = $1`
	var ch domain.Chapter
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&ch.ID, &ch.WorkSourceID, &ch.ChapterNum, &ch.Title,
		&ch.RemoteChapterURL, &ch.StorageKey, &ch.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, domain.Internal("get chapter", err)
	}
	return &ch, nil
}

func (r *PostgresChapterRepository) ListByWorkSource(ctx context.Context, workSourceID uuid.UUID) ([]domain.Chapter, error) {
	const q = `
SELECT id, work_source_id, chapter_num, title, remote_chapter_url, storage_key, created_at
FROM chapters WHERE work_source_id = $1
ORDER BY chapter_num ASC`
	rows, err := r.pool.Query(ctx, q, workSourceID)
	if err != nil {
		return nil, domain.Internal("list chapters", err)
	}
	defer rows.Close()
	out := make([]domain.Chapter, 0)
	for rows.Next() {
		var ch domain.Chapter
		if err := rows.Scan(
			&ch.ID, &ch.WorkSourceID, &ch.ChapterNum, &ch.Title,
			&ch.RemoteChapterURL, &ch.StorageKey, &ch.CreatedAt,
		); err != nil {
			return nil, domain.Internal("scan chapter", err)
		}
		out = append(out, ch)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal("iterate chapters", err)
	}
	return out, nil
}

func (r *PostgresChapterRepository) GetByWorkSourceAndNum(ctx context.Context, workSourceID uuid.UUID, chapterNum float64) (*domain.Chapter, error) {
	const q = `
SELECT id, work_source_id, chapter_num, title, remote_chapter_url, storage_key, created_at
FROM chapters WHERE work_source_id = $1 AND chapter_num = $2`
	var ch domain.Chapter
	err := r.pool.QueryRow(ctx, q, workSourceID, chapterNum).Scan(
		&ch.ID, &ch.WorkSourceID, &ch.ChapterNum, &ch.Title,
		&ch.RemoteChapterURL, &ch.StorageKey, &ch.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, domain.Internal("get chapter by num", err)
	}
	return &ch, nil
}

func (r *PostgresChapterRepository) SetStorageKey(ctx context.Context, id uuid.UUID, storageKey string) error {
	const q = `UPDATE chapters SET storage_key = $2 WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id, storageKey)
	if err != nil {
		return domain.Internal("set chapter storage_key", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

var _ domain.ChapterRepository = (*PostgresChapterRepository)(nil)
