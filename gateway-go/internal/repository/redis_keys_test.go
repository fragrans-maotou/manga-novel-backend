package repository

import (
	"testing"

	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/domain"
)

func TestRedisKeysMatchSPEC(t *testing.T) {
	if KeyQueueScrapeRealtime != "queue:scrape:realtime" {
		t.Fatalf("realtime key mismatch: %s", KeyQueueScrapeRealtime)
	}
	if KeyQueueScrapeCron != "queue:scrape:cron" {
		t.Fatalf("cron key mismatch: %s", KeyQueueScrapeCron)
	}
	if keyStream("abc") != "stream:abc" {
		t.Fatal("stream key mismatch")
	}
	if keyChapterCache("cid") != "cache:chapter:cid" {
		t.Fatal("cache key mismatch")
	}
	if keyProxyTicket("t") != "proxy:ticket:t" {
		t.Fatal("proxy key mismatch")
	}
	if keyHealthSources(domain.MediaTypeNovel) != "health:sources:novel" {
		t.Fatal("health novel key mismatch")
	}
	if keyHealthSources(domain.MediaTypeManga) != "health:sources:manga" {
		t.Fatal("health manga key mismatch")
	}
}
