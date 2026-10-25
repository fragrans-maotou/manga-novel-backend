"""Offline rule debugger — local JSON rule + HTML fixture or URL; no Redis required."""

from __future__ import annotations

import argparse
import asyncio
import json
import sys
from pathlib import Path

from app.engine.fetcher import Fetcher
from app.worker import load_rule_file, offline_parse


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        prog="python -m app.cli",
        description="Offline scrape-rule debugger (no Redis). Uses local HTML or URL.",
    )
    p.add_argument(
        "--rule",
        required=True,
        help="Path to scrape rule JSON (app/rules/*.json)",
    )
    src = p.add_mutually_exclusive_group(required=True)
    src.add_argument("--html", help="Local HTML fixture path")
    src.add_argument("--url", help="Fetch remote URL via curl_cffi (needs network)")
    p.add_argument(
        "--dump-html",
        metavar="PATH",
        help="Write fetched/loaded HTML to PATH for XPath tuning",
    )
    p.add_argument(
        "--pretty",
        action="store_true",
        help="Pretty-print JSON result",
    )
    return p


async def _fetch(url: str, headers: dict[str, str]) -> str:
    async with Fetcher(default_headers=headers) as fetcher:
        return await fetcher.get_text(url, headers=headers)


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    rule_path = Path(args.rule)
    if not rule_path.is_file():
        print(f"error: rule file not found: {rule_path}", file=sys.stderr)
        return 2

    rule = load_rule_file(rule_path)
    page_url = args.url or rule.base_url

    if args.html:
        html_path = Path(args.html)
        if not html_path.is_file():
            print(f"error: html fixture not found: {html_path}", file=sys.stderr)
            return 2
        html_text = html_path.read_text(encoding="utf-8")
    else:
        html_text = asyncio.run(_fetch(args.url, rule.headers))

    if args.dump_html:
        dump_path = Path(args.dump_html)
        dump_path.parent.mkdir(parents=True, exist_ok=True)
        dump_path.write_text(html_text, encoding="utf-8")
        print(f"# dumped HTML -> {dump_path}", file=sys.stderr)

    result = offline_parse(rule, html_text, page_url=page_url or "")
    if args.pretty:
        print(json.dumps(result, ensure_ascii=False, indent=2))
    else:
        print(json.dumps(result, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
