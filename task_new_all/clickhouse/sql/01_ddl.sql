-- ============================================================
-- БАЗА ДАННЫХ ДЛЯ НОВОСТНОГО АГРЕГАТОРА
-- ============================================================
CREATE DATABASE IF NOT EXISTS news;

-- ============================================================
-- ТАБЛИЦА СОБЫТИЙ (аналог transport_events)
-- ============================================================
CREATE TABLE IF NOT EXISTS news.news_events
(
    -- Системные поля
    event_id     UUID,
    event_time   DateTime,
    event_type   LowCardinality(String),   -- NewsPublished, NewsViewed, NewsLiked, NewsShared, CommentAdded
    ingested_at  DateTime DEFAULT now(),

    -- Идентификаторы сущностей
    news_id      UUID,
    user_id      String,
    author_id    UInt32,
    channel_id   UInt32,
    comment_id   UInt64,

    -- Денормализованные данные (для быстрых запросов)
    title        String,
    category     LowCardinality(String),
    tags         Array(String),
    author_name  String,
    author_type  LowCardinality(String),   -- journalist, agency, blogger

    -- Поля для разных типов событий
    read_duration_sec UInt32,              -- для NewsViewed
    like_value        Int8,                -- для NewsLiked (1 или -1)
    platform          LowCardinality(String), -- для NewsShared
    shared_to_user_id String,              -- для NewsShared
    comment_text      String               -- для CommentAdded
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(event_time)
ORDER BY (event_type, event_time, news_id)
SETTINGS index_granularity = 8192;