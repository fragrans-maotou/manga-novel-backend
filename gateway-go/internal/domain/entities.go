package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Work is the platform canonical work row (table: works).
type Work struct {
	ID          uuid.UUID  `json:"id"`
	Title       string     `json:"title"`
	Author      string     `json:"author"`
	MediaType   MediaType  `json:"media_type"`
	CoverURL    *string    `json:"cover_url,omitempty"`
	Description *string    `json:"description,omitempty"`
	Status      WorkStatus `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// ScraperSource is an external scrape source with declarative JSON rules.
type ScraperSource struct {
	SourceID   string          `json:"source_id"`
	Name       string          `json:"name"`
	MediaType  MediaType       `json:"media_type"`
	BaseURL    string          `json:"base_url"`
	RuleConfig json.RawMessage `json:"rule_config"`
	IsEnabled  bool            `json:"is_enabled"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// WorkSource maps a work to one remote source listing.
type WorkSource struct {
	ID                 uuid.UUID  `json:"id"`
	WorkID             uuid.UUID  `json:"work_id"`
	SourceID           string     `json:"source_id"`
	RemoteWorkURL      string     `json:"remote_work_url"`
	LatestChapterTitle *string    `json:"latest_chapter_title,omitempty"`
	LastSyncedAt       *time.Time `json:"last_synced_at,omitempty"`
}

// Chapter is the chapter index row; empty StorageKey means realtime scrape.
type Chapter struct {
	ID               uuid.UUID `json:"id"`
	WorkSourceID     uuid.UUID `json:"work_source_id"`
	ChapterNum       float64   `json:"chapter_num"`
	Title            string    `json:"title"`
	RemoteChapterURL string    `json:"remote_chapter_url"`
	StorageKey       *string   `json:"storage_key,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// ReadingProgress tracks per-user reading position on a work.
type ReadingProgress struct {
	UserID         uuid.UUID `json:"user_id"`
	WorkID         uuid.UUID `json:"work_id"`
	ChapterID      uuid.UUID `json:"chapter_id"`
	PositionOffset int       `json:"position_offset"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ScrapeTask is the Redis queue payload (SPEC §5.1).
type ScrapeTask struct {
	TaskID    string `json:"task_id"`
	MediaType string `json:"media_type"`
	SourceID  string `json:"source_id"`
	ChapterID string `json:"chapter_id"`
	RemoteURL string `json:"remote_url"`
	TimeoutMS int    `json:"timeout_ms"`
}

// ProxyTicket is the manga image proxy credential hash (SPEC §5.4).
type ProxyTicket struct {
	URL     string `json:"url"`
	Referer string `json:"referer"`
	UA      string `json:"ua"`
	XORKey  string `json:"xor_key"`
}

// MangaChapterCache is the JSON value stored in cache:chapter:{id} for manga.
type MangaChapterCache struct {
	ChapterID string   `json:"chapter_id"`
	Images    []string `json:"images"`
}
