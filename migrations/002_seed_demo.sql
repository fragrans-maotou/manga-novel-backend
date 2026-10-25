-- Demo seed for local mock-source (idempotent upserts).
-- Fixed UUIDs are documented in README for curl examples.

INSERT INTO scraper_sources (source_id, name, media_type, base_url, rule_config, is_enabled, updated_at)
VALUES
  (
    'demo_novel',
    'Demo Novel (mock-source)',
    'novel',
    'http://mock-source:8090/',
    '{"source_id":"demo_novel","media_type":"novel","novel":{"paragraph_xpath":"//div[@id=''content'']//p/text()"}}'::jsonb,
    TRUE,
    NOW()
  ),
  (
    'demo_manga_js',
    'Demo Manga JS (mock-source)',
    'manga',
    'http://mock-source:8090/',
    '{"source_id":"demo_manga_js","media_type":"manga","manga":{"xor_key":"12","image_base_url":"http://mock-source:8090/manga/img/","js_extract":"b64json"}}'::jsonb,
    TRUE,
    NOW()
  )
ON CONFLICT (source_id) DO UPDATE SET
  name = EXCLUDED.name,
  media_type = EXCLUDED.media_type,
  base_url = EXCLUDED.base_url,
  rule_config = EXCLUDED.rule_config,
  is_enabled = EXCLUDED.is_enabled,
  updated_at = NOW();

INSERT INTO works (id, title, author, media_type, cover_url, description, status, created_at, updated_at)
VALUES
  (
    '11111111-1111-1111-1111-111111111101',
    '晨光小镇（演示小说）',
    'Demo Author',
    'novel',
    NULL,
    '本地 mock-source 演示用小说，非真实作品。',
    'ongoing',
    NOW(),
    NOW()
  ),
  (
    '11111111-1111-1111-1111-111111111102',
    '沙盒条漫（演示漫画）',
    'Demo Artist',
    'manga',
    NULL,
    '本地 mock-source + QuickJS 演示用漫画，非真实作品。',
    'ongoing',
    NOW(),
    NOW()
  )
ON CONFLICT (id) DO UPDATE SET
  title = EXCLUDED.title,
  author = EXCLUDED.author,
  media_type = EXCLUDED.media_type,
  description = EXCLUDED.description,
  status = EXCLUDED.status,
  updated_at = NOW();

INSERT INTO work_sources (id, work_id, source_id, remote_work_url, latest_chapter_title, last_synced_at)
VALUES
  (
    '22222222-2222-2222-2222-222222222201',
    '11111111-1111-1111-1111-111111111101',
    'demo_novel',
    'http://mock-source:8090/novel/chapter-1.html',
    '第一章 晨光',
    NOW()
  ),
  (
    '22222222-2222-2222-2222-222222222202',
    '11111111-1111-1111-1111-111111111102',
    'demo_manga_js',
    'http://mock-source:8090/manga/chapter-1.html',
    '第1话 沙盒解混淆演示',
    NOW()
  )
ON CONFLICT (id) DO UPDATE SET
  remote_work_url = EXCLUDED.remote_work_url,
  latest_chapter_title = EXCLUDED.latest_chapter_title,
  last_synced_at = EXCLUDED.last_synced_at;

INSERT INTO chapters (id, work_source_id, chapter_num, title, remote_chapter_url, storage_key, created_at)
VALUES
  (
    '33333333-3333-3333-3333-333333333301',
    '22222222-2222-2222-2222-222222222201',
    1,
    '第一章 晨光',
    'http://mock-source:8090/novel/chapter-1.html',
    NULL,
    NOW()
  ),
  (
    '33333333-3333-3333-3333-333333333302',
    '22222222-2222-2222-2222-222222222202',
    1,
    '第1话 沙盒解混淆演示',
    'http://mock-source:8090/manga/chapter-1.html',
    NULL,
    NOW()
  )
ON CONFLICT (id) DO UPDATE SET
  title = EXCLUDED.title,
  remote_chapter_url = EXCLUDED.remote_chapter_url,
  storage_key = NULL;
