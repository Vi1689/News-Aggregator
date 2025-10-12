-- ==========================
-- АГРЕГИРУЮЩИЕ ЗАПРОСЫ
-- ==========================

-- Счёт постов каждого автора
CREATE VIEW top_authors AS
SELECT author_id, COUNT(*) AS posts_count
FROM posts
GROUP BY author_id;

-- Счёт комментариев каждого пользователя (по nickname)
CREATE VIEW active_users AS
SELECT nickname, COUNT(*) AS comments_count
FROM comments
GROUP BY nickname;

-- Счёт использования тегов
CREATE VIEW popular_tags AS
SELECT tag_id, COUNT(*) AS usage_count
FROM post_tags
GROUP BY tag_id;

-- Счёт постов по каналам
CREATE VIEW posts_by_channel AS
SELECT channel_id, COUNT(*) AS posts_count
FROM posts
GROUP BY channel_id;

-- Среднее количество комментариев на пост
CREATE VIEW avg_comments_per_post AS
SELECT post_id, COUNT(*) AS comments_count
FROM comments
GROUP BY post_id;

-- ==========================
-- ОКОННЫЕ ФУНКЦИИ
-- ==========================

-- Ранжирование постов по дате внутри каждого автора
CREATE VIEW posts_ranked AS
SELECT post_id, author_id, title,
       ROW_NUMBER() OVER (PARTITION BY author_id ORDER BY created_at DESC) AS rank_recent,
       COUNT(*) OVER (PARTITION BY author_id) AS total_posts
FROM posts;

-- Скользящее среднее комментариев на пост (по nickname)
CREATE VIEW comments_moving_avg AS
SELECT post_id, nickname, created_at,
       AVG(COUNT(*)) OVER (PARTITION BY post_id ORDER BY created_at ROWS BETWEEN 2 PRECEDING AND CURRENT ROW) AS moving_avg_comments
FROM comments
GROUP BY post_id, nickname, created_at;

-- Кумулятивное количество постов на автора
CREATE VIEW cumulative_posts AS
SELECT post_id, author_id, created_at,
       SUM(1) OVER (PARTITION BY author_id ORDER BY created_at) AS cumulative_count
FROM posts;

-- Ранжирование тегов по популярности
CREATE VIEW tag_rank AS
SELECT tag_id, COUNT(*) AS usage_count,
       RANK() OVER (ORDER BY COUNT(*) DESC) AS rank_usage
FROM post_tags
GROUP BY tag_id;

-- Ранжирование пользователей по активности (по nickname)
CREATE VIEW user_activity_rank AS
SELECT nickname, COUNT(*) AS comments_count,
       DENSE_RANK() OVER (ORDER BY COUNT(*) DESC) AS activity_rank
FROM comments
GROUP BY nickname;

-- ==========================
-- JOIN ЗАПРОСЫ
-- ==========================

-- Посты с авторами
CREATE VIEW posts_with_authors AS
SELECT p.post_id, p.title, a.name AS author
FROM posts p
JOIN authors a ON p.author_id = a.author_id;

-- Комментарии с пользователями (по nickname)
CREATE VIEW comments_with_users AS
SELECT c.comment_id, c.post_id, c.nickname AS user_name, c.text
FROM comments c;

-- Посты с тегами
CREATE VIEW posts_with_tags AS
SELECT p.post_id, p.title, t.name AS tag_name
FROM posts p
JOIN post_tags pt ON p.post_id = pt.post_id
JOIN tags t ON pt.tag_id = t.tag_id;

-- Посты с авторами и каналами
CREATE VIEW posts_authors_channels AS
SELECT p.post_id, p.title, a.name AS author, ch.name AS channel_name
FROM posts p
JOIN authors a ON p.author_id = a.author_id
JOIN channels ch ON p.channel_id = ch.channel_id;

-- Комментарии с постами и пользователями
CREATE VIEW comments_posts_users AS
SELECT c.comment_id, c.text, p.title AS post_title, c.nickname AS user_name
FROM comments c
JOIN posts p ON c.post_id = p.post_id;

-- Посты с авторами и тегами
CREATE VIEW posts_authors_tags AS
SELECT p.post_id, p.title, a.name AS author, t.name AS tag_name
FROM posts p
JOIN authors a ON p.author_id = a.author_id
JOIN post_tags pt ON p.post_id = pt.post_id
JOIN tags t ON pt.tag_id = t.tag_id;

-- Посты с авторами, каналами и тегами
CREATE VIEW full_post_info AS
SELECT p.post_id, p.title, a.name AS author, ch.name AS channel_name, t.name AS tag_name
FROM posts p
JOIN authors a ON p.author_id = a.author_id
JOIN channels ch ON p.channel_id = ch.channel_id
JOIN post_tags pt ON p.post_id = pt.post_id
JOIN tags t ON pt.tag_id = t.tag_id;

-- Посты с авторами, каналами, тегами и медиа
CREATE VIEW full_post_media AS
SELECT p.post_id, p.title, a.name AS author, ch.name AS channel_name, t.name AS tag_name, m.media_content AS media_url
FROM posts p
JOIN authors a ON p.author_id = a.author_id
JOIN channels ch ON p.channel_id = ch.channel_id
JOIN post_tags pt ON p.post_id = pt.post_id
JOIN tags t ON pt.tag_id = t.tag_id
JOIN media m ON m.post_id = p.post_id;
