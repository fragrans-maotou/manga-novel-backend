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
