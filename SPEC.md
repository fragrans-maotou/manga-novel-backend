# 漫画与小说多源聚合阅读平台 - 工业级后端架构规范 (SPEC.md)

## 1. 项目背景与核心要解决的工程问题
本项目构建一个高性能、支持多源自动故障转移（Failover）的漫画与小说聚合阅读后端系统。系统必须解决以下四个核心工程痛点：
1. **上游反爬对抗与热更新需求**：外部漫画与小说源站普遍存在 TLS 指纹校验、Cloudflare 质询、动态 JS 混淆图源数组、图片字节流异或（XOR）加密及防盗链（Referer/Cookie 校验）。需要利用声明式 JSON 规则与沙盒环境动态解析，避免硬编码。
2. **高并发读请求与慢速采集的矛盾**：上游采集通常耗时 500ms 至 3000ms，不能阻塞主 Web 服务的事件循环，且必须防止同一热门章节被并发请求击穿导致上游封禁 IP。
3. **高昂的漫画带宽与内存开销**：高清条漫单章体积达数十 MB。网关在实时代理模式下严禁将整张图片读入内存或落盘后再转发，必须采用固定缓冲区的流式透传（Stream Pipe）边解密边返回。
4. **单点源站失效问题**：单一采集源极易宕机或改版，一部作品需绑定多个外部源站，根据源站实时健康度评分自动降级切换。

## 2. 核心架构模式：控制流与数据流分离 (Go + Python 协同)
采用 Monorepo 结构，将系统拆分为两个物理隔离的服务，通过 Redis 6+ 与 PostgreSQL 16 协同：
- **`gateway-go` (Go 1.23 + Fiber v2)**：主网关与业务 API。负责用户鉴权、书架与阅读进度 CRUD、利用 `singleflight` 合并并发章节请求、通过 Redis 队列调度实时采集任务、提供小说 SSE 流式输出以及漫画图片 32KB 缓冲区的流式防盗链解密反向代理。
- **`scraper-py` (Python 3.12 + uv + asyncio)**：纯后台无状态采集 Worker（不含任何 HTTP Web 框架）。负责监听 Redis 高低优先级队列，使用 `curl_cffi` 模拟浏览器 TLS 指纹请求源站，通过 `lxml`/`cssselect` 提取 DOM，使用 `quickjs` 沙盒执行动态解密脚本，将解析出的小说正文流或漫画图片访问凭证写回 Redis。

## 3. Monorepo 目录结构规范
```text
manga-novel-platform/
├── SPEC.md
├── docker-compose.yml
├── migrations/
│   └── 001_init.sql                # PostgreSQL 初始化建表文件
├── gateway-go/
│   ├── cmd/api/main.go             # 唯一入口：配置加载、依赖注入组装(DI)、优雅停机
│   ├── internal/
│   │   ├── config/                 # 环境变量配置解析
│   │   ├── domain/                 # 核心实体、统一错误 AppError、Repository/Service 接口定义
│   │   ├── repository/             # Postgres (pgx/v5) 与 Redis (go-redis/v9) 接口实现
│   │   ├── service/                # 业务逻辑：Singleflight 合并、多源健康度路由、RPC 编排
│   │   ├── handler/                # Fiber 控制器：参数校验、SSE 流、图片流式代理
│   │   └── middleware/             # 全局 ErrorHandler、RequestID 链路追踪与 slog 日志
│   ├── go.mod
│   └── Dockerfile
└── scraper-py/
    ├── app/
    │   ├── main.py                 # 异步 Worker 消费主循环 (支持并发协程池)
    │   ├── cli.py                  # 命令行独立调试工具 (脱离 Redis 测试单个源站规则)
    │   ├── config.py               # Redis 与数据库连接配置
    │   ├── engine/
    │   │   ├── fetcher.py          # curl_cffi TLS 伪装请求器与重试逻辑
    │   │   ├── parser.py           # 基于 JSON 规则的 XPath/CSS/Regex 提取器
    │   │   └── sandbox.py          # QuickJS 内存沙盒执行器 (解密混淆 JS)
    │   └── rules/                  # 存放各源站 JSON 采集规则示例
    ├── pyproject.toml
    └── Dockerfile
```

## 4. PostgreSQL 核心数据库表结构 (`migrations/001_init.sql`)
```sql
CREATE TYPE media_type_enum AS ENUM ('novel', 'manga');
CREATE TYPE work_status_enum AS ENUM ('ongoing', 'completed', 'hiatus');

-- 1. 平台标准作品主表
CREATE TABLE works (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(255) NOT NULL,
    author VARCHAR(128) NOT NULL DEFAULT 'Unknown',
    media_type media_type_enum NOT NULL,
    cover_url TEXT,
    description TEXT,
    status work_status_enum NOT NULL DEFAULT 'ongoing',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (title, author, media_type)
);

-- 2. 外部采集源站与动态 JSON 规则表
CREATE TABLE scraper_sources (
    source_id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    media_type media_type_enum NOT NULL,
    base_url TEXT NOT NULL,
    rule_config JSONB NOT NULL,
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 3. 作品与外部源站多对多映射表 (支撑多源聚合与换源)
CREATE TABLE work_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    work_id UUID NOT NULL REFERENCES works(id) ON DELETE CASCADE,
    source_id VARCHAR(64) NOT NULL REFERENCES scraper_sources(source_id) ON DELETE CASCADE,
    remote_work_url TEXT NOT NULL,
    latest_chapter_title VARCHAR(255),
    last_synced_at TIMESTAMPTZ,
    UNIQUE (work_id, source_id)
);

-- 4. 章节索引表 (storage_key 非空代表已持久化至 S3/R2，为空代表需走实时采集)
CREATE TABLE chapters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    work_source_id UUID NOT NULL REFERENCES work_sources(id) ON DELETE CASCADE,
    chapter_num DOUBLE PRECISION NOT NULL,
    title VARCHAR(255) NOT NULL,
    remote_chapter_url TEXT NOT NULL,
    storage_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (work_source_id, chapter_num)
);
CREATE INDEX idx_chapters_work_source_num ON chapters(work_source_id, chapter_num ASC);

-- 5. 用户阅读进度表
CREATE TABLE reading_progress (
    user_id UUID NOT NULL,
    work_id UUID NOT NULL REFERENCES works(id) ON DELETE CASCADE,
    chapter_id UUID NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
    position_offset INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, work_id)
);
```

