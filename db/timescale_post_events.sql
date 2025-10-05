-- хранит действия пользователей во времени — просмотры, лайки, клики

CREATE TABLE post_events (
    time TIMESTAMPTZ NOT NULL DEFAULT now(),
    post_id INT NOT NULL,
    user_id INT,
    event_type TEXT NOT NULL,  -- 'view', 'like', 'share', ...
    value INT DEFAULT 1
);

-- Превращаем таблицу в hypertable
SELECT create_hypertable('post_events', 'time', if_not_exists => TRUE);

-- Индексы
CREATE INDEX idx_post_events_post_time ON post_events(post_id, time DESC);

-- Пример агрегирующего запроса
-- Сколько просмотров постов за последний час
SELECT time_bucket('1 hour', time) AS hour,
       post_id,
       COUNT(*) AS views
FROM post_events
WHERE event_type = 'view'
GROUP BY hour, post_id
ORDER BY hour DESC;
