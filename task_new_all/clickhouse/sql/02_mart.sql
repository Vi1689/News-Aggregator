-- ============================================================
-- ВИТРИНА: часовая статистика по категориям и авторам
-- ============================================================
CREATE TABLE IF NOT EXISTS news.mart_category_hourly
(
    category        LowCardinality(String),
    hour            DateTime,
    published_count UInt32,
    viewed_count    UInt32,
    liked_count     UInt32,
    shared_count    UInt32,
    comment_count   UInt32,
    unique_users    UInt32,
    avg_read_time   Float32
)
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(hour)
ORDER BY (category, hour);

-- ============================================================
-- MATERIALIZED VIEW — автоматически заполняет витрину
-- ============================================================
CREATE MATERIALIZED VIEW IF NOT EXISTS news.mv_category_hourly
TO news.mart_category_hourly
AS
SELECT
    category,
    toStartOfHour(event_time) AS hour,
    countIf(event_type = 'NewsPublished') AS published_count,
    countIf(event_type = 'NewsViewed')    AS viewed_count,
    countIf(event_type = 'NewsLiked')     AS liked_count,
    countIf(event_type = 'NewsShared')    AS shared_count,
    countIf(event_type = 'CommentAdded')  AS comment_count,
    uniqIf(user_id, event_type != 'NewsPublished') AS unique_users,
    avgIf(read_duration_sec, event_type = 'NewsViewed') AS avg_read_time
FROM news.news_events
GROUP BY category, hour;