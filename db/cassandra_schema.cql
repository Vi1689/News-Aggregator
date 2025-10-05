-- хранит много данных, которые часто записываются,
-- но не требуют сложных связей (JOIN).

CREATE KEYSPACE IF NOT EXISTS aggregator
WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 2};

USE aggregator;

-- Посты одного автора, отсортированные по дате
-- Что бы быстро выводить все посты конкретного автора
CREATE TABLE posts_by_author (
    author_id bigint,
    created_at timestamp,
    post_id bigint,
    title text,
    PRIMARY KEY ((author_id), created_at, post_id)
) WITH CLUSTERING ORDER BY (created_at DESC);

-- Посты с определённым тегом
-- Что бы быстро искать по тегу
CREATE TABLE posts_by_tag (
    tag text,
    created_at timestamp,
    post_id bigint,
    title text,
    PRIMARY KEY ((tag), created_at, post_id)
) WITH CLUSTERING ORDER BY (created_at DESC);

-- Лента постов для каждого пользователя
-- Что бы формировать персонализированную ленту
CREATE TABLE user_feed (
    user_id bigint,
    score double,
    created_at timestamp,
    post_id bigint,
    title text,
    PRIMARY KEY ((user_id), score, created_at, post_id)
) WITH CLUSTERING ORDER BY (score DESC, created_at DESC);

-- Кол-во постов, лайков и т.д. по авторам
-- Для аналитики
CREATE TABLE author_stats (
    author_id bigint PRIMARY KEY,
    total_posts int,
    total_likes int,
    total_comments int,
    last_post_at timestamp
);
