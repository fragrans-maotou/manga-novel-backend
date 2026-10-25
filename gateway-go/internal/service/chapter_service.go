package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/domain"
	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"
)

const (
	streamEOF        = "[EOF]"
	streamReady      = "READY"
	streamErrorPref  = "[ERROR]"
	healthPenalty    = -20.0
	healthReward     = 1.0
	maxFailoverTries = 5
)

// ChapterService orchestrates cache lookup, singleflight scrape dispatch,
// stream consumption, and multi-source failover (SPEC §5 / §6.2).
type ChapterService struct {
	chapters    domain.ChapterRepository
	workSources domain.WorkSourceRepository
	works       domain.WorkRepository
	cache       domain.ChapterCacheRepository
	queue       domain.ScrapeQueueRepository
	stream      domain.StreamRepository
	health      domain.SourceHealthRepository
	logger      *slog.Logger

	streamTimeout   time.Duration
	scrapeTimeoutMS int
	cacheTTL        time.Duration

	sf singleflight.Group
}

// ChapterServiceDeps bundles injected collaborators (no package-level globals).
type ChapterServiceDeps struct {
	Chapters    domain.ChapterRepository
	WorkSources domain.WorkSourceRepository
	Works       domain.WorkRepository
	Cache       domain.ChapterCacheRepository
	Queue       domain.ScrapeQueueRepository
	Stream      domain.StreamRepository
	Health      domain.SourceHealthRepository
	Logger      *slog.Logger

	StreamTimeout   time.Duration
	ScrapeTimeoutMS int
	CacheTTL        time.Duration
}

// NewChapterService constructs a ChapterService via explicit DI.
func NewChapterService(d ChapterServiceDeps) *ChapterService {
	if d.Chapters == nil || d.WorkSources == nil || d.Works == nil ||
		d.Cache == nil || d.Queue == nil || d.Stream == nil || d.Health == nil {
		panic("service: NewChapterService: all repositories are required")
	}
	logger := d.Logger
	if logger == nil {
		logger = slog.Default()
	}
	timeout := d.StreamTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	scrapeMS := d.ScrapeTimeoutMS
	if scrapeMS <= 0 {
		scrapeMS = 8000
	}
	ttl := d.CacheTTL
	if ttl <= 0 {
		ttl = 86400 * time.Second
	}
	return &ChapterService{
		chapters:        d.Chapters,
		workSources:     d.WorkSources,
		works:           d.Works,
		cache:           d.Cache,
		queue:           d.Queue,
		stream:          d.Stream,
		health:          d.Health,
		logger:          logger,
		streamTimeout:   timeout,
		scrapeTimeoutMS: scrapeMS,
		cacheTTL:        ttl,
	}
}

// GetMangaChapter returns manga chapter JSON from cache, scraping on miss.
func (s *ChapterService) GetMangaChapter(ctx context.Context, chapterID uuid.UUID) (*domain.MangaChapterCache, error) {
	raw, err := s.ensureChapterCache(ctx, chapterID, domain.MediaTypeManga)
	if err != nil {
		return nil, err
	}
	var out domain.MangaChapterCache
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, domain.Internal("decode manga chapter cache", err)
	}
	if out.ChapterID == "" {
		out.ChapterID = chapterID.String()
	}
	if out.Images == nil {
		out.Images = []string{}
	}
	return &out, nil
}

