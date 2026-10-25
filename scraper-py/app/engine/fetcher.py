"""HTTP fetcher using curl_cffi AsyncSession with Chrome TLS impersonation."""

from __future__ import annotations

import asyncio
import logging
from typing import Any, Mapping

from curl_cffi.requests import AsyncSession

logger = logging.getLogger(__name__)


class FetchError(RuntimeError):
    """Raised when the upstream HTTP fetch ultimately fails."""


class Fetcher:
    """Thin async wrapper around curl_cffi.AsyncSession."""

    def __init__(
        self,
        *,
        impersonate: str = "chrome124",
        retries: int = 2,
        default_timeout_s: float = 8.0,
        default_headers: Mapping[str, str] | None = None,
        session: AsyncSession | None = None,
    ) -> None:
        self._impersonate = impersonate
        self._retries = max(0, retries)
        self._default_timeout_s = default_timeout_s
        self._default_headers = dict(default_headers or {})
        self._session = session
        self._owns_session = session is None

    async def __aenter__(self) -> "Fetcher":
        if self._session is None:
            self._session = AsyncSession()
            self._owns_session = True
        return self

    async def __aexit__(self, *exc: object) -> None:
        await self.aclose()

    async def aclose(self) -> None:
        if self._owns_session and self._session is not None:
            await self._session.close()
            self._session = None

    async def get_text(
        self,
        url: str,
        *,
        timeout_s: float | None = None,
        headers: Mapping[str, str] | None = None,
        referer: str | None = None,
    ) -> str:
        """GET url and return decoded text body with retries."""
        if not url:
            raise FetchError("url is empty")
        timeout = self._default_timeout_s if timeout_s is None else timeout_s
        merged = dict(self._default_headers)
        if headers:
            merged.update(headers)
        if referer:
            merged["Referer"] = referer

        if self._session is None:
            self._session = AsyncSession()
            self._owns_session = True

        last_err: Exception | None = None
        attempts = self._retries + 1
        for attempt in range(1, attempts + 1):
            try:
                resp = await self._session.get(
                    url,
                    headers=merged or None,
                    timeout=timeout,
                    impersonate=self._impersonate,
                    allow_redirects=True,
                )
                if resp.status_code >= 400:
                    raise FetchError(f"HTTP {resp.status_code} for {url}")
                text = resp.text
                if text is None:
                    raise FetchError(f"empty body for {url}")
                return text
            except Exception as exc:  # noqa: BLE001 — retry then wrap
                last_err = exc
                logger.warning(
                    "fetch attempt %s/%s failed url=%s err=%s",
                    attempt,
                    attempts,
                    url,
                    exc,
                )
                if attempt < attempts:
                    await asyncio.sleep(min(0.25 * attempt, 1.0))
        raise FetchError(f"fetch failed for {url}: {last_err}") from last_err

    async def get_bytes(
        self,
        url: str,
        *,
        timeout_s: float | None = None,
        headers: Mapping[str, str] | None = None,
    ) -> bytes:
        """GET url and return raw bytes (used rarely; prefer get_text for HTML)."""
        # Reuse get_text path via session for consistency on headers/TLS.
        if self._session is None:
            self._session = AsyncSession()
            self._owns_session = True
        timeout = self._default_timeout_s if timeout_s is None else timeout_s
        merged: dict[str, Any] = dict(self._default_headers)
        if headers:
            merged.update(headers)
        resp = await self._session.get(
            url,
            headers=merged or None,
            timeout=timeout,
            impersonate=self._impersonate,
            allow_redirects=True,
        )
        if resp.status_code >= 400:
            raise FetchError(f"HTTP {resp.status_code} for {url}")
        return resp.content
