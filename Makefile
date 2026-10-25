# manga-novel-backend — local orchestration helpers
# If your Docker daemon listens on TCP (e.g. CI sandboxes): export DOCKER_HOST=tcp://127.0.0.1:2375

COMPOSE ?= docker compose
PROJECT ?= manga-novel-backend

.PHONY: help up down build logs ps config restart smoke cli-novel cli-manga cli-test health sql redis-ping sync-migration

help:
	@echo "Targets:"
	@echo "  make up          - build & start full stack (detached)"
	@echo "  make down        - stop stack (keep volumes)"
	@echo "  make down-v      - stop stack and delete volumes"
	@echo "  make build       - build images"
	@echo "  make logs        - follow all service logs"
	@echo "  make ps          - compose ps"
	@echo "  make config      - validate compose file"
	@echo "  make health      - curl gateway /healthz"
	@echo "  make sql         - psql into postgres (list tables)"
	@echo "  make redis-ping  - redis-cli PING"
	@echo "  make cli-novel   - offline novel rule parse (docker run)"
	@echo "  make cli-manga   - offline manga+sandbox rule parse (docker run)"
	@echo "  make cli-test    - both CLI rule tests"
	@echo "  make smoke       - up → wait healthy → healthz → cli-test"

config:
	$(COMPOSE) config >/dev/null
	@echo "compose config: OK"

# Keep postgres image SQL in sync with migrations/001_init.sql (SPEC §4).
sync-migration:
	cp migrations/001_init.sql postgres/001_init.sql
	@echo "synced migrations/001_init.sql -> postgres/001_init.sql"

build: sync-migration
	$(COMPOSE) build

up: sync-migration config
	$(COMPOSE) up -d --build
	@echo "stack starting; try: make health / make logs"


down:
	$(COMPOSE) down

down-v:
	$(COMPOSE) down -v

logs:
	$(COMPOSE) logs -f --tail=200

ps:
	$(COMPOSE) ps

restart:
	$(COMPOSE) restart

health:
	@curl -fsS "http://127.0.0.1:$${GATEWAY_HTTP_PORT:-8080}/healthz"; echo
	@curl -fsS "http://127.0.0.1:$${GATEWAY_HTTP_PORT:-8080}/healthz" | grep -q '"status"'
	@echo "healthz: OK"

sql:
	$(COMPOSE) exec -T postgres \
	  psql -U "$${POSTGRES_USER:-manga}" -d "$${POSTGRES_DB:-manga}" \
	  -c "\dt"

redis-ping:
	$(COMPOSE) exec -T redis redis-cli PING

# Offline CLI against the built scraper image (no Redis needed).
cli-novel:
	$(COMPOSE) build scraper-py
	$(COMPOSE) run --rm --no-deps --entrypoint uv scraper-py \
	  run --no-dev python -m app.cli \
	  --rule app/rules/demo_novel.json --html fixtures/novel_chapter.html --pretty

cli-manga:
	$(COMPOSE) build scraper-py
	$(COMPOSE) run --rm --no-deps --entrypoint uv scraper-py \
	  run --no-dev python -m app.cli \
	  --rule app/rules/demo_manga_js.json --html fixtures/manga_chapter.html --pretty

cli-test: cli-novel cli-manga

smoke: up
	@echo "waiting for gateway healthy..."
	@i=0; \
	until curl -fsS "http://127.0.0.1:$${GATEWAY_HTTP_PORT:-8080}/healthz" >/dev/null 2>&1; do \
	  i=$$((i+1)); \
	  if [ $$i -gt 60 ]; then echo "timeout waiting for healthz"; $(COMPOSE) ps; $(COMPOSE) logs --tail=80 gateway-go; exit 1; fi; \
	  sleep 2; \
	done
	@$(MAKE) health
	@$(MAKE) redis-ping
	@$(MAKE) sql
	@$(MAKE) cli-test
	@echo "smoke: OK"
