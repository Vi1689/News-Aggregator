-- ============================================================
-- 1. Количество публикаций по дням
-- Бизнес-задача: отслеживать активность редакции
-- ============================================================
SELECT
    toDate(event_time) AS day,
    count() AS publications
FROM news.news_events
WHERE event_type = 'NewsPublished'
  AND event_time >= now() - INTERVAL 8 WEEK
GROUP BY day
ORDER BY day;

-- ============================================================
-- 2. Топ-10 авторов по просмотрам
-- Бизнес-задача: выявить самых читаемых авторов
-- ============================================================
SELECT
    author_name,
    author_type,
    countIf(event_type = 'NewsViewed') AS total_views,
    count(DISTINCT news_id) AS articles_published
FROM news.news_events
WHERE event_type IN ('NewsPublished', 'NewsViewed')
  AND event_time >= now() - INTERVAL 8 WEEK
GROUP BY author_name, author_type
ORDER BY total_views DESC
LIMIT 10;

-- ============================================================
-- 3. Среднее время чтения по категориям
-- Бизнес-задача: понять, какие темы читают дольше всего
-- ============================================================
SELECT
    category,
    round(avg(read_duration_sec), 1) AS avg_read_seconds,
    count() AS views_count
FROM news.news_events
WHERE event_type = 'NewsViewed'
  AND read_duration_sec > 0
GROUP BY category
ORDER BY avg_read_seconds DESC;

-- ============================================================
-- 4. Лайки/дизлайки по авторам
-- Бизнес-задача: оценить качество контента авторов
-- ============================================================
SELECT
    author_name,
    sumIf(like_value, like_value = 1) AS likes,
    sumIf(like_value, like_value = -1) AS dislikes,
    round(likes * 100.0 / (likes + dislikes), 2) AS like_rate_pct
FROM news.news_events
WHERE event_type = 'NewsLiked'
GROUP BY author_name
HAVING likes + dislikes > 0
ORDER BY like_rate_pct DESC
LIMIT 20;

-- ============================================================
-- 5. Активность по часам суток
-- Бизнес-задача: определить пиковые часы чтения новостей
-- ============================================================
SELECT
    toHour(event_time) AS hour,
    countIf(event_type = 'NewsViewed') AS views,
    countIf(event_type = 'NewsLiked') AS likes,
    countIf(event_type = 'NewsShared') AS shares,
    uniq(user_id) AS active_users
FROM news.news_events
WHERE event_time >= now() - INTERVAL 8 WEEK
GROUP BY hour
ORDER BY hour;

-- ============================================================
-- 6. Популярность тегов за последнюю неделю
-- Бизнес-задача: отслеживать тренды
-- ============================================================
SELECT
    tag,
    count() AS mentions,
    uniq(news_id) AS unique_news
FROM (
    SELECT arrayJoin(tags) AS tag, news_id
    FROM news.news_events
    WHERE event_type = 'NewsPublished'
      AND event_time >= now() - INTERVAL 1 WEEK
)
GROUP BY tag
ORDER BY mentions DESC
LIMIT 20;

-- ============================================================
-- 7. Вовлечённость пользователей
-- Бизнес-задача: найти самых активных читателей
-- ============================================================
SELECT
    user_id,
    countIf(event_type = 'NewsViewed') AS views,
    countIf(event_type = 'NewsLiked') AS likes,
    countIf(event_type = 'NewsShared') AS shares,
    countIf(event_type = 'CommentAdded') AS comments,
    avgIf(read_duration_sec, event_type = 'NewsViewed') AS avg_read_time
FROM news.news_events
WHERE user_id != '' AND user_id NOT LIKE 'demo%'
GROUP BY user_id
ORDER BY views DESC
LIMIT 20;

-- ============================================================
-- 8. Процент вовлечённости по категориям
-- Бизнес-задача: определить, какие категории вызывают больше реакций
-- ============================================================
SELECT
    category,
    countIf(event_type = 'NewsViewed') AS views,
    countIf(event_type IN ('NewsLiked', 'NewsShared', 'CommentAdded')) AS engagements,
    round(engagements * 100.0 / views, 2) AS engagement_rate_pct
FROM news.news_events
GROUP BY category
HAVING views > 100
ORDER BY engagement_rate_pct DESC;