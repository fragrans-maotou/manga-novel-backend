# manga-novel-backend — local orchestration helpers
# If your Docker daemon listens on TCP (e.g. CI sandboxes): export DOCKER_HOST=tcp://127.0.0.1:2375

COMPOSE ?= docker compose
GATEWAY ?= http://127.0.0.1:$(or $(GATEWAY_HTTP_PORT),8080)

# Fixed demo IDs (migrations/002_seed_demo.sql)
NOVEL_WORK_ID ?= 11111111-1111-1111-1111-111111111101
MANGA_WORK_ID ?= 11111111-1111-1111-1111-111111111102
NOVEL_CHAPTER_ID ?= 33333333-3333-3333-3333-333333333301
MANGA_CHAPTER_ID ?= 33333333-3333-3333-3333-333333333302
DEMO_USER_ID ?= 44444444-4444-4444-4444-444444444401

.PHONY: help up down down-v build logs ps config restart smoke \
	cli-novel cli-manga cli-test health sql redis-ping sync-migration \
	seed reset-demo demo wait-healthy

help:
	@echo "Targets:"
	@echo "  make up           - build & start full stack"
	@echo "  make reset-demo   - down -v + up + seed (clean demo)"
	@echo "  make seed         - apply migrations/002_seed_demo.sql via psql"
	@echo "  make demo         - curl works + novel SSE + manga JSON + proxy"
	@echo "  make health / sql / redis-ping / logs / down"

config:
	$(COMPOSE) config >/dev/null
	@echo "compose config: OK"

# Keep postgres image SQL in sync with migrations/*.sql
sync-migration:
	cp migrations/001_init.sql postgres/001_init.sql
	cp migrations/002_seed_demo.sql postgres/002_seed_demo.sql
	@echo "synced migrations -> postgres/"

build: sync-migration
	$(COMPOSE) build

up: sync-migration config
	$(COMPOSE) up -d --build
	@echo "stack starting; try: make wait-healthy && make demo"

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

wait-healthy:
	@echo "waiting for gateway healthy..."
	@i=0; \
	until curl -fsS "$(GATEWAY)/healthz" >/dev/null 2>&1; do \
	  i=$$((i+1)); \
	  if [ $$i -gt 60 ]; then echo "timeout"; $(COMPOSE) ps; $(COMPOSE) logs --tail=80; exit 1; fi; \
	  sleep 2; \
	done
	@curl -fsS "$(GATEWAY)/healthz"; echo
	@echo "healthy: OK"

health:
	@curl -fsS "$(GATEWAY)/healthz"; echo
	@curl -fsS "$(GATEWAY)/healthz" | grep -q '"status"'
	@echo "healthz: OK"

sql:
	$(COMPOSE) exec -T postgres \
	  psql -U "$${POSTGRES_USER:-manga}" -d "$${POSTGRES_DB:-manga}" \
	  -c "\dt"

redis-ping:
	$(COMPOSE) exec -T redis redis-cli PING

# Apply seed even when volume already initialized (stdin pipe, no bind-mount).
seed:
	@echo "applying migrations/002_seed_demo.sql ..."
	@cat migrations/002_seed_demo.sql | $(COMPOSE) exec -T postgres \
	  psql -U "$${POSTGRES_USER:-manga}" -d "$${POSTGRES_DB:-manga}" -v ON_ERROR_STOP=1
	@echo "seed: OK"
	@$(COMPOSE) exec -T redis redis-cli ZADD health:sources:novel 100 demo_novel >/dev/null || true
	@$(COMPOSE) exec -T redis redis-cli ZADD health:sources:manga 100 demo_manga_js >/dev/null || true
	@echo "redis health seeds: OK"

reset-demo: down-v up wait-healthy seed
	@echo "reset-demo: OK — run make demo"

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

# End-to-end demo: list works → novel SSE → manga JSON → image proxy HEAD
demo: wait-healthy
	@echo "==== GET /api/v1/works ===="
	@curl -fsS "$(GATEWAY)/api/v1/works" | tee /tmp/manga-works.json; echo
	@python3 -c 'import json; d=json.load(open("/tmp/manga-works.json")); assert len(d.get("items",[]))>=2, d; print("works:", len(d["items"]))'
	@echo "==== novel SSE (first chunks, ~12s max) ===="
	@curl -fsS -N --max-time 20 \
	  "$(GATEWAY)/api/v1/chapters/$(NOVEL_CHAPTER_ID)/novel" \
	  | tee /tmp/manga-novel-sse.txt | head -c 800; echo
	@grep -q "data:" /tmp/manga-novel-sse.txt
	@grep -E -q "晨光|第一段|面包店|灯塔" /tmp/manga-novel-sse.txt || \
	  (echo "novel SSE missing expected paragraph text"; cat /tmp/manga-novel-sse.txt; exit 1)
	@echo "==== manga chapter JSON ===="
	@curl -fsS --max-time 30 \
	  "$(GATEWAY)/api/v1/chapters/$(MANGA_CHAPTER_ID)/manga" \
	  | tee /tmp/manga-chapter.json; echo
	@python3 -c 'import json; d=json.load(open("/tmp/manga-chapter.json")); imgs=d.get("images") or []; assert len(imgs)>=1, d; print("images:", len(imgs)); open("/tmp/manga-first-image.path","w").write(imgs[0])'
	@echo "==== proxy image HEAD ===="
	@IMG_PATH=$$(cat /tmp/manga-first-image.path); \
	  curl -fsSI --max-time 20 "$(GATEWAY)$$IMG_PATH" | tee /tmp/manga-proxy.headers; \
	  grep -qi "200 OK\|HTTP/1.1 200\|HTTP/2 200" /tmp/manga-proxy.headers
	@echo "==== progress PUT/GET ===="
	@curl -fsS -X PUT "$(GATEWAY)/api/v1/works/$(NOVEL_WORK_ID)/progress" \
	  -H 'Content-Type: application/json' \
	  -d '{"user_id":"$(DEMO_USER_ID)","chapter_id":"$(NOVEL_CHAPTER_ID)","position_offset":42}' \
	  | tee /tmp/manga-progress.json; echo
	@curl -fsS "$(GATEWAY)/api/v1/works/$(NOVEL_WORK_ID)/progress?user_id=$(DEMO_USER_ID)" | grep -q '"position_offset":42'
	@echo "demo: OK"

smoke: up wait-healthy
	@$(MAKE) health
	@$(MAKE) redis-ping
	@$(MAKE) sql
	@$(MAKE) seed
	@$(MAKE) demo
	@echo "smoke: OK"
