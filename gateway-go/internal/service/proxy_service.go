package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/domain"
)

// ProxyService resolves manga proxy tickets and opens upstream image streams.
// Handlers must stream via a fixed 32KiB buffer (SPEC §6.3); this type never ReadAlls.
type ProxyService struct {
	tickets domain.ProxyTicketRepository
	client  *http.Client
}

// NewProxyService injects ticket repo and HTTP client.
func NewProxyService(tickets domain.ProxyTicketRepository, client *http.Client) *ProxyService {
	if tickets == nil {
		panic("service: NewProxyService: tickets must not be nil")
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &ProxyService{tickets: tickets, client: client}
}

// ImageStream is an opened upstream image response ready for pipe decryption.
type ImageStream struct {
	Ticket      *domain.ProxyTicket
	Body        io.ReadCloser
	ContentType string
	XORKey      byte
	XOREnabled  bool
}

// Close closes the upstream body.
func (s *ImageStream) Close() error {
	if s == nil || s.Body == nil {
		return nil
	}
	return s.Body.Close()
}

// OpenImage looks up proxy:ticket:{ticket} and GETs the remote URL with referer/UA.
func (s *ProxyService) OpenImage(ctx context.Context, ticket string) (*ImageStream, error) {
	if ticket == "" {
		return nil, domain.BadRequest("ticket is empty", domain.ErrInvalidInput)
	}
	pt, err := s.tickets.Get(ctx, ticket)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NotFound("proxy ticket not found", err)
		}
		return nil, err
	}
	xorKey, xorOn, err := ParseXORKey(pt.XORKey)
	if err != nil {
		return nil, domain.BadRequest("invalid xor_key on ticket", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pt.URL, nil)
	if err != nil {
		return nil, domain.Internal("build upstream image request", err)
	}
	if pt.Referer != "" {
		req.Header.Set("Referer", pt.Referer)
	}
	if pt.UA != "" {
		req.Header.Set("User-Agent", pt.UA)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, domain.Unavailable("upstream image fetch failed", err)
	}
	if resp.StatusCode >= 400 {
		_ = resp.Body.Close()
		return nil, domain.Unavailable(fmt.Sprintf("upstream image status %d", resp.StatusCode), nil)
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/octet-stream"
	}
	return &ImageStream{
		Ticket:      pt,
		Body:        resp.Body,
		ContentType: ct,
		XORKey:      xorKey,
		XOREnabled:  xorOn,
	}, nil
}
