-- Создание базы данных
CREATE DATABASE IF NOT EXISTS newsdb;

-- Использование базы
USE newsdb;

-- ============================================
-- ТАБЛИЦА СЫРЫХ СОБЫТИЙ (с TTL)
-- ============================================
CREATE TABLE IF NOT EXISTS news_events_raw
(
    -- Системные поля
    eventId String,
    eventType String,
    entityId String,
    timestamp DateTime,   
    source String,
    version UInt8,
    ingestionTime DateTime DEFAULT now(),
    
    -- Данные новости (денормализованы)
    newsId String,
    title String,
    summary String,
    fullText String,
    author String,
    authorType String,
    categories Array(String),
    tags Array(String),
    published_at DateTime,   
    
    -- Данные пользователя
    userId String,
    
    -- Поля для конкретных событий
    readDurationSec UInt32,
    likeValue Int8,
    platform String,
    sharedToUserId String,
    commentText String,
    parentCommentId String
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (eventType, timestamp, newsId)
TTL timestamp + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

-- ============================================
-- МАТЕРИАЛИЗОВАННАЯ ВИДРИНА 1: Дневная статистика
-- ============================================
CREATE MATERIALIZED VIEW IF NOT EXISTS news_daily_stats
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (date, newsId)
AS SELECT
    toDate(timestamp) AS date,
    newsId,
    any(title) AS title,
    countIf(eventType = 'NewsViewed') AS views,
    countIf(eventType = 'NewsLiked') AS likes,
    uniqIf(userId, eventType = 'NewsViewed') AS unique_viewers,
    avgIf(readDurationSec, eventType = 'NewsViewed') AS avg_read_duration
FROM news_events_raw
WHERE eventType IN ('NewsViewed', 'NewsLiked')
GROUP BY date, newsId;

-- ============================================
-- МАТЕРИАЛИЗОВАННАЯ ВИДРИНА 2: Популярность тегов по часам
-- ============================================
CREATE MATERIALIZED VIEW IF NOT EXISTS tag_hourly_popularity
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(hour)
ORDER BY (hour, tag)
AS SELECT
    toStartOfHour(timestamp) AS hour,
    arrayJoin(tags) AS tag,
    count() AS mentions_count,
    uniq(newsId) AS unique_news
FROM news_events_raw
WHERE eventType = 'NewsPublished' AND length(tags) > 0
GROUP BY hour, tag;

-- ============================================
-- ТАБЛИЦА ДЛЯ ДЕДУПЛИКАЦИИ (ReplacingMergeTree)
-- ============================================
CREATE TABLE IF NOT EXISTS news_events_dedup
(
    eventId String,
    eventType String,
    timestamp DateTime,
    payload String,
    version UInt8,
    _is_deleted UInt8 DEFAULT 0
)
ENGINE = ReplacingMergeTree(version)
ORDER BY (eventId, timestamp);