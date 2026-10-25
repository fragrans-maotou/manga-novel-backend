from __future__ import annotations

import json
from pathlib import Path

import pytest

from app.engine.sandbox import SandboxError, run_js
from app.worker import load_rule_file, offline_parse

ROOT = Path(__file__).resolve().parents[1]
RULES = ROOT / "app" / "rules"
FIX = ROOT / "fixtures"


def test_offline_novel_fixture():
    rule = load_rule_file(RULES / "demo_novel.json")
    html = (FIX / "novel_chapter.html").read_text(encoding="utf-8")
    result = offline_parse(rule, html)
    assert result["media_type"] == "novel"
    assert result["count"] >= 6
    assert "第一段" in result["paragraphs"][0]


def test_offline_manga_sandbox_fixture():
    rule = load_rule_file(RULES / "demo_manga_js.json")
    html = (FIX / "manga_chapter.html").read_text(encoding="utf-8")
    result = offline_parse(rule, html, page_url=rule.base_url)
    assert result["media_type"] == "manga"
    assert result["count"] == 4
    assert result["images"][0].startswith("https://cdn.demo-manga.example.invalid/i/")
    assert result["images"][0].endswith("a01.png")


def test_sandbox_timeout():
    with pytest.raises(SandboxError, match="timed out"):
        run_js("while(true){}", "", timeout_seconds=0.3)


def test_sandbox_returns_json_array():
    out = run_js("return ['a','b'];", "", timeout_seconds=1.0)
    assert out == ["a", "b"]


def test_md5_ticket_len():
    from app.redis_protocol import md5_ticket16

    t = md5_ticket16("https://cdn.example/x.jpg", salt="chap")
    assert len(t) == 16
    assert all(c in "0123456789abcdef" for c in t)
