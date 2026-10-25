# syntax=docker/dockerfile:1.7

FROM ghcr.io/astral-sh/uv:python3.12-bookworm-slim

WORKDIR /app

# System libs sometimes needed by lxml / curl_cffi wheels.
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/* \
  && useradd --create-home --uid 10001 --shell /usr/sbin/nologin app

ENV UV_COMPILE_BYTECODE=1 \
    UV_LINK_MODE=copy \
    PYTHONUNBUFFERED=1 \
    RULES_DIR=/app/app/rules

# Dependency layer first for better cache.
COPY pyproject.toml uv.lock README.md ./
COPY app ./app
COPY fixtures ./fixtures

RUN uv sync --frozen --no-dev \
  && chown -R app:app /app

USER app

# Worker entry — no web framework.
CMD ["uv", "run", "--no-dev", "python", "-m", "app.main"]
