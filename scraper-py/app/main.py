"""Async scrape worker: BRPOP realtime then cron; coroutine pool; no web framework."""

from __future__ import annotations

import asyncio
import logging
import os
import signal
import sys
from pathlib import Path

from redis.asyncio import Redis

from app.config import Settings, get_settings
from app.engine.fetcher import Fetcher
from app.redis_protocol import QUEUE_CRON, QUEUE_REALTIME
from app.worker import RuleStore, handle_task

logging.basicConfig(
    level=os.getenv("LOG_LEVEL", "INFO"),
    format="%(asctime)s %(levelname)s %(name)s %(message)s",
)
logger = logging.getLogger("scraper.main")


def _default_rules_dir() -> Path:
    return Path(__file__).resolve().parent / "rules"


async def worker_loop(
    name: str,
    rdb: Redis,
    fetcher: Fetcher,
    rules: RuleStore,
    settings: Settings,
    stop: asyncio.Event,
) -> None:
    """One consumer coroutine: BRPOP realtime first, then cron (SPEC §5.1)."""
    logger.info("worker %s started", name)
    while not stop.is_set():
        try:
            item = await rdb.brpop(
                [QUEUE_REALTIME, QUEUE_CRON],
                timeout=settings.redis_brpop_timeout,
            )
        except asyncio.CancelledError:
            raise
        except Exception as exc:  # noqa: BLE001
            logger.error("worker %s BRPOP error: %s", name, exc)
            await asyncio.sleep(1.0)
            continue

        if item is None:
            continue
        _queue_name, payload = item
        try:
            await handle_task(rdb, fetcher, rules, settings, payload)
        except Exception as exc:  # noqa: BLE001
            logger.exception("worker %s unhandled: %s", name, exc)
    logger.info("worker %s stopped", name)


async def async_main(settings: Settings | None = None) -> None:
    settings = settings or get_settings()
    rules_dir = Path(os.getenv("RULES_DIR", str(_default_rules_dir())))
    rules = RuleStore(rules_dir)

    rdb = Redis.from_url(settings.redis_url, decode_responses=True)
    fetcher = Fetcher(
        impersonate=settings.fetcher_impersonate,
        retries=settings.fetcher_retries,
        default_timeout_s=settings.default_timeout_ms / 1000.0,
        default_headers={"User-Agent": settings.user_agent},
    )

    stop = asyncio.Event()
    loop = asyncio.get_running_loop()

    def _ask_stop() -> None:
        logger.info("shutdown signal received")
        stop.set()

    for sig in (signal.SIGINT, signal.SIGTERM):
        try:
            loop.add_signal_handler(sig, _ask_stop)
        except NotImplementedError:
            signal.signal(sig, lambda *_: _ask_stop())

    await fetcher.__aenter__()
    try:
        n = max(1, settings.worker_concurrency)
        tasks = [
            asyncio.create_task(
                worker_loop(f"w{i}", rdb, fetcher, rules, settings, stop),
                name=f"scrape-worker-{i}",
            )
            for i in range(n)
        ]
        logger.info(
            "scraper listening queues=%s,%s concurrency=%s rules_dir=%s",
            QUEUE_REALTIME,
            QUEUE_CRON,
            n,
            rules_dir,
        )
        await stop.wait()
        for t in tasks:
            t.cancel()
        await asyncio.gather(*tasks, return_exceptions=True)
    finally:
        await fetcher.aclose()
        await rdb.aclose()
        logger.info("scraper exited")


def main() -> None:
    try:
        asyncio.run(async_main())
    except KeyboardInterrupt:
        sys.exit(0)


if __name__ == "__main__":
    main()