// StreamNovel emits novel text chunks via emit (SSE-friendly).
// Cache hit → one emit of full body. Miss → scrape under singleflight;
// chunks are forwarded as they arrive for the flight owner result path.
func (s *ChapterService) StreamNovel(ctx context.Context, chapterID uuid.UUID, emit func(chunk string) error) error {
	if emit == nil {
		return domain.BadRequest("emit callback is nil", domain.ErrInvalidInput)
	}
	idStr := chapterID.String()
	if cached, err := s.cache.Get(ctx, idStr); err == nil {
		if cached == "" {
			return emit("")
		}
		return emit(cached)
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	v, err, _ := s.sf.Do("novel:"+idStr, func() (interface{}, error) {
		return s.scrapeNovel(ctx, chapterID)
	})
	if err != nil {
		return err
	}
	chunks, ok := v.([]string)
	if !ok {
		return domain.Internal("unexpected novel scrape result type", nil)
	}
	for _, chunk := range chunks {
		if err := emit(chunk); err != nil {
			return err
		}
	}
	return nil
}

// ensureChapterCache returns cache body, using singleflight scrape on miss.
func (s *ChapterService) ensureChapterCache(ctx context.Context, chapterID uuid.UUID, expect domain.MediaType) (string, error) {
	idStr := chapterID.String()
	if cached, err := s.cache.Get(ctx, idStr); err == nil {
		return cached, nil
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return "", err
	}

	v, err, _ := s.sf.Do("chapter:"+idStr, func() (interface{}, error) {
		// Double-check after winning the flight.
		if cached, err := s.cache.Get(ctx, idStr); err == nil {
			return cached, nil
		} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		if expect == domain.MediaTypeManga {
			if err := s.scrapeManga(ctx, chapterID); err != nil {
				return nil, err
			}
			cached, err := s.cache.Get(ctx, idStr)
			if err != nil {
				if errors.Is(err, domain.ErrNotFound) {
					return nil, domain.Unavailable("manga scrape finished without cache", domain.ErrUnavailable)
				}
				return nil, err
			}
			return cached, nil
		}
		chunks, err := s.scrapeNovel(ctx, chapterID)
		if err != nil {
			return nil, err
		}
		body := strings.Join(chunks, "")
		return body, nil
	})
	if err != nil {
		return "", err
	}
	raw, ok := v.(string)
	if !ok {
		return "", domain.Internal("unexpected cache scrape result type", nil)
	}
	return raw, nil
}

func (s *ChapterService) scrapeManga(ctx context.Context, chapterID uuid.UUID) error {
	return s.scrapeWithFailover(ctx, chapterID, domain.MediaTypeManga, func(ctx context.Context, taskID string) error {
		for {
			msg, err := s.stream.BlockingPop(ctx, taskID, s.streamTimeout)
			if err != nil {
				if errors.Is(err, domain.ErrTimeout) {
					return domain.ErrTimeout
				}
				return err
			}
			if msg == streamReady {
				return nil
			}
			if isStreamError(msg) {
				return fmt.Errorf("%w: %s", domain.ErrUnavailable, msg)
			}
			// Ignore unexpected intermediate messages for manga.
			s.logger.Warn("manga stream unexpected message", "task_id", taskID, "msg", msg)
		}
	})
}

func (s *ChapterService) scrapeNovel(ctx context.Context, chapterID uuid.UUID) ([]string, error) {
	var collected []string
	err := s.scrapeWithFailover(ctx, chapterID, domain.MediaTypeNovel, func(ctx context.Context, taskID string) error {
		collected = collected[:0]
		var builder strings.Builder
		for {
			msg, err := s.stream.BlockingPop(ctx, taskID, s.streamTimeout)
			if err != nil {
				if errors.Is(err, domain.ErrTimeout) {
					return domain.ErrTimeout
				}
				return err
			}
			if msg == streamEOF {
				body := builder.String()
				if body != "" {
					if err := s.cache.Set(ctx, chapterID.String(), body, s.cacheTTL); err != nil {
						return err
					}
				}
				return nil
			}
			if isStreamError(msg) {
				return fmt.Errorf("%w: %s", domain.ErrUnavailable, msg)
			}
			collected = append(collected, msg)
			builder.WriteString(msg)
		}
	})
	if err != nil {
		return nil, err
	}
	// Prefer cache (authoritative full text) when present.
	if cached, err := s.cache.Get(ctx, chapterID.String()); err == nil && cached != "" {
		return []string{cached}, nil
	}
	return collected, nil
}

type scrapeAttemptFunc func(ctx context.Context, taskID string) error

func (s *ChapterService) scrapeWithFailover(ctx context.Context, chapterID uuid.UUID, mediaType domain.MediaType, consume scrapeAttemptFunc) error {
	candidates, err := s.candidateChapters(ctx, chapterID, mediaType)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return domain.NotFound("no scrape candidates for chapter", domain.ErrNotFound)
	}

	var lastErr error
	tried := 0
	for _, cand := range candidates {
		if tried >= maxFailoverTries {
			break
		}
		tried++
		if skip, _ := s.shouldSkipSource(ctx, mediaType, cand.SourceID); skip {
			s.logger.Info("skip circuit-open source", "source_id", cand.SourceID, "chapter_id", cand.Chapter.ID)
			continue
		}
		taskID := uuid.New().String()
		task := &domain.ScrapeTask{
			TaskID:    taskID,
			MediaType: string(mediaType),
			SourceID:  cand.SourceID,
			ChapterID: cand.Chapter.ID.String(),
			RemoteURL: cand.Chapter.RemoteChapterURL,
			TimeoutMS: s.scrapeTimeoutMS,
		}
		if err := s.queue.Enqueue(ctx, domain.ScrapeQueueRealtime, task); err != nil {
			lastErr = err
			continue
		}
		err := consume(ctx, taskID)
		if err == nil {
			if _, herr := s.health.IncrBy(ctx, mediaType, cand.SourceID, healthReward); herr != nil {
				s.logger.Warn("health reward failed", "source_id", cand.SourceID, "err", herr)
			}
			return nil
		}
		lastErr = err
		if errors.Is(err, domain.ErrTimeout) || isUnavailable(err) {
			if _, herr := s.health.IncrBy(ctx, mediaType, cand.SourceID, healthPenalty); herr != nil {
				s.logger.Warn("health penalty failed", "source_id", cand.SourceID, "err", herr)
			}
			s.logger.Warn("scrape attempt failed; trying next source",
				"chapter_id", cand.Chapter.ID, "source_id", cand.SourceID, "err", err)
			continue
		}
		return err
	}
	if lastErr == nil {
		lastErr = domain.ErrUnavailable
	}
	return domain.Unavailable("all scrape sources failed", lastErr)
}

