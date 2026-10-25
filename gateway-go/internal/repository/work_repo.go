package repository

import (
	"context"
	"errors"

	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresWorkRepository implements domain.WorkRepository with pgx.
type PostgresWorkRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresWorkRepository injects a pgx pool.
func NewPostgresWorkRepository(pool *pgxpool.Pool) *PostgresWorkRepository {
	if pool == nil {
		panic("repository: NewPostgresWorkRepository: pool must not be nil")
	}
	return &PostgresWorkRepository{pool: pool}
}

func (r *PostgresWorkRepository) Create(ctx context.Context, work *domain.Work) error {
	if work == nil {
		return domain.BadRequest("work is nil", domain.ErrInvalidInput)
	}
	const q = `
INSERT INTO works (id, title, author, media_type, cover_url, description, status, created_at, updated_at)
VALUES (
  COALESCE(NULLIF($1, '00000000-0000-0000-0000-000000000000'::uuid), gen_random_uuid()),
  $2, $3, $4, $5, $6, $7, NOW(), NOW()
)
RETURNING id, title, author, media_type, cover_url, description, status, created_at, updated_at`
	err := r.pool.QueryRow(ctx, q,
		work.ID, work.Title, work.Author, work.MediaType,
		work.CoverURL, work.Description, work.Status,
	).Scan(
		&work.ID, &work.Title, &work.Author, &work.MediaType,
		&work.CoverURL, &work.Description, &work.Status,
		&work.CreatedAt, &work.UpdatedAt,
	)
	if err != nil {
		return domain.Internal("insert work", err)
	}
	return nil
}

func (r *PostgresWorkRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Work, error) {
	const q = `
SELECT id, title, author, media_type, cover_url, description, status, created_at, updated_at
FROM works WHERE id = $1`
	var w domain.Work
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&w.ID, &w.Title, &w.Author, &w.MediaType,
		&w.CoverURL, &w.Description, &w.Status,
		&w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, domain.Internal("get work by id", err)
	}
	return &w, nil
}

func (r *PostgresWorkRepository) GetByTitleAuthorMedia(ctx context.Context, title, author string, mediaType domain.MediaType) (*domain.Work, error) {
	const q = `
SELECT id, title, author, media_type, cover_url, description, status, created_at, updated_at
FROM works WHERE title = $1 AND author = $2 AND media_type = $3`
	var w domain.Work
	err := r.pool.QueryRow(ctx, q, title, author, mediaType).Scan(
		&w.ID, &w.Title, &w.Author, &w.MediaType,
		&w.CoverURL, &w.Description, &w.Status,
		&w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, domain.Internal("get work by title/author/media", err)
	}
	return &w, nil
}

func (r *PostgresWorkRepository) Update(ctx context.Context, work *domain.Work) error {
	if work == nil {
		return domain.BadRequest("work is nil", domain.ErrInvalidInput)
	}
	const q = `
UPDATE works
SET title = $2, author = $3, media_type = $4, cover_url = $5,
    description = $6, status = $7, updated_at = NOW()
WHERE id = $1
RETURNING updated_at`
	err := r.pool.QueryRow(ctx, q,
		work.ID, work.Title, work.Author, work.MediaType,
		work.CoverURL, work.Description, work.Status,
	).Scan(&work.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return domain.Internal("update work", err)
	}
	return nil
}

func (r *PostgresWorkRepository) List(ctx context.Context, mediaType *domain.MediaType, limit, offset int) ([]domain.Work, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	var (
		rows pgx.Rows
		err  error
	)
	if mediaType != nil {
		const q = `
SELECT id, title, author, media_type, cover_url, description, status, created_at, updated_at
FROM works WHERE media_type = $1
ORDER BY updated_at DESC
LIMIT $2 OFFSET $3`
		rows, err = r.pool.Query(ctx, q, *mediaType, limit, offset)
	} else {
		const q = `
SELECT id, title, author, media_type, cover_url, description, status, created_at, updated_at
FROM works
ORDER BY updated_at DESC
LIMIT $1 OFFSET $2`
		rows, err = r.pool.Query(ctx, q, limit, offset)
	}
	if err != nil {
		return nil, domain.Internal("list works", err)
	}
	defer rows.Close()

	out := make([]domain.Work, 0)
	for rows.Next() {
		var w domain.Work
		if err := rows.Scan(
			&w.ID, &w.Title, &w.Author, &w.MediaType,
			&w.CoverURL, &w.Description, &w.Status,
			&w.CreatedAt, &w.UpdatedAt,
		); err != nil {
			return nil, domain.Internal("scan work", err)
		}
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal("iterate works", err)
	}
	return out, nil
}

var _ domain.WorkRepository = (*PostgresWorkRepository)(nil)
