package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/config"
	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/handler"
	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/middleware"
	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/repository"
	"github.com/fragrans-maotou/manga-novel-backend/gateway-go/internal/service"
	"github.com/gofiber/fiber/v2"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()

	pool, err := repository.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("postgres", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb, err := repository.NewRedisClient(ctx, repository.RedisOptions{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if err != nil {
		logger.Error("redis", "err", err)
		os.Exit(1)
	}
	defer func() { _ = rdb.Close() }()

	workRepo := repository.NewPostgresWorkRepository(pool)
	workSourceRepo := repository.NewPostgresWorkSourceRepository(pool)
	chapterRepo := repository.NewPostgresChapterRepository(pool)
	progressRepo := repository.NewPostgresReadingProgressRepository(pool)
	cacheRepo := repository.NewRedisChapterCache(rdb, cfg.ChapterCacheTTL)
	queueRepo := repository.NewRedisScrapeQueue(rdb)
	streamRepo := repository.NewRedisStream(rdb)
	healthRepo := repository.NewRedisSourceHealth(rdb)
	ticketRepo := repository.NewRedisProxyTicket(rdb)

	workSvc := service.NewWorkService(workRepo, workSourceRepo, chapterRepo)
	progressSvc := service.NewProgressService(progressRepo, workRepo, chapterRepo)
	chapterSvc := service.NewChapterService(service.ChapterServiceDeps{
		Chapters:        chapterRepo,
		WorkSources:     workSourceRepo,
		Works:           workRepo,
		Cache:           cacheRepo,
		Queue:           queueRepo,
		Stream:          streamRepo,
		Health:          healthRepo,
		Logger:          logger,
		StreamTimeout:   cfg.StreamBLPopTimeout,
		ScrapeTimeoutMS: cfg.ScrapeTimeoutMS,
		CacheTTL:        cfg.ChapterCacheTTL,
	})
	proxySvc := service.NewProxyService(ticketRepo, &http.Client{Timeout: cfg.ProxyHTTPTimeout})

	healthH := handler.NewHealthHandler(pool, rdb)
	workH := handler.NewWorkHandler(workSvc, progressSvc)
	chapterH := handler.NewChapterHandler(chapterSvc)
	proxyH := handler.NewProxyHandler(proxySvc)

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ErrorHandler:          middleware.NewErrorHandler(logger),
		ReadTimeout:           30 * time.Second,
		WriteTimeout:          0,
		IdleTimeout:           60 * time.Second,
	})
	app.Use(middleware.RequestID())
	app.Use(middleware.AccessLog(logger))

	app.Get("/healthz", healthH.Healthz)

	api := app.Group("/api/v1")
	api.Get("/works", workH.ListWorks)
	api.Get("/works/:workID", workH.GetWork)
	api.Get("/works/:workID/chapters", workH.ListChapters)
	api.Get("/works/:workID/progress", workH.GetProgress)
	api.Put("/works/:workID/progress", workH.PutProgress)
	api.Get("/chapters/:chapterID/manga", chapterH.GetMangaChapter)
	api.Get("/chapters/:chapterID/novel", chapterH.StreamNovel)
	api.Get("/proxy/image/:ticket", proxyH.ProxyImage)

	go func() {
		logger.Info("gateway listening", "addr", cfg.HTTPAddr)
		if err := app.Listen(cfg.HTTPAddr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("listen failed", "err", err)
			os.Exit(1)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	logger.Info("shutdown signal received", "signal", sig.String())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	logger.Info("gateway stopped")
}
