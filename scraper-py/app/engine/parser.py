"""Declarative JSON scrape rules (Pydantic) and HTML/JS extraction."""

from __future__ import annotations

import logging
import re
from typing import Any, Literal
from urllib.parse import urljoin

from lxml import html as lxml_html
from pydantic import BaseModel, Field, field_validator, model_validator

from app.engine.sandbox import SandboxError, run_js

logger = logging.getLogger(__name__)


class NovelRule(BaseModel):
    paragraph_xpath: str | None = None
    paragraph_css: str | None = None

    @model_validator(mode="after")
    def _need_one_selector(self) -> "NovelRule":
        if not self.paragraph_xpath and not self.paragraph_css:
            raise ValueError("novel rule requires paragraph_xpath or paragraph_css")
        return self


class MangaJSExtract(BaseModel):
    """Extract a raw string from the page, then evaluate js_body in QuickJS."""

    script_xpath: str | None = None
    script_css: str | None = None
    # If set, use this literal as sandbox input instead of DOM extract (fixtures/tests).
    input_literal: str | None = None
    js_body: str = Field(
        ...,
        description="Function body receiving `raw` string; must return JSON array of image URLs",
    )

    @model_validator(mode="after")
    def _need_input_source(self) -> "MangaJSExtract":
        if not self.input_literal and not self.script_xpath and not self.script_css:
            raise ValueError("js_extract needs script_xpath, script_css, or input_literal")
        return self


class MangaRule(BaseModel):
    image_xpath: str | None = None
    image_css: str | None = None
    js_extract: MangaJSExtract | None = None
    xor_key: str = ""
    # Absolute URL join base override; else uses page URL / rule base_url.
    image_base_url: str | None = None

    @model_validator(mode="after")
    def _need_extractor(self) -> "MangaRule":
        if not self.image_xpath and not self.image_css and self.js_extract is None:
            raise ValueError("manga rule requires image_xpath/image_css or js_extract")
        return self


class ScrapeRule(BaseModel):
    source_id: str
    name: str
    media_type: Literal["novel", "manga"]
    base_url: str = ""
    encoding: str = "utf-8"
    headers: dict[str, str] = Field(default_factory=dict)
    novel: NovelRule | None = None
    manga: MangaRule | None = None
    sandbox_timeout_seconds: float = 2.0

    @field_validator("source_id")
    @classmethod
    def _non_empty_source(cls, v: str) -> str:
        if not v.strip():
            raise ValueError("source_id is empty")
        return v.strip()

    @model_validator(mode="after")
    def _media_section(self) -> "ScrapeRule":
        if self.media_type == "novel" and self.novel is None:
            raise ValueError("media_type=novel requires novel section")
        if self.media_type == "manga" and self.manga is None:
            raise ValueError("media_type=manga requires manga section")
        return self


class ParseError(RuntimeError):
    """Raised when HTML/JS extraction fails."""


_WS_RE = re.compile(r"[ \t\u00a0]+")
_MULTI_NL_RE = re.compile(r"\n{3,}")


def _clean_text(value: str) -> str:
    text = value.replace("\r\n", "\n").replace("\r", "\n")
    text = _WS_RE.sub(" ", text)
    text = "\n".join(line.strip() for line in text.split("\n"))
    text = _MULTI_NL_RE.sub("\n\n", text)
    return text.strip()


def _select_texts(doc: lxml_html.HtmlElement, xpath: str | None, css: str | None) -> list[str]:
    nodes: list[Any] = []
    if xpath:
        nodes = doc.xpath(xpath)
    elif css:
        nodes = doc.cssselect(css)
    out: list[str] = []
    for node in nodes:
        if isinstance(node, str):
            t = _clean_text(node)
        else:
            t = _clean_text("".join(node.itertext()))
        if t:
            out.append(t)
    return out


def _select_attrs(
    doc: lxml_html.HtmlElement,
    xpath: str | None,
    css: str | None,
) -> list[str]:
    nodes: list[Any] = []
    if xpath:
        nodes = doc.xpath(xpath)
    elif css:
        nodes = doc.cssselect(css)
    out: list[str] = []
    for node in nodes:
        if isinstance(node, str):
            v = node.strip()
        else:
            v = (node.get("src") or node.get("data-src") or node.get("href") or "").strip()
            if not v:
                v = _clean_text("".join(node.itertext()))
        if v:
            out.append(v)
    return out


def parse_novel_paragraphs(html_text: str, rule: ScrapeRule) -> list[str]:
    if rule.novel is None:
        raise ParseError("novel rule missing")
    try:
        doc = lxml_html.fromstring(html_text)
    except Exception as exc:  # noqa: BLE001
        raise ParseError(f"invalid HTML: {exc}") from exc
    paras = _select_texts(doc, rule.novel.paragraph_xpath, rule.novel.paragraph_css)
    if not paras:
        raise ParseError("no novel paragraphs matched")
    return paras


def parse_manga_image_urls(
    html_text: str,
    rule: ScrapeRule,
    *,
    page_url: str = "",
) -> list[str]:
    if rule.manga is None:
        raise ParseError("manga rule missing")
    manga = rule.manga
    urls: list[str] = []

    if manga.js_extract is not None:
        raw = manga.js_extract.input_literal
        if raw is None:
            try:
                doc = lxml_html.fromstring(html_text)
            except Exception as exc:  # noqa: BLE001
                raise ParseError(f"invalid HTML: {exc}") from exc
            chunks = _select_texts(
                doc,
                manga.js_extract.script_xpath,
                manga.js_extract.script_css,
            )
            if not chunks:
                raise ParseError("js_extract selector matched nothing")
            raw = "\n".join(chunks)
        try:
            data = run_js(
                manga.js_extract.js_body,
                raw,
                timeout_seconds=rule.sandbox_timeout_seconds,
            )
        except SandboxError as exc:
            raise ParseError(str(exc)) from exc
        for item in data:
            if isinstance(item, str) and item.strip():
                urls.append(item.strip())
            elif isinstance(item, dict):
                u = str(item.get("url") or item.get("src") or "").strip()
                if u:
                    urls.append(u)
    else:
        try:
            doc = lxml_html.fromstring(html_text)
        except Exception as exc:  # noqa: BLE001
            raise ParseError(f"invalid HTML: {exc}") from exc
        urls = _select_attrs(doc, manga.image_xpath, manga.image_css)

    if not urls:
        raise ParseError("no manga image URLs extracted")

    base = manga.image_base_url or page_url or rule.base_url or ""
    abs_urls: list[str] = []
    seen: set[str] = set()
    for u in urls:
        full = urljoin(base, u) if base else u
        if full in seen:
            continue
        seen.add(full)
        abs_urls.append(full)
    return abs_urls
