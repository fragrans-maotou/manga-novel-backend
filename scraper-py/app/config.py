"""Runtime configuration for scraper-py (Redis + worker knobs)."""

from __future__ import annotations

from functools import lru_cache

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Environment-driven settings. Instantiated per process; no hidden globals in engine."""

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )

    redis_url: str = Field(default="redis://127.0.0.1:6379/0", alias="REDIS_URL")
    redis_brpop_timeout: int = Field(default=5, alias="REDIS_BRPOP_TIMEOUT")
    worker_concurrency: int = Field(default=4, alias="WORKER_CONCURRENCY")

    default_timeout_ms: int = Field(default=8000, alias="SCRAPE_TIMEOUT_MS")
    fetcher_retries: int = Field(default=2, alias="FETCHER_RETRIES")
    fetcher_impersonate: str = Field(default="chrome124", alias="FETCHER_IMPERSONATE")

    stream_ttl_seconds: int = Field(default=60, alias="STREAM_TTL_SECONDS")
    chapter_cache_ttl_seconds: int = Field(default=86400, alias="CHAPTER_CACHE_TTL_SECONDS")
    proxy_ticket_ttl_seconds: int = Field(default=7200, alias="PROXY_TICKET_TTL_SECONDS")

    sandbox_timeout_seconds: float = Field(default=2.0, alias="SANDBOX_TIMEOUT_SECONDS")
    novel_chunk_size: int = Field(default=5, alias="NOVEL_CHUNK_SIZE")

    user_agent: str = Field(
        default=(
            "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
            "AppleWebKit/537.36 (KHTML, like Gecko) "
            "Chrome/124.0.0.0 Safari/537.36"
        ),
        alias="SCRAPER_UA",
    )


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    """Cached settings loader for the worker process."""
    return Settings()
