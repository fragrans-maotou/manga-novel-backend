#!/usr/bin/env python3
"""Lightweight fictional source-site mock (stdlib only, no Flask/FastAPI)."""

from __future__ import annotations

import argparse
import mimetypes
import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import unquote

STATIC = Path(__file__).resolve().parent / "static"


class Handler(BaseHTTPRequestHandler):
    server_version = "mock-source/1.0"

    def log_message(self, fmt: str, *args) -> None:  # quieter docker logs
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
        if path == "/manga/chapter-1.html":
            self._file(STATIC / "manga_chapter_1.html", "text/html; charset=utf-8")
            return
        if path.startswith("/manga/img/"):
            name = path[len("/manga/img/") :]
            if "/" in name or name.startswith(".") or not name:
                self._text(400, "bad name\n", "text/plain; charset=utf-8")
                return
            self._file(STATIC / name, "image/png")
            return
        self._text(404, "not found\n", "text/plain; charset=utf-8")

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
    parser = argparse.ArgumentParser(description="mock fictional source site")
    parser.add_argument("--host", default=os.getenv("MOCK_HOST", "0.0.0.0"))
    parser.add_argument("--port", type=int, default=int(os.getenv("MOCK_PORT", "8090")))
    args = parser.parse_args()
    httpd = ThreadingHTTPServer((args.host, args.port), Handler)
    print(f"mock-source listening on http://{args.host}:{args.port}", flush=True)
    httpd.serve_forever()


if __name__ == "__main__":
    main()