## 5. 跨语言 Redis 通信协议与键名契约
Go 与 Python 之间严禁使用非结构化字符串通信，必须严格遵守以下 Redis Key 命名与 JSON 格式：

1. **任务投递队列 (List)**：
   - 实时高优队列：`queue:scrape:realtime` (Go 执行 `LPUSH`，Python 执行 `BRPOP` 优先消费)
   - 定时低优队列：`queue:scrape:cron`
   - 任务载荷 JSON 结构：
     ```json
     {
       "task_id": "uuid-v4-string",
       "media_type": "novel | manga",
       "source_id": "example_source",
       "chapter_id": "uuid-of-chapter",
       "remote_url": "https://target-site.com/read/1001.html",
       "timeout_ms": 8000
     }
     ```

2. **流式回调与通知通道 (List)**：
   - Key：`stream:{task_id}` (Python 写入后必须设置 `EXPIRE 60` 秒)
   - **小说模式**：Python 每解析完 5 个段落，立即 `RPUSH` 纯文本块到 `stream:{task_id}`；全部解析完成后 `RPUSH` 字符串 `"[EOF]"`，若发生异常则 `RPUSH` 字符串 `"[ERROR]:具体原因"`。Go 端循环执行 `BLPOP` 并通过 SSE 实时推送给前端。
   - **漫画模式**：Python 解析出章节内所有图片地址后，为每张图片生成 16 位 MD5 Ticket 并写入 `proxy:ticket:{ticket}`，随后将完整的章节图片代理 URL 数组写入 `cache:chapter:{chapter_id}`，最后向 `stream:{task_id}` 推入 `"READY"` 唤醒正在等待的 Go 协程。

3. **章节结果缓存 (String)**：
   - Key：`cache:chapter:{chapter_id}` (TTL: 86400 秒)
   - 小说存储清洗后的完整正文文本；漫画存储 JSON 字符串：`{"chapter_id": "...", "images": ["/api/v1/proxy/image/a1b2c3d4..."]}`。

4. **漫画单图防盗链代理凭证 (Hash)**：
   - Key：`proxy:ticket:{ticket}` (TTL: 7200 秒)
   - Hash 字段：`url` (源站真实图片地址), `referer` (源站章节页地址), `ua` (抓取时使用的 User-Agent), `xor_key` (可选的单字节十进制异或密钥字符串，无加密则为空字符串)。

5. **源站健康度评分池 (ZSET)**：
   - Key：`health:sources:{media_type}`
   - Member：`source_id`，Score：浮点数健康分（初始 100，采集成功 +1 最大 100，超时或失败 -20；低于 20 分触发熔断）。

## 6. 编码红线与工业级约束（AI 生成代码时必须严格执行）
1. **Go 严禁全局变量**：`gateway-go` 中严禁出现包级全局数据库连接或 Redis 客户端变量。所有依赖必须通过构造函数 `NewXxx(...)` 显式注入，在 `cmd/api/main.go` 中完成组装。
2. **Go 架构分层铁律**：
   - `handler` 层只做 HTTP 参数解析与响应写入，遇到错误直接 `return err`，交由全局 `ErrorHandler` 中间件统一转换为 HTTP 状态码。
   - `service` 层必须使用 `golang.org/x/sync/singleflight` 合并针对同一 `chapter_id` 的并发采集请求。
   - 所有跨层函数调用第一个参数必须为 `ctx context.Context`。
   - `main.go` 必须实现监听 `SIGINT`/`SIGTERM` 信号的优雅停机（Graceful Shutdown）。
3. **Go 流式代理零落盘**：漫画图片代理接口必须使用 `c.Context().SetBodyStreamWriter`，分配固定 `32 * 1024` 字节缓冲区循环读取上游 `resp.Body`，在内存中完成可选的字节异或解密后立即 `Flush` 到客户端，严禁使用 `io.ReadAll` 将整张图片读入内存。
4. **Python 全异步与异常兜底**：`scraper-py` 必须全程使用 `redis.asyncio` 与 `curl_cffi.requests.AsyncSession`。任何采集异常必须在 `try...except` 中捕获，并向对应的 `stream:{task_id}` 写入 `"[ERROR]"` 或 `"[EOF]"`，绝不能让 Go 端的 `BLPOP` 无限期挂起直到超时。
5. **禁止留白与占位符**：生成的代码必须包含完整的 import、结构体定义、错误处理与边界检查，禁止使用 `// TODO: implement later` 省略核心逻辑。
