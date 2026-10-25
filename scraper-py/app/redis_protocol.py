"""SPEC §5 Redis key helpers and write-back helpers (redis.asyncio)."""

from __future__ import annotations

import hashlib
import json
import logging
from typing import Any

from redis.asyncio import Redis

logger = logging.getLogger(__name__)

QUEUE_REALTIME = "queue:scrape:realtime"
QUEUE_CRON = "queue:scrape:cron"

STREAM_TTL = 60
CHAPTER_CACHE_TTL = 86400
PROXY_TICKET_TTL = 7200

HEALTH_SUCCESS_DELTA = 1.0
HEALTH_SCORE_MAX = 100.0


def key_stream(task_id: str) -> str:
    return f"stream:{task_id}"


def key_chapter_cache(chapter_id: str) -> str:
    return f"cache:chapter:{chapter_id}"


def key_proxy_ticket(ticket: str) -> str:
    return f"proxy:ticket:{ticket}"


def key_health_sources(media_type: str) -> str:
    return f"health:sources:{media_type}"


def md5_ticket16(url: str, salt: str = "") -> str:
    """Return first 16 hex chars of MD5(url+salt) — SPEC §5.2 '16 位 MD5 Ticket'."""
    digest = hashlib.md5(f"{salt}{url}".encode("utf-8")).hexdigest()
    return digest[:16]


async def stream_rpush_expire(
    rdb: Redis,
    task_id: str,
    value: str,
    *,
    ttl: int = STREAM_TTL,
) -> None:
    key = key_stream(task_id)
    await rdb.rpush(key, value)
    await rdb.expire(key, ttl)


async def push_error(rdb: Redis, task_id: str, reason: str, *, ttl: int = STREAM_TTL) -> None:
    safe = reason.replace("\n", " ").strip() or "unknown"
    if len(safe) > 500:
        safe = safe[:500] + "..."
    await stream_rpush_expire(rdb, task_id, f"[ERROR]:{safe}", ttl=ttl)


async def push_eof(rdb: Redis, task_id: str, *, ttl: int = STREAM_TTL) -> None:
    await stream_rpush_expire(rdb, task_id, "[EOF]", ttl=ttl)


async def push_ready(rdb: Redis, task_id: str, *, ttl: int = STREAM_TTL) -> None:
    await stream_rpush_expire(rdb, task_id, "READY", ttl=ttl)


async def write_novel_cache(
    rdb: Redis,
    chapter_id: str,
    body: str,
    *,
    ttl: int = CHAPTER_CACHE_TTL,
) -> None:
    await rdb.set(key_chapter_cache(chapter_id), body, ex=ttl)


async def write_manga_cache(
    rdb: Redis,
    chapter_id: str,
    image_proxy_paths: list[str],
    *,
    ttl: int = CHAPTER_CACHE_TTL,
) -> None:
    payload = json.dumps(
        {"chapter_id": chapter_id, "images": image_proxy_paths},
        ensure_ascii=False,
        separators=(",", ":"),
    )
    await rdb.set(key_chapter_cache(chapter_id), payload, ex=ttl)


async def write_proxy_ticket(
    rdb: Redis,
    ticket: str,
    *,
    url: str,
    referer: str,
    ua: str,
    xor_key: str = "",
    ttl: int = PROXY_TICKET_TTL,
) -> None:
    key = key_proxy_ticket(ticket)
    await rdb.hset(
        key,
        mapping={
            "url": url,
            "referer": referer,
            "ua": ua,
            "xor_key": xor_key or "",
        },
    )
    await rdb.expire(key, ttl)


async def health_success(
    rdb: Redis,
    media_type: str,
    source_id: str,
) -> float:
    """ZINCRBY +1 then clamp to 100 (SPEC §5.5)."""
    key = key_health_sources(media_type)
    score = await rdb.zincrby(key, HEALTH_SUCCESS_DELTA, source_id)
    if score > HEALTH_SCORE_MAX:
        await rdb.zadd(key, {source_id: HEALTH_SCORE_MAX})
        return HEALTH_SCORE_MAX
    return float(score)


def parse_task_payload(raw: str | bytes) -> dict[str, Any]:
    if isinstance(raw, bytes):
        raw = raw.decode("utf-8")
    data = json.loads(raw)
    if not isinstance(data, dict):
        raise ValueError("task payload must be a JSON object")
    for field in ("task_id", "media_type", "source_id", "chapter_id", "remote_url"):
        if not data.get(field):
            raise ValueError(f"task missing field: {field}")
    return data
