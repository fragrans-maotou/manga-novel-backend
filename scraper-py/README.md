# scraper-py

Async scrape worker for `manga-novel-backend`. No HTTP web framework.

```bash
uv sync
uv run python -m app.cli --help
uv run python -m app.cli --rule app/rules/demo_novel.json --html fixtures/novel_chapter.html
uv run python -m app.cli --rule app/rules/demo_manga_js.json --html fixtures/manga_chapter.html
uv run python -m app.main   # Redis worker (needs REDIS_URL)
```
