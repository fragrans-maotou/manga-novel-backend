package repository

import (
	"fmt"

	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/domain"
)

// Redis key names — must match SPEC §5 verbatim.
const (
	KeyQueueScrapeRealtime = "queue:scrape:realtime"
	KeyQueueScrapeCron     = "queue:scrape:cron"

	chapterCacheTTLSeconds = 86400
	proxyTicketTTLSeconds  = 7200
)

func keyStream(taskID string) string {
	return fmt.Sprintf("stream:%s", taskID)
}

func keyChapterCache(chapterID string) string {
	return fmt.Sprintf("cache:chapter:%s", chapterID)
}

func keyProxyTicket(ticket string) string {
	return fmt.Sprintf("proxy:ticket:%s", ticket)
}

func keyHealthSources(mediaType domain.MediaType) string {
	return fmt.Sprintf("health:sources:%s", mediaType)
}

func queueKey(priority domain.ScrapeQueuePriority) (string, error) {
	switch priority {
	case domain.ScrapeQueueRealtime:
		return KeyQueueScrapeRealtime, nil
	case domain.ScrapeQueueCron:
		return KeyQueueScrapeCron, nil
	default:
		return "", fmt.Errorf("unknown scrape queue priority: %q", priority)
	}
}