type scrapeCandidate struct {
	Chapter  domain.Chapter
	SourceID string
	Score    float64
}

func (s *ChapterService) candidateChapters(ctx context.Context, chapterID uuid.UUID, mediaType domain.MediaType) ([]scrapeCandidate, error) {
	primary, err := s.chapters.GetByID(ctx, chapterID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NotFound("chapter not found", err)
		}
		return nil, err
	}
	ws, err := s.workSources.GetByID(ctx, primary.WorkSourceID)
	if err != nil {
		return nil, err
	}
	work, err := s.works.GetByID(ctx, ws.WorkID)
	if err != nil {
		return nil, err
	}
	if work.MediaType != mediaType {
		return nil, domain.BadRequest("chapter media_type mismatch", domain.ErrInvalidInput)
	}

	allWS, err := s.workSources.ListByWorkID(ctx, ws.WorkID)
	if err != nil {
		return nil, err
	}

	out := make([]scrapeCandidate, 0, len(allWS))
	for _, mapping := range allWS {
		ch := primary
		if mapping.ID != primary.WorkSourceID {
			alt, err := s.chapters.GetByWorkSourceAndNum(ctx, mapping.ID, primary.ChapterNum)
			if err != nil {
				if errors.Is(err, domain.ErrNotFound) {
					continue
				}
				return nil, err
			}
			ch = alt
		}
		score, err := s.health.Score(ctx, mediaType, mapping.SourceID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				score = 100 // SPEC initial health
			} else {
				return nil, err
			}
		}
		out = append(out, scrapeCandidate{Chapter: *ch, SourceID: mapping.SourceID, Score: score})
	}

	// Highest health first; stable-ish by insertion for ties.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Score > out[i].Score {
				out[i], out[j] = out[j], out[i]
			}
		}
	}

	// Ensure primary chapter_id is preferred when scores equal: already sorted by score.
	// Put exact requested chapter first among equal top if present.
	for i, c := range out {
		if c.Chapter.ID == chapterID {
			if i > 0 && out[0].Score == c.Score {
				out[0], out[i] = out[i], out[0]
			}
			break
		}
	}
	return out, nil
}

func (s *ChapterService) shouldSkipSource(ctx context.Context, mediaType domain.MediaType, sourceID string) (bool, error) {
	score, err := s.health.Score(ctx, mediaType, sourceID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return repository.BelowCircuitFloor(score), nil
}

func isStreamError(msg string) bool {
	return strings.HasPrefix(msg, streamErrorPref)
}

func isUnavailable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, domain.ErrUnavailable) {
		return true
	}
	var ae *domain.AppError
	if errors.As(err, &ae) && ae.Code == 503 {
		return true
	}
	return strings.Contains(err.Error(), streamErrorPref)
}
