"""Per-task scrape orchestration with mandatory [ERROR]/[EOF]/READY write-back."""

from __future__ import annotations

import logging
from pathlib import Path
from typing import Any

from redis.asyncio import Redis

from app.config import Settings
from app.engine.fetcher import FetchError, Fetcher
from app.engine.parser import (
    ParseError,
    ScrapeRule,
    parse_manga_image_urls,
    parse_novel_paragraphs,
)
from app.redis_protocol import (
    health_success,
    md5_ticket16,
    parse_task_payload,
    push_eof,
    push_error,
    push_ready,
    stream_rpush_expire,
    write_manga_cache,
    write_novel_cache,
    write_proxy_ticket,
)

logger = logging.getLogger(__name__)


class RuleStore:
    """Load ScrapeRule JSON files from a directory keyed by source_id."""

    def __init__(self, rules_dir: Path) -> None:
        self._dir = rules_dir
        self._cache: dict[str, ScrapeRule] = {}

    def get(self, source_id: str) -> ScrapeRule:
        if source_id in self._cache:
            return self._cache[source_id]
        path = self._dir / f"{source_id}.json"
        if not path.is_file():
            # Also allow any *.json whose source_id field matches.
            for candidate in sorted(self._dir.glob("*.json")):
                rule = ScrapeRule.model_validate_json(candidate.read_text(encoding="utf-8"))
                self._cache[rule.source_id] = rule
                if rule.source_id == source_id:
                    return rule
            raise FileNotFoundError(f"no rule file for source_id={source_id} in {self._dir}")
        rule = ScrapeRule.model_validate_json(path.read_text(encoding="utf-8"))
        if rule.source_id != source_id:
            raise ValueError(f"rule file {path} source_id={rule.source_id} != {source_id}")
        self._cache[source_id] = rule
        return rule


async def handle_task(
    rdb: Redis,
    fetcher: Fetcher,
    rules: RuleStore,
    settings: Settings,
    raw_payload: str | bytes,
) -> None:
    """Process one queue payload. Always terminates Go's BLPOP via stream write-back."""
    task_id = ""
    try:
        task = parse_task_payload(raw_payload)
        task_id = str(task["task_id"])
        media_type = str(task["media_type"])
        source_id = str(task["source_id"])
        chapter_id = str(task["chapter_id"])
        remote_url = str(task["remote_url"])
        timeout_ms = int(task.get("timeout_ms") or settings.default_timeout_ms)

        rule = rules.get(source_id)
        if rule.media_type != media_type:
            raise ValueError(
                f"task media_type={media_type} mismatches rule media_type={rule.media_type}"
            )

        html_text = await fetcher.get_text(
            remote_url,
            timeout_s=max(timeout_ms / 1000.0, 1.0),
            headers=rule.headers,
            referer=rule.base_url or None,
        )

        if media_type == "novel":
            await _handle_novel(
                rdb, settings, rule, task_id, chapter_id, source_id, html_text
            )
        elif media_type == "manga":
            await _handle_manga(
                rdb,
                settings,
                rule,
                task_id=task_id,
                chapter_id=chapter_id,
                source_id=source_id,
                remote_url=remote_url,
                html_text=html_text,
            )
        else:
            raise ValueError(f"unsupported media_type: {media_type}")

        try:
            await health_success(rdb, media_type, source_id)
        except Exception as exc:  # noqa: BLE001 — health is best-effort
            logger.warning("health_success failed source=%s err=%s", source_id, exc)

    except Exception as exc:  # noqa: BLE001 — never let Go BLPOP hang
        logger.exception("task failed task_id=%s", task_id or "?")
        if task_id:
            try:
                await push_error(rdb, task_id, str(exc), ttl=settings.stream_ttl_seconds)
            except Exception as push_exc:  # noqa: BLE001
                logger.error("failed to push [ERROR] task_id=%s err=%s", task_id, push_exc)
        else:
            logger.error("cannot push [ERROR]: task_id unknown; payload=%r", raw_payload)


async def _handle_novel(
    rdb: Redis,
    settings: Settings,
    rule: ScrapeRule,
    task_id: str,
    chapter_id: str,
    source_id: str,
    html_text: str,
) -> None:
    paragraphs = parse_novel_paragraphs(html_text, rule)
    chunk_size = max(1, settings.novel_chunk_size)
    buf: list[str] = []
    full_parts: list[str] = []

    async def flush() -> None:
        nonlocal buf
        if not buf:
            return
        block = "\n".join(buf)
        full_parts.append(block)
        await stream_rpush_expire(
            rdb, task_id, block, ttl=settings.stream_ttl_seconds
        )
        buf = []

    for para in paragraphs:
        buf.append(para)
        if len(buf) >= chunk_size:
            await flush()
    await flush()

    body = "\n\n".join(full_parts)
    await write_novel_cache(
        rdb, chapter_id, body, ttl=settings.chapter_cache_ttl_seconds
    )
    await push_eof(rdb, task_id, ttl=settings.stream_ttl_seconds)
    logger.info(
        "novel done source=%s chapter=%s paragraphs=%s",
        source_id,
        chapter_id,
        len(paragraphs),
    )


async def _handle_manga(
    rdb: Redis,
    settings: Settings,
    rule: ScrapeRule,
    *,
    task_id: str,
    chapter_id: str,
    source_id: str,
    remote_url: str,
    html_text: str,
) -> None:
    urls = parse_manga_image_urls(html_text, rule, page_url=remote_url)
    xor_key = rule.manga.xor_key if rule.manga else ""
    proxy_paths: list[str] = []
    for url in urls:
        ticket = md5_ticket16(url, salt=chapter_id)
        await write_proxy_ticket(
            rdb,
            ticket,
            url=url,
            referer=remote_url,
            ua=settings.user_agent,
            xor_key=xor_key,
            ttl=settings.proxy_ticket_ttl_seconds,
        )
        proxy_paths.append(f"/api/v1/proxy/image/{ticket}")

    await write_manga_cache(
        rdb,
        chapter_id,
        proxy_paths,
        ttl=settings.chapter_cache_ttl_seconds,
    )
    await push_ready(rdb, task_id, ttl=settings.stream_ttl_seconds)
    logger.info(
        "manga done source=%s chapter=%s images=%s",
        source_id,
        chapter_id,
        len(proxy_paths),
    )


def load_rule_file(path: Path) -> ScrapeRule:
    return ScrapeRule.model_validate_json(path.read_text(encoding="utf-8"))


def offline_parse(
    rule: ScrapeRule,
    html_text: str,
    *,
    page_url: str = "",
) -> dict[str, Any]:
    """CLI/offline helper — no Redis."""
    if rule.media_type == "novel":
        paras = parse_novel_paragraphs(html_text, rule)
        return {"media_type": "novel", "paragraphs": paras, "count": len(paras)}
    if rule.media_type == "manga":
        urls = parse_manga_image_urls(html_text, rule, page_url=page_url or rule.base_url)
        return {"media_type": "manga", "images": urls, "count": len(urls)}
    raise ParseError(f"unsupported media_type: {rule.media_type}")


# Re-export for typing clarity
__all__ = [
    "RuleStore",
    "handle_task",
    "load_rule_file",
    "offline_parse",
    "FetchError",
    "ParseError",
]
