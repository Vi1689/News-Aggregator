-- ==========================
-- АГРЕГИРУЮЩИЕ ЗАПРОСЫ
-- ==========================

-- 1. Статистика активности каналов (посты + комментарии)
CREATE VIEW channel_activity_stats AS
SELECT 
    c.channel_id,
    c.name AS channel_name,
    COUNT(DISTINCT p.post_id) AS posts_count,
    COUNT(DISTINCT cm.comment_id) AS total_comments,
    COALESCE(AVG(p.likes_count), 0) AS avg_likes_per_post
FROM channels c
LEFT JOIN posts p ON c.channel_id = p.channel_id
LEFT JOIN comments cm ON p.post_id = cm.post_id
GROUP BY c.channel_id, c.name;

-- 2. Анализ авторов (полная статистика)
CREATE VIEW author_performance AS
SELECT 
    a.author_id,
    a.name AS author_name,
    COUNT(p.post_id) AS total_posts,
    SUM(p.likes_count) AS total_likes,
    SUM(p.comments_count) AS total_comments,
    COALESCE(AVG(p.likes_count), 0) AS avg_likes_per_post
FROM authors a
LEFT JOIN posts p ON a.author_id = p.author_id
GROUP BY a.author_id, a.name;

-- 3. Популярность тегов с дополнительной статистикой
CREATE VIEW tag_popularity_detailed AS
SELECT 
    t.tag_id,
    t.name AS tag_name,
    COUNT(pt.post_id) AS usage_count,
    COUNT(DISTINCT p.channel_id) AS channels_using_tag,
    COALESCE(AVG(p.likes_count), 0) AS avg_likes
FROM tags t
LEFT JOIN post_tags pt ON t.tag_id = pt.tag_id
LEFT JOIN posts p ON pt.post_id = p.post_id
GROUP BY t.tag_id, t.name;

-- 4. Статистика постов по источникам
CREATE VIEW source_post_stats AS
SELECT 
    s.source_id,
    s.name AS source_name,
    COUNT(p.post_id) AS total_posts,
    COUNT(DISTINCT p.author_id) AS unique_authors,
    SUM(p.likes_count) AS total_likes,
    SUM(p.comments_count) AS total_comments
FROM sources s
LEFT JOIN channels c ON s.source_id = c.source_id
LEFT JOIN posts p ON c.channel_id = p.channel_id
GROUP BY s.source_id, s.name;

-- 5. Активность пользователей по комментариям
CREATE VIEW user_comment_activity AS
SELECT 
    nickname,
    COUNT(*) AS comments_count,
    COUNT(DISTINCT post_id) AS posts_commented,
    SUM(likes_count) AS total_comment_likes,
    AVG(likes_count) AS avg_likes_per_comment
FROM comments
GROUP BY nickname;

-- ==========================
-- ОКОННЫЕ ФУНКЦИИ
-- ==========================

-- 1. Ранжирование постов по популярности внутри каждого канала
CREATE VIEW posts_ranked_by_popularity AS
SELECT 
    p.post_id,
    p.title,
    c.name AS channel_name,
    p.likes_count,
    p.comments_count,
    RANK() OVER (PARTITION BY p.channel_id ORDER BY p.likes_count DESC) AS popularity_rank_in_channel,
    PERCENT_RANK() OVER (PARTITION BY p.channel_id ORDER BY p.likes_count) AS popularity_percentile
FROM posts p
JOIN channels c ON p.channel_id = c.channel_id;

-- 2. Скользящее среднее лайков для авторов
CREATE VIEW author_likes_trend AS
SELECT 
    post_id,
    author_id,
    title,
    likes_count,
    created_at,
    AVG(likes_count) OVER (
        PARTITION BY author_id 
        ORDER BY created_at 
        ROWS BETWEEN 2 PRECEDING AND CURRENT ROW
    ) AS moving_avg_likes
FROM posts;

-- 3. Кумулятивная статистика постов по датам
CREATE VIEW cumulative_posts_analysis AS
SELECT 
    post_id,
    channel_id,
    created_at::DATE AS post_date,
    COUNT(*) OVER (
        PARTITION BY channel_id 
        ORDER BY created_at::DATE
    ) AS cumulative_posts,
    SUM(likes_count) OVER (
        PARTITION BY channel_id 
        ORDER BY created_at::DATE
    ) AS cumulative_likes
FROM posts;

-- 4. Ранжирование тегов по популярности в каждом канале
CREATE VIEW tag_rank_by_channel AS
SELECT 
    c.channel_id,
    c.name AS channel_name,
    t.tag_id,
    t.name AS tag_name,
    COUNT(pt.post_id) AS usage_count,
    RANK() OVER (PARTITION BY c.channel_id ORDER BY COUNT(pt.post_id) DESC) AS tag_rank_in_channel
