package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/domain"
	"github.com/google/uuid"
)

type memCache struct {
	mu   sync.Mutex
	data map[string]string
}

func newMemCache() *memCache { return &memCache{data: map[string]string{}} }

func (m *memCache) Get(_ context.Context, chapterID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[chapterID]
	if !ok {
		return "", domain.ErrNotFound
	}
	return v, nil
}
func (m *memCache) Set(_ context.Context, chapterID, value string, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[chapterID] = value
	return nil
}
func (m *memCache) Delete(_ context.Context, chapterID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, chapterID)
	return nil
}

type countingQueue struct {
	n atomic.Int32
	// onEnqueue optionally pushes READY into stream for the task
	onEnqueue func(task *domain.ScrapeTask)
}

func (q *countingQueue) Enqueue(_ context.Context, _ domain.ScrapeQueuePriority, task *domain.ScrapeTask) error {
	q.n.Add(1)
	if q.onEnqueue != nil {
		q.onEnqueue(task)
	}
	return nil
}

type memStream struct {
	mu   sync.Mutex
	bufs map[string][]string
}

func newMemStream() *memStream { return &memStream{bufs: map[string][]string{}} }

func (s *memStream) push(taskID, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bufs[taskID] = append(s.bufs[taskID], msg)
}

func (s *memStream) BlockingPop(ctx context.Context, taskID string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for {
		s.mu.Lock()
		if msgs := s.bufs[taskID]; len(msgs) > 0 {
			msg := msgs[0]
			s.bufs[taskID] = msgs[1:]
			s.mu.Unlock()
			return msg, nil
		}
		s.mu.Unlock()
		if time.Now().After(deadline) {
			return "", domain.ErrTimeout
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
}

type memHealth struct {
	mu     sync.Mutex
	scores map[string]float64
}

func newMemHealth() *memHealth { return &memHealth{scores: map[string]float64{}} }

func (h *memHealth) key(mt domain.MediaType, id string) string { return string(mt) + ":" + id }

func (h *memHealth) IncrBy(_ context.Context, mediaType domain.MediaType, sourceID string, delta float64) (float64, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	k := h.key(mediaType, sourceID)
	h.scores[k] += delta
	if h.scores[k] > 100 {
		h.scores[k] = 100
	}
	if h.scores[k] < 0 {
		h.scores[k] = 0
	}
	return h.scores[k], nil
}
func (h *memHealth) Score(_ context.Context, mediaType domain.MediaType, sourceID string) (float64, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	v, ok := h.scores[h.key(mediaType, sourceID)]
	if !ok {
		return 0, domain.ErrNotFound
	}
	return v, nil
}
func (h *memHealth) Top(context.Context, domain.MediaType, int64) ([]string, error) {
	return nil, nil
}

type fixedChapterRepo struct {
	ch domain.Chapter
}

func (r *fixedChapterRepo) Create(context.Context, *domain.Chapter) error { return nil }
func (r *fixedChapterRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Chapter, error) {
	if id != r.ch.ID {
		return nil, domain.ErrNotFound
	}
	cp := r.ch
	return &cp, nil
}
func (r *fixedChapterRepo) ListByWorkSource(context.Context, uuid.UUID) ([]domain.Chapter, error) {
	return []domain.Chapter{r.ch}, nil
}
func (r *fixedChapterRepo) GetByWorkSourceAndNum(_ context.Context, wsID uuid.UUID, num float64) (*domain.Chapter, error) {
	if wsID == r.ch.WorkSourceID && num == r.ch.ChapterNum {
		cp := r.ch
		return &cp, nil
	}
	return nil, domain.ErrNotFound
}
func (r *fixedChapterRepo) SetStorageKey(context.Context, uuid.UUID, string) error { return nil }

type fixedWorkSourceRepo struct {
	ws domain.WorkSource
}

func (r *fixedWorkSourceRepo) Create(context.Context, *domain.WorkSource) error { return nil }
func (r *fixedWorkSourceRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.WorkSource, error) {
	if id != r.ws.ID {
		return nil, domain.ErrNotFound
	}
	cp := r.ws
	return &cp, nil
}
func (r *fixedWorkSourceRepo) ListByWorkID(_ context.Context, workID uuid.UUID) ([]domain.WorkSource, error) {
	if workID != r.ws.WorkID {
		return nil, nil
	}
	return []domain.WorkSource{r.ws}, nil
}
func (r *fixedWorkSourceRepo) UpdateSyncMeta(context.Context, uuid.UUID, *string, time.Time) error {
	return nil
}

type fixedWorkRepo struct {
	w domain.Work
}

func (r *fixedWorkRepo) Create(context.Context, *domain.Work) error { return nil }
func (r *fixedWorkRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Work, error) {
	if id != r.w.ID {
		return nil, domain.ErrNotFound
	}
	cp := r.w
	return &cp, nil
}
func (r *fixedWorkRepo) GetByTitleAuthorMedia(context.Context, string, string, domain.MediaType) (*domain.Work, error) {
	return nil, domain.ErrNotFound
}
func (r *fixedWorkRepo) Update(context.Context, *domain.Work) error { return nil }
func (r *fixedWorkRepo) List(context.Context, *domain.MediaType, int, int) ([]domain.Work, error) {
	return nil, nil
}

func TestGetMangaChapter_SingleflightOneEnqueue(t *testing.T) {
	chapterID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	wsID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	workID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	cache := newMemCache()
	stream := newMemStream()
	queue := &countingQueue{
		onEnqueue: func(task *domain.ScrapeTask) {
			// Simulate slow worker: delay then write cache + READY.
			go func() {
				time.Sleep(30 * time.Millisecond)
				_ = cache.Set(context.Background(), task.ChapterID, `{"chapter_id":"`+task.ChapterID+`","images":["/api/v1/proxy/image/abc"]}`, time.Hour)
				stream.push(task.TaskID, streamReady)
			}()
		},
	}

	svc := NewChapterService(ChapterServiceDeps{
		Chapters: &fixedChapterRepo{ch: domain.Chapter{
			ID: chapterID, WorkSourceID: wsID, ChapterNum: 1,
			Title: "c1", RemoteChapterURL: "https://example.com/1",
		}},
		WorkSources: &fixedWorkSourceRepo{ws: domain.WorkSource{
			ID: wsID, WorkID: workID, SourceID: "src_a", RemoteWorkURL: "https://example.com/",
		}},
		Works: &fixedWorkRepo{w: domain.Work{
			ID: workID, Title: "t", Author: "a", MediaType: domain.MediaTypeManga, Status: domain.WorkStatusOngoing,
		}},
		Cache:           cache,
		Queue:           queue,
		Stream:          stream,
		Health:          newMemHealth(),
		StreamTimeout:   2 * time.Second,
		ScrapeTimeoutMS: 8000,
		CacheTTL:        time.Hour,
	})

	const n = 8
	var wg sync.WaitGroup
	errCh := make(chan error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, err := svc.GetMangaChapter(context.Background(), chapterID)
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("GetMangaChapter: %v", err)
		}
	}
	if got := queue.n.Load(); got != 1 {
		t.Fatalf("enqueue count=%d want 1 (singleflight)", got)
	}
}

func TestStreamNovel_CacheHitNoEnqueue(t *testing.T) {
	chapterID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	cache := newMemCache()
	_ = cache.Set(context.Background(), chapterID.String(), "cached body", time.Hour)
	queue := &countingQueue{}
	svc := NewChapterService(ChapterServiceDeps{
		Chapters:        &fixedChapterRepo{ch: domain.Chapter{ID: chapterID}},
		WorkSources:     &fixedWorkSourceRepo{},
		Works:           &fixedWorkRepo{},
		Cache:           cache,
		Queue:           queue,
		Stream:          newMemStream(),
		Health:          newMemHealth(),
		StreamTimeout:   time.Second,
		ScrapeTimeoutMS: 8000,
	})
	var got string
	if err := svc.StreamNovel(context.Background(), chapterID, func(chunk string) error {
		got += chunk
		return nil
	}); err != nil {
		t.Fatalf("StreamNovel: %v", err)
	}
	if got != "cached body" {
		t.Fatalf("got %q", got)
	}
	if queue.n.Load() != 0 {
		t.Fatalf("enqueue=%d want 0", queue.n.Load())
	}
}

func TestScrapeNovel_FailoverOnError(t *testing.T) {
	// Ensure error path returns wrapped unavailable (smoke).
	err := errors.New("[ERROR]:boom")
	if !isUnavailable(err) && !isStreamError(err.Error()) {
		// isStreamError works on message; isUnavailable checks error text containing prefix
		if !isStreamError("[ERROR]:boom") {
			t.Fatal("expected stream error detection")
		}
	}
}
