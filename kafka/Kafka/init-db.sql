-- Таблицы для агрегатора новостей

-- Все входящие события
CREATE TABLE IF NOT EXISTS news_events (
    id          BIGSERIAL PRIMARY KEY,
    event_id    VARCHAR(36) NOT NULL UNIQUE,
    event_type  VARCHAR(50) NOT NULL,   -- NewsPublished / NewsUpdated / NewsTrending
    timestamp   TIMESTAMPTZ NOT NULL,
    source      VARCHAR(100),
    version     VARCHAR(10),
    entry_id    VARCHAR(100),
    source_id   VARCHAR(50),
    payload     JSONB,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Статьи
CREATE TABLE IF NOT EXISTS articles (
    id              BIGSERIAL PRIMARY KEY,
    article_id      VARCHAR(36) NOT NULL UNIQUE,
    source_id       VARCHAR(50) NOT NULL,
    source_region   VARCHAR(100),
    category        VARCHAR(50),
    title           TEXT,
    author          VARCHAR(100),
    views           INTEGER DEFAULT 0,
    likes           INTEGER DEFAULT 0,
    shares          INTEGER DEFAULT 0,
    reliability     VARCHAR(20),
    credibility_score FLOAT DEFAULT 5.0,
    published_at    TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Агрегации по категориям
CREATE TABLE IF NOT EXISTS category_stats (
    id              BIGSERIAL PRIMARY KEY,
    category        VARCHAR(50) NOT NULL,
    total_articles  INTEGER DEFAULT 0,
    total_views     BIGINT DEFAULT 0,
    avg_credibility FLOAT DEFAULT 0,
    updated_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(category)
);

-- Агрегации по регионам
CREATE TABLE IF NOT EXISTS region_stats (
    id              BIGSERIAL PRIMARY KEY,
    region          VARCHAR(100) NOT NULL,
    total_articles  INTEGER DEFAULT 0,
    trending_count  INTEGER DEFAULT 0,
    updated_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(region)
);

-- Трендовые новости (оконные агрегации)
CREATE TABLE IF NOT EXISTS trending_news (
    id              BIGSERIAL PRIMARY KEY,
    source_id       VARCHAR(50) NOT NULL,
    window_start    TIMESTAMPTZ NOT NULL,
    window_end      TIMESTAMPTZ NOT NULL,
    trending_count  INTEGER DEFAULT 0,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Справочник источников (для Kafka Connect Source)
CREATE TABLE IF NOT EXISTS news_sources (
    source_id       VARCHAR(50) PRIMARY KEY,
    source_name     VARCHAR(100),
    region          VARCHAR(100),
    reliability     VARCHAR(20),
    fact_check_score FLOAT DEFAULT 5.0,
    is_active       BOOLEAN DEFAULT true,
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Тестовые данные
INSERT INTO news_sources (source_id, source_name, region, reliability, fact_check_score) VALUES
    ('reuters', 'Reuters', 'Global', 'HIGH', 9.5),
    ('bbc', 'BBC News', 'Europe', 'HIGH', 9.0),
    ('cnn', 'CNN', 'Americas', 'MEDIUM', 7.5),
    ('aljazeera', 'Al Jazeera', 'MiddleEast', 'HIGH', 8.5),
    ('tass', 'TASS', 'Russia', 'MEDIUM', 6.0)
ON CONFLICT DO NOTHING;

-- Индексы
CREATE INDEX IF NOT EXISTS idx_events_source ON news_events(source);
CREATE INDEX IF NOT EXISTS idx_events_type ON news_events(event_type);
CREATE INDEX IF NOT EXISTS idx_articles_source ON articles(source_id);
CREATE INDEX IF NOT EXISTS idx_articles_category ON articles(category);
CREATE INDEX IF NOT EXISTS idx_trending_window ON trending_news(window_start, window_end);