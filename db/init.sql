-- init.sql для PostgreSQL 

-- Пользователи агрегатора
CREATE TABLE IF NOT EXISTS users (
    user_id SERIAL PRIMARY KEY,
    username VARCHAR(255) UNIQUE,
    access_level VARCHAR(20) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Авторы постов
CREATE TABLE IF NOT EXISTS authors (
    author_id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    UNIQUE(name)
);

-- Тексты новостей
CREATE TABLE IF NOT EXISTS news_texts (
    text_id SERIAL PRIMARY KEY,
    text TEXT NOT NULL
);

-- Источники (сайты)
CREATE TABLE IF NOT EXISTS sources (
    source_id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    address VARCHAR(255) NOT NULL,
    topic VARCHAR(255)
);

-- Сообщества / каналы
CREATE TABLE IF NOT EXISTS channels (
    channel_id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    link VARCHAR(255),
    subscribers_count INT DEFAULT 0,
    source_id INT REFERENCES sources(source_id) ON DELETE SET NULL,
    topic VARCHAR(255)
);

-- Новости (посты)
CREATE TABLE IF NOT EXISTS posts (
    post_id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    author_id INT REFERENCES authors(author_id) ON DELETE SET NULL,
    text_id INT REFERENCES news_texts(text_id) ON DELETE CASCADE,
    channel_id INT REFERENCES channels(channel_id) ON DELETE SET NULL,
    comments_count INT DEFAULT 0,
    likes_count INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Медиа (фото, видео и т.п.)
CREATE TABLE IF NOT EXISTS media (
    media_id SERIAL PRIMARY KEY,
    post_id INT REFERENCES posts(post_id) ON DELETE CASCADE,
    media_content VARCHAR(1000),
    media_type VARCHAR(50) DEFAULT 'image'
);

-- Теги
CREATE TABLE IF NOT EXISTS tags (
    tag_id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE
);

-- Связка постов и тегов
CREATE TABLE IF NOT EXISTS post_tags (
    post_id INT REFERENCES posts(post_id) ON DELETE CASCADE,
    tag_id INT REFERENCES tags(tag_id) ON DELETE CASCADE,
    PRIMARY KEY (post_id, tag_id)
);

-- Комментарии
CREATE TABLE IF NOT EXISTS comments (
    comment_id SERIAL PRIMARY KEY,
    post_id INT REFERENCES posts(post_id) ON DELETE CASCADE,
    nickname VARCHAR(255) NOT NULL,
    parent_comment_id INT REFERENCES comments(comment_id) ON DELETE CASCADE,
    text TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    likes_count INT DEFAULT 0
);
