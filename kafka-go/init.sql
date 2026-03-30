-- Таблица авторов
CREATE TABLE IF NOT EXISTS authors (
    author_id INTEGER PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Таблица текстов новостей
CREATE TABLE IF NOT EXISTS news_texts (
    text_id SERIAL PRIMARY KEY,
    text TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Таблица постов
CREATE TABLE IF NOT EXISTS posts (
    post_id INTEGER PRIMARY KEY,
    title VARCHAR(500) NOT NULL,
    author_id INTEGER REFERENCES authors(author_id),
    text_id INTEGER REFERENCES news_texts(text_id),
    channel_id INTEGER,
    likes_count INTEGER DEFAULT 0,
    comments_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Таблица тегов
CREATE TABLE IF NOT EXISTS tags (
    tag_id SERIAL PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Связь постов с тегами
CREATE TABLE IF NOT EXISTS post_tags (
    post_id INTEGER REFERENCES posts(post_id) ON DELETE CASCADE,
    tag_id INTEGER REFERENCES tags(tag_id) ON DELETE CASCADE,
    PRIMARY KEY (post_id, tag_id)
);
