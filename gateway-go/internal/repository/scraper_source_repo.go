package repository

import (
	"context"
	"errors"

	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresScraperSourceRepository implements domain.ScraperSourceRepository.
type PostgresScraperSourceRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresScraperSourceRepository(pool *pgxpool.Pool) *PostgresScraperSourceRepository {
	if pool == nil {
		panic("repository: NewPostgresScraperSourceRepository: pool must not be nil")
	}
	return &PostgresScraperSourceRepository{pool: pool}
}

func (r *PostgresScraperSourceRepository) Upsert(ctx context.Context, src *domain.ScraperSource) error {
	if src == nil {
		return domain.BadRequest("scraper source is nil", domain.ErrInvalidInput)
	}
	if src.SourceID == "" {
		return domain.BadRequest("source_id is empty", domain.ErrInvalidInput)
	}
	const q = `
INSERT INTO scraper_sources (source_id, name, media_type, base_url, rule_config, is_enabled, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, NOW())
ON CONFLICT (source_id) DO UPDATE SET
  name = EXCLUDED.name,
  media_type = EXCLUDED.media_type,
  base_url = EXCLUDED.base_url,
  rule_config = EXCLUDED.rule_config,
  is_enabled = EXCLUDED.is_enabled,
  updated_at = NOW()
RETURNING source_id, name, media_type, base_url, rule_config, is_enabled, updated_at`
	err := r.pool.QueryRow(ctx, q,
		src.SourceID, src.Name, src.MediaType, src.BaseURL, src.RuleConfig, src.IsEnabled,
	).Scan(
		&src.SourceID, &src.Name, &src.MediaType, &src.BaseURL,
		&src.RuleConfig, &src.IsEnabled, &src.UpdatedAt,
	)
	if err != nil {
		return domain.Internal("upsert scraper_source", err)
	}
	return nil
}

func (r *PostgresScraperSourceRepository) GetByID(ctx context.Context, sourceID string) (*domain.ScraperSource, error) {
	const q = `
SELECT source_id, name, media_type, base_url, rule_config, is_enabled, updated_at
FROM scraper_sources WHERE source_id = $1`
	var s domain.ScraperSource
	err := r.pool.QueryRow(ctx, q, sourceID).Scan(
		&s.SourceID, &s.Name, &s.MediaType, &s.BaseURL,
		&s.RuleConfig, &s.IsEnabled, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, domain.Internal("get scraper_source", err)
	}
	return &s, nil
}

func (r *PostgresScraperSourceRepository) ListEnabled(ctx context.Context, mediaType domain.MediaType) ([]domain.ScraperSource, error) {
	const q = `
SELECT source_id, name, media_type, base_url, rule_config, is_enabled, updated_at
FROM scraper_sources
WHERE media_type = $1 AND is_enabled = TRUE
ORDER BY source_id`
	rows, err := r.pool.Query(ctx, q, mediaType)
	if err != nil {
		return nil, domain.Internal("list scraper_sources", err)
	}
	defer rows.Close()
	out := make([]domain.ScraperSource, 0)
	for rows.Next() {
		var s domain.ScraperSource
		if err := rows.Scan(
			&s.SourceID, &s.Name, &s.MediaType, &s.BaseURL,
			&s.RuleConfig, &s.IsEnabled, &s.UpdatedAt,
		); err != nil {
			return nil, domain.Internal("scan scraper_source", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal("iterate scraper_sources", err)
	}
	return out, nil
}

var _ domain.ScraperSourceRepository = (*PostgresScraperSourceRepository)(nil)
