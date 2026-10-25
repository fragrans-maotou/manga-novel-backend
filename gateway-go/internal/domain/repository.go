package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// WorkRepository persists platform works.
type WorkRepository interface {
	Create(ctx context.Context, work *Work) error
	GetByID(ctx context.Context, id uuid.UUID) (*Work, error)
	GetByTitleAuthorMedia(ctx context.Context, title, author string, mediaType MediaType) (*Work, error)
	Update(ctx context.Context, work *Work) error
	List(ctx context.Context, mediaType *MediaType, limit, offset int) ([]Work, error)
}

// ScraperSourceRepository persists scrape source rules.
type ScraperSourceRepository interface {
	Upsert(ctx context.Context, src *ScraperSource) error
	GetByID(ctx context.Context, sourceID string) (*ScraperSource, error)
	ListEnabled(ctx context.Context, mediaType MediaType) ([]ScraperSource, error)
}

// WorkSourceRepository persists work↔source mappings.
type WorkSourceRepository interface {
	Create(ctx context.Context, ws *WorkSource) error
	GetByID(ctx context.Context, id uuid.UUID) (*WorkSource, error)
	ListByWorkID(ctx context.Context, workID uuid.UUID) ([]WorkSource, error)
	UpdateSyncMeta(ctx context.Context, id uuid.UUID, latestTitle *string, syncedAt time.Time) error
}

// ChapterRepository persists chapter indexes.
type ChapterRepository interface {
	Create(ctx context.Context, ch *Chapter) error
	GetByID(ctx context.Context, id uuid.UUID) (*Chapter, error)
	ListByWorkSource(ctx context.Context, workSourceID uuid.UUID) ([]Chapter, error)
	GetByWorkSourceAndNum(ctx context.Context, workSourceID uuid.UUID, chapterNum float64) (*Chapter, error)
	SetStorageKey(ctx context.Context, id uuid.UUID, storageKey string) error
}

// ReadingProgressRepository persists user reading progress.
type ReadingProgressRepository interface {
	Upsert(ctx context.Context, rp *ReadingProgress) error
	Get(ctx context.Context, userID, workID uuid.UUID) (*ReadingProgress, error)
}

// ScrapeQueuePriority selects which Redis list receives a scrape task.
type ScrapeQueuePriority string

const (
	ScrapeQueueRealtime ScrapeQueuePriority = "realtime"
	ScrapeQueueCron     ScrapeQueuePriority = "cron"
)

// ScrapeQueueRepository enqueues scrape tasks (Go LPUSH → Python BRPOP).
type ScrapeQueueRepository interface {
	Enqueue(ctx context.Context, priority ScrapeQueuePriority, task *ScrapeTask) error
}

// StreamRepository consumes Python stream callbacks via BLPOP on stream:{task_id}.
type StreamRepository interface {
	BlockingPop(ctx context.Context, taskID string, timeout time.Duration) (string, error)
}

// ChapterCacheRepository reads/writes cache:chapter:{chapter_id}.
type ChapterCacheRepository interface {
	Get(ctx context.Context, chapterID string) (string, error)
	Set(ctx context.Context, chapterID, value string, ttl time.Duration) error
	Delete(ctx context.Context, chapterID string) error
}

// ProxyTicketRepository loads manga proxy credentials from proxy:ticket:{ticket}.
type ProxyTicketRepository interface {
	Get(ctx context.Context, ticket string) (*ProxyTicket, error)
}

// SourceHealthRepository maintains health:sources:{media_type} ZSET scores.
type SourceHealthRepository interface {
	IncrBy(ctx context.Context, mediaType MediaType, sourceID string, delta float64) (float64, error)
	Score(ctx context.Context, mediaType MediaType, sourceID string) (float64, error)
	Top(ctx context.Context, mediaType MediaType, n int64) ([]string, error)
}
