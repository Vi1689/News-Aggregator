-- ============================================================
-- ТАБЛИЦА С ДЕДУПЛИКАЦИЕЙ (ReplacingMergeTree)
-- Сценарий: одно событие может прийти дважды (сбой сети)
-- ============================================================
CREATE TABLE IF NOT EXISTS news.news_events_dedup
(
    event_id     UUID,
    event_time   DateTime,
    event_type   LowCardinality(String),
    ingested_at  DateTime DEFAULT now(),
    news_id      UUID,
    user_id      String,
    title        String,
    category     LowCardinality(String),
    tags         Array(String)
)
ENGINE = ReplacingMergeTree(ingested_at)
PARTITION BY toYYYYMM(event_time)
ORDER BY (event_type, event_time, news_id, event_id);

-- Тестовые дубликаты
INSERT INTO news.news_events_dedup VALUES
    ('11111111-1111-1111-1111-111111111111', now(), 'NewsViewed', now(), 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'user_001', 'Квантовый прорыв', 'technology', ['ai', 'science']),
    ('11111111-1111-1111-1111-111111111111', now(), 'NewsViewed', now(), 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'user_001', 'Квантовый прорыв', 'technology', ['ai', 'science']),
    ('22222222-2222-2222-2222-222222222222', now(), 'NewsLiked', now(), 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'user_002', 'Спортивные новости', 'sports', ['football']);