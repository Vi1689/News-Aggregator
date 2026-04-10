-- ============================================================
-- МЕТРИКА 1: Общая статистика системы
-- ============================================================
SELECT
    uniq(news_id) AS total_news,
    uniq(user_id) AS total_users,
    uniq(author_name) AS total_authors,
    count() AS total_events
FROM news.news_events;

-- ============================================================
-- МЕТРИКА 2: Среднее количество просмотров на новость
-- ============================================================
SELECT
    round(countIf(event_type = 'NewsViewed') * 1.0 / uniq(news_id), 1) AS avg_views_per_news
FROM news.news_events;

-- ============================================================
-- МЕТРИКА 3: Самый популярный час для чтения
-- ============================================================
SELECT
    toHour(event_time) AS hour,
    count() AS views
FROM news.news_events
WHERE event_type = 'NewsViewed'
GROUP BY hour
ORDER BY views DESC
LIMIT 1;

-- ============================================================
-- МЕТРИКА 4: Категория с наибольшей вовлечённостью
-- ============================================================
SELECT
    category,
    round((countIf(event_type = 'NewsLiked') + countIf(event_type = 'NewsShared')) * 100.0 / countIf(event_type = 'NewsViewed'), 2) AS engagement_rate
FROM news.news_events
GROUP BY category
ORDER BY engagement_rate DESC
LIMIT 1;

-- ============================================================
-- МЕТРИКА 5: Лучший автор по лайкам на публикацию
-- ============================================================
SELECT
    author_name,
    round(countIf(event_type = 'NewsLiked') * 1.0 / countIf(event_type = 'NewsPublished'), 2) AS likes_per_article
FROM news.news_events
GROUP BY author_name
HAVING countIf(event_type = 'NewsPublished') > 0
ORDER BY likes_per_article DESC
LIMIT 5;