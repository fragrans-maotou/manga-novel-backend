# manga-novel-backend

漫画与小说多源聚合阅读平台的**可运行成品后端**（非学习仓）。

- 学习仓（边写边学）：[manga-novel-platform](https://github.com/fragrans-maotou/manga-novel-platform)
- 本仓：[manga-novel-backend](https://github.com/fragrans-maotou/manga-novel-backend)
- 架构与 Redis/SQL 契约以根目录 [`SPEC.md`](./SPEC.md) 为准

## 组件

| 服务 | 技术 | 职责 |
|------|------|------|
| `gateway-go` | Go 1.23 + Fiber | API、singleflight、SSE、32KB 图片流式代理 |
| `scraper-py` | Python 3.12 + uv | Redis 队列 Worker（无 Web 框架）、curl_cffi + QuickJS |
| `postgres` | 16 | 作品 / 源 / 章节 / 进度 |
| `redis` | 7 | 队列、stream、cache、ticket、健康度 |

## 快速启动

> **说明**：`migrations/001_init.sql` 会通过 `postgres/Dockerfile` 复制进 Postgres 镜像的
> `/docker-entrypoint-initdb.d/`（`make up` / `make build` 会先 `sync-migration`）。
> 这样即使 Docker daemon 无法 bind-mount 宿主机文件，首次空库仍能自动建表。


```bash
cp .env.example .env   # 按需修改密码/端口
make up                # docker compose up -d --build
make health            # curl /healthz
make logs              # 跟随日志
make down              # 停止（保留数据卷）
```

若当前环境的 Docker daemon 在 TCP 上（例如沙箱 `2375`）：

```bash
export DOCKER_HOST=tcp://127.0.0.1:2375
make up
```

### 主要环境变量（见 `.env.example`）

- **gateway**：`DATABASE_URL`、`REDIS_ADDR`、`HTTP_ADDR`、`SCRAPE_TIMEOUT_MS`
- **scraper**：`REDIS_URL`、`WORKER_CONCURRENCY`、`FETCHER_IMPERSONATE`、`RULES_DIR`
- **端口**：`GATEWAY_HTTP_PORT`（默认 8080）、`POSTGRES_PORT`、`REDIS_PORT`

## 主要 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/healthz` | 健康检查（含 postgres/redis ping） |
| GET | `/api/v1/works` | 作品列表（`?media_type=&limit=&offset=`） |
| GET | `/api/v1/works/:workID` | 作品详情 |
| GET | `/api/v1/works/:workID/chapters` | 章节目录 |
| GET | `/api/v1/chapters/:chapterID/novel` | 小说 SSE |
| GET | `/api/v1/chapters/:chapterID/manga` | 漫画章节 JSON |
| GET | `/api/v1/proxy/image/:ticket` | 漫画图片流式代理（可选 XOR） |

## 离线测采集规则（CLI）

不依赖 Redis，用仓库内虚构 fixture：

```bash
make cli-novel   # 小说段落解析
make cli-manga   # 漫画 QuickJS 沙盒解出图片 URL
# 或本机：
cd scraper-py && uv sync && \
  uv run python -m app.cli --rule app/rules/demo_novel.json --html fixtures/novel_chapter.html --pretty
```

## 验证清单

```bash
make config                 # compose 配置合法
make up
make health                 # gateway /healthz
make redis-ping             # PONG
make sql                    # 应看到 works / scraper_sources / work_sources / chapters / reading_progress
docker compose logs scraper-py | head   # 应显示 listening queues=queue:scrape:realtime,...
make cli-test
make smoke                  # 一键冒烟（含 CLI）
```

## 开发阶段对照

1. migrations + Go repository  
2. Go service / handler / main  
3. Python scraper + CLI  
4. **Docker 多阶段 + compose + Makefile（本阶段）**

## License

见 [LICENSE](./LICENSE)。
