#!/usr/bin/env python3
"""Fictional source-site mock with common anti-hotlink / XOR / obfuscation tricks."""

from __future__ import annotations

import argparse
import mimetypes
import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import unquote, urlparse

STATIC = Path(__file__).resolve().parent / "static"

# Single-byte XOR key for manga images (must match demo_manga_js.json xor_key).
XOR_KEY = int(os.getenv("MOCK_XOR_KEY", "12"))
CHAPTER_PATH = "/manga/chapter-1.html"
ALLOWED_REFERER_PREFIXES = (
    "http://mock-source:8090/",
    "http://127.0.0.1:8090/",
    "http://localhost:8090/",
)
SOFT_COOKIE_NAME = "mock_session"
SOFT_COOKIE_VALUE = "ok"


def _xor_bytes(data: bytes, key: int) -> bytes:
    if key <= 0:
        return data
    return bytes(b ^ key for b in data)


class Handler(BaseHTTPRequestHandler):
    server_version = "mock-source/2.0-hostile"

    def log_message(self, fmt: str, *args) -> None:
        sys_stderr = __import__("sys").stderr
        print(f"[mock-source] {self.address_string()} - {fmt % args}", file=sys_stderr)

    def do_GET(self) -> None:  # noqa: N802
        path = unquote(self.path.split("?", 1)[0])
        if path in ("/", "/healthz"):
            self._text(200, "ok\n", "text/plain; charset=utf-8")
            return
        if path == "/novel/chapter-1.html":
            self._file(STATIC / "novel_chapter_1.html", "text/html; charset=utf-8")
            return
        if path == CHAPTER_PATH:
            self._file(STATIC / "manga_chapter_1.html", "text/html; charset=utf-8")
            return
        if path.startswith("/manga/img/"):
            self._serve_protected_image(path)
            return
        self._text(404, "not found\n", "text/plain; charset=utf-8")

    def _serve_protected_image(self, path: str) -> None:
        name = path[len("/manga/img/") :]
        if "/" in name or name.startswith(".") or not name:
            self._text(400, "bad name\n", "text/plain; charset=utf-8")
            return

        referer = self.headers.get("Referer", "") or ""
        if not self._referer_ok(referer):
            self._text(403, "hotlink forbidden: bad or missing Referer\n", "text/plain; charset=utf-8")
            return

        # Soft cookie check: missing cookie is allowed when Referer is valid (SPEC ticket has no Cookie field).
        cookie = self.headers.get("Cookie", "") or ""
        if SOFT_COOKIE_NAME + "=" + SOFT_COOKIE_VALUE not in cookie.replace(" ", ""):
            self.log_message(
                "soft-cookie miss on %s (Referer ok; still serving)", path
            )

        file_path = STATIC / name
        if not file_path.is_file():
            self._text(404, "not found\n", "text/plain; charset=utf-8")
            return
        raw = file_path.read_bytes()
        body = _xor_bytes(raw, XOR_KEY)
        self.send_response(200)
        self.send_header("Content-Type", "application/octet-stream")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("X-Mock-XOR-Key", str(XOR_KEY))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        self.wfile.write(body)

    def _referer_ok(self, referer: str) -> bool:
        if not referer:
            return False
        try:
            parsed = urlparse(referer)
        except Exception:
            return False
        # Exact chapter page, or same-origin prefix under mock-source.
        if referer.rstrip("/") == ("http://mock-source:8090" + CHAPTER_PATH).rstrip("/"):
            return True
        if referer.rstrip("/") == ("http://127.0.0.1:8090" + CHAPTER_PATH).rstrip("/"):
            return True
        if referer.rstrip("/") == ("http://localhost:8090" + CHAPTER_PATH).rstrip("/"):
            return True
        for prefix in ALLOWED_REFERER_PREFIXES:
            if referer.startswith(prefix):
                # Require path under /manga/ when host matches mock
                if parsed.path.startswith("/manga/") or parsed.path in ("", "/"):
                    return True
                if parsed.path.startswith("/novel/"):
                    # novel referer should not unlock manga imgs
                    return False
        return False

    def _file(self, file_path: Path, content_type: str | None = None) -> None:
        if not file_path.is_file():
            self._text(404, "not found\n", "text/plain; charset=utf-8")
            return
        data = file_path.read_bytes()
        ctype = content_type or mimetypes.guess_type(str(file_path))[0] or "application/octet-stream"
        self.send_response(200)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(data)))
        self.send_header("Cache-Control", "no-cache")
        self.end_headers()
        self.wfile.write(data)

    def _text(self, code: int, body: str, content_type: str) -> None:
        data = body.encode("utf-8")
        self.send_response(code)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)


def main() -> None:
    parser = argparse.ArgumentParser(description="hostile fictional mock source")
    parser.add_argument("--host", default=os.getenv("MOCK_HOST", "0.0.0.0"))
    parser.add_argument("--port", type=int, default=int(os.getenv("MOCK_PORT", "8090")))
    args = parser.parse_args()
    httpd = ThreadingHTTPServer((args.host, args.port), Handler)
    print(
        f"mock-source listening on http://{args.host}:{args.port} xor_key={XOR_KEY}",
        flush=True,
    )
    httpd.serve_forever()


if __name__ == "__main__":
    main()