FROM channels c
JOIN posts p ON c.channel_id = p.channel_id
JOIN post_tags pt ON p.post_id = pt.post_id
JOIN tags t ON pt.tag_id = t.tag_id
GROUP BY c.channel_id, c.name, t.tag_id, t.name;

-- 5. Анализ комментаторской активности с оконными функциями
CREATE VIEW commenter_analysis AS
SELECT 
    nickname,
    post_id,
    created_at,
    likes_count,
    COUNT(*) OVER (PARTITION BY nickname) AS total_comments_by_user,
    AVG(likes_count) OVER (PARTITION BY nickname) AS avg_likes_per_comment,
    LAG(created_at) OVER (PARTITION BY nickname ORDER BY created_at) AS previous_comment_time
FROM comments;

-- ==========================
-- JOIN ЗАПРОСЫ
-- ==========================

-- 1. Посты с полной информацией об авторах
CREATE VIEW posts_with_detailed_authors AS
SELECT 
    p.post_id,
    p.title,
    p.created_at,
    p.likes_count,
    p.comments_count,
    a.name AS author_name
FROM posts p
JOIN authors a ON p.author_id = a.author_id;

-- 2. Каналы с информацией об источниках
CREATE VIEW channels_with_sources AS
SELECT 
    c.channel_id,
    c.name AS channel_name,
    c.subscribers_count,
    s.name AS source_name,
    s.address AS source_url
FROM channels c
JOIN sources s ON c.source_id = s.source_id;

-- 3. Посты с авторами и текстами
CREATE VIEW posts_with_authors_and_texts AS
SELECT 
    p.post_id,
    p.title,
    a.name AS author_name,
    nt.text AS content,
    p.created_at
FROM posts p
JOIN authors a ON p.author_id = a.author_id
JOIN news_texts nt ON p.text_id = nt.text_id;

-- 4. Комментарии с информацией о посте и авторе комментария
CREATE VIEW comments_with_post_info AS
SELECT 
    c.comment_id,
    c.nickname,
    c.text AS comment_text,
    c.likes_count AS comment_likes,
    p.title AS post_title,
    a.name AS post_author
FROM comments c
JOIN posts p ON c.post_id = p.post_id
JOIN authors a ON p.author_id = a.author_id;

-- 5. Посты с тегами и каналами
CREATE VIEW posts_with_tags_and_channels AS
SELECT 
    p.post_id,
    p.title,
    t.name AS tag_name,
    c.name AS channel_name,
    p.created_at
FROM posts p
JOIN post_tags pt ON p.post_id = pt.post_id
JOIN tags t ON pt.tag_id = t.tag_id
JOIN channels c ON p.channel_id = c.channel_id;

-- 6. Медиа с информацией о посте и канале
CREATE VIEW media_with_context AS
SELECT 
    m.media_id,
    m.media_content,
    m.media_type,
    p.title AS post_title,
    c.name AS channel_name
FROM media m
JOIN posts p ON m.post_id = p.post_id
JOIN channels c ON p.channel_id = c.channel_id;

-- 7. Полная информация о постах (автор, канал, теги, текст)
CREATE VIEW comprehensive_post_info AS
SELECT 
    p.post_id,
    p.title,
    a.name AS author_name,
    c.name AS channel_name,
    s.name AS source_name,
    nt.text AS content,
    p.likes_count,
    p.comments_count,
    p.created_at
FROM posts p
JOIN authors a ON p.author_id = a.author_id
JOIN channels c ON p.channel_id = c.channel_id
JOIN sources s ON c.source_id = s.source_id
JOIN news_texts nt ON p.text_id = nt.text_id;

-- 8. Расширенная аналитика постов (все связи)
CREATE VIEW extended_post_analytics AS
SELECT 
    p.post_id,
    p.title,
    a.name AS author_name,
    c.name AS channel_name,
    s.name AS source_name,
    s.topic AS source_topic,
    STRING_AGG(DISTINCT t.name, ', ') AS tags,
    COUNT(DISTINCT m.media_id) AS media_count,
    p.likes_count,
    p.comments_count,
    nt.text AS content_preview
FROM posts p
JOIN authors a ON p.author_id = a.author_id
JOIN channels c ON p.channel_id = c.channel_id
JOIN sources s ON c.source_id = s.source_id
JOIN news_texts nt ON p.text_id = nt.text_id
LEFT JOIN post_tags pt ON p.post_id = pt.post_id
LEFT JOIN tags t ON pt.tag_id = t.tag_id
LEFT JOIN media m ON p.post_id = m.post_id
GROUP BY 
    p.post_id, p.title, a.name, c.name, s.name, s.topic, 
    p.likes_count, p.comments_count, nt.text;