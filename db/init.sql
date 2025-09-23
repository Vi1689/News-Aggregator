-- Инициализация схемы для агрегатора новостей

CREATE TABLE IF NOT EXISTS users (
    user_id SERIAL PRIMARY KEY,
    access_level VARCHAR(20) NOT NULL
);

CREATE TABLE IF NOT EXISTS authors (
    author_id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);

CREATE TABLE IF NOT EXISTS news_texts (
    text_id SERIAL PRIMARY KEY,
    text TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sources (
    source_id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    address VARCHAR(255) NOT NULL
);

CREATE TABLE IF NOT EXISTS channels (
    channel_id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    link VARCHAR(255),
    subscribers_count INT DEFAULT 0,
    source_id INT REFERENCES sources(source_id),
    topic VARCHAR(255)
);

CREATE TABLE IF NOT EXISTS posts (
    post_id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    author_id INT REFERENCES authors(author_id),
    text_id INT REFERENCES news_texts(text_id),
    channel_id INT REFERENCES channels(channel_id),
    comments_count INT DEFAULT 0,
    likes_count INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS media (
    media_id SERIAL PRIMARY KEY,
    post_id INT REFERENCES posts(post_id),
    media_content VARCHAR(255)
);

CREATE TABLE IF NOT EXISTS tags (
    tag_id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS post_tags (
    post_id INT REFERENCES posts(post_id),
    tag_id INT REFERENCES tags(tag_id),
    PRIMARY KEY (post_id, tag_id)
);

CREATE TABLE IF NOT EXISTS comments (
    comment_id SERIAL PRIMARY KEY,
    post_id INT REFERENCES posts(post_id),
    nickname VARCHAR(255) NOT NULL,
    parent_comment_id INT REFERENCES comments(comment_id),
    text TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    likes_count INT DEFAULT 0
);
