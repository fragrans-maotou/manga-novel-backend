# manga-novel-backend

漫画与小说多源聚合阅读平台的**可运行成品后端**（非学习仓）。

- 本仓：https://github.com/fragrans-maotou/manga-novel-backend
- 学习仓：https://github.com/fragrans-maotou/manga-novel-platform
- 契约：根目录 [`SPEC.md`](./SPEC.md)

## 五分钟看效果

```bash
export DOCKER_HOST=tcp://127.0.0.1:2375   # 仅当 Docker 在 TCP 上时需要
cp -n .env.example .env
make reset-demo    # down -v → up --build → 等 healthy → seed
make demo          # works 非空 → 小说 SSE → 漫画 JSON → 图片代理 → 进度 API
```

预期：

- `GET /api/v1/works` 至少 2 部作品（小说 + 漫画）
- 小说 SSE 出现「第一段…」「面包店」等段落
- 漫画 JSON 含 `images: ["/api/v1/proxy/image/..."]`
- 代理 `GET` 返回 200（PNG）

### 固定演示 UUID（`migrations/002_seed_demo.sql`）

| 用途 | UUID |
|------|------|
| 小说 work | `11111111-1111-1111-1111-111111111101` |
| 漫画 work | `11111111-1111-1111-1111-111111111102` |
| 小说 chapter | `33333333-3333-3333-3333-333333333301` |
| 漫画 chapter | `33333333-3333-3333-3333-333333333302` |
| 演示 user | `44444444-4444-4444-4444-444444444401` |

```bash
# 手工等价于 make demo 的一部分
curl -N http://127.0.0.1:8080/api/v1/chapters/33333333-3333-3333-3333-333333333301/novel
curl http://127.0.0.1:8080/api/v1/chapters/33333333-3333-3333-3333-333333333302/manga
```

> **图片票 TTL**：`proxy:ticket` 约 2 小时；`cache:chapter` 约 24 小时。票过期后 gateway 会丢掉章节缓存并自动重采，避免把死 ticket URL 返回给客户端。

> **数据卷提示**：Postgres 只在**空数据目录**执行 init SQL。已有卷时用 `make seed` 补种，或 `make reset-demo` 清空重来。

## 组件

| 服务 | 说明 |
|------|------|
| `gateway-go` | Go 1.23 API / SSE / 32KB 图片流式代理 |
| `scraper-py` | Python Worker（无 Web 框架） |
| `mock-source` | 虚构源站（stdlib http.server），供采集与代理 |
| `postgres:16` | 表结构 + demo seed（镜像内 001/002） |
| `redis:7` | 队列 / stream / cache / ticket / health |


## mock-source 对抗能力（本地压测）

虚构演示站模拟真站常见套路（**不含真实作品内容**）：

| 手段 | 行为 |
|------|------|
| Referer 防盗链 | `/manga/img/*` 无 Referer 或不匹配 → **403**；章节页 Referer → 200 |
| 单字节 XOR | 图响应按 `xor_key=12` 异或；gateway 按 ticket 流式解密后为 PNG |
| JS 混淆图源 | 章节 HTML 内 base64(JSON 数组)，QuickJS 规则解码；诱饵明文列表会被忽略 |
| Cookie（软校验） | 期望 `mock_session=ok`；缺 Cookie 但 Referer 正确仍放行（SPEC ticket 无 Cookie 字段） |

```bash
make demo-hostile   # 裸 curl 图 → 403；经 gateway proxy → 200 PNG
```

## 常用命令

```bash
make up / make down / make logs / make health
make seed              # 幂等写入演示数据
make reset-demo        # 干净演示环境
make demo              # 端到端验收
make cli-novel         # 离线规则（不依赖 Redis）
```

## 主要 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/healthz` | 健康检查 |
| GET | `/api/v1/works` | 作品列表 |
| GET | `/api/v1/works/:id/chapters` | 章节目录 |
| GET | `/api/v1/chapters/:id/novel` | 小说 SSE |
| GET | `/api/v1/chapters/:id/manga` | 漫画章节 JSON |
| GET | `/api/v1/proxy/image/:ticket` | 图片流式代理 |
| PUT | `/api/v1/works/:id/progress` | 阅读进度（body: user_id, chapter_id, position_offset） |
| GET | `/api/v1/works/:id/progress?user_id=` | 读取进度 |

## License

见 [LICENSE](./LICENSE)。
