#!/bin/bash
set -e

# --- Настройки ---
TOTAL_POSTS=3200000
BATCH_SIZE=50000
DB_USER="news_user"
DB_NAME="news_db"
CPP_SERVER_URL="http://localhost:8080/health"

# --- 1. Остановка старых контейнеров ---
echo "🚀 Останавливаем и удаляем старые контейнеры..."
docker compose down -v --remove-orphans

# --- 2. Пересборка образов ---
echo "🛠 Пересобираем проект..."
docker compose build --no-cache

# --- 3. Поднимаем контейнеры ---
echo "⬆️ Поднимаем контейнеры..."
docker compose up -d

# --- 4. Ждём готовности PostgreSQL ---
echo "⏳ Ждём запуск базы данных..."
until docker compose exec -T db pg_isready -U $DB_USER -d $DB_NAME > /dev/null 2>&1; do
    echo "Waiting for PostgreSQL..."
    sleep 2
done
echo "✅ PostgreSQL готов!"

# --- 5. Очистка таблиц ---
echo "🧹 Очищаем таблицы..."
docker compose exec -T db psql -U $DB_USER -d $DB_NAME <<EOF
TRUNCATE TABLE post_tags, posts, news_texts, authors, channels, sources, tags, comments, media CASCADE;
EOF

# --- 6. Генерация источников, авторов, текстов и тегов ---
echo "📥 Генерируем источники, авторов, тексты новостей и теги..."
docker compose exec -T db psql -U $DB_USER -d $DB_NAME <<EOF
-- Источники
INSERT INTO sources (name, address)
SELECT 'Источник #' || g, 'https://source' || g || '.com'
FROM generate_series(1, 10) g;

-- Авторы
INSERT INTO authors (name)
SELECT 'Автор #' || g FROM generate_series(1, 80000) g;

-- Тексты новостей
INSERT INTO news_texts (text)
SELECT 'Текст новости номер ' || g FROM generate_series(1, $TOTAL_POSTS) g;

-- Теги
INSERT INTO tags (name)
SELECT 'Тег #' || g FROM generate_series(1, 10000) g;
EOF

# --- 7. Генерация каналов с корректными source_id ---
echo "📺 Генерируем каналы..."
docker compose exec -T db psql -U $DB_USER -d $DB_NAME <<EOF
INSERT INTO channels (name, link, subscribers_count, source_id, topic)
SELECT 
    'Канал #' || g,
    'https://channel' || g || '.com',
    (random() * 10000)::int,
    s.source_id,
    'Тема ' || ((g % 6) + 1)
FROM generate_series(1, 50) g
JOIN sources s ON s.source_id = ((g - 1) % (SELECT COUNT(*) FROM sources) + 1);
EOF

# --- 8. Генерация постов, post_tags, комментариев и медиа по батчам ---
echo "📰 Генерируем посты, теги, комментарии и медиа..."
for ((i=1; i<=TOTAL_POSTS; i+=BATCH_SIZE)); do
docker compose exec -T db psql -U $DB_USER -d $DB_NAME <<EOF
DO \$\$
DECLARE
    g INT;
    total_authors INT := (SELECT COUNT(*) FROM authors);
    total_tags INT := (SELECT COUNT(*) FROM tags);
    comment_count INT;
    media_count INT;
    c INT;
BEGIN
    FOR g IN 1..$BATCH_SIZE LOOP
        -- Вставка поста с существующим channel_id
        INSERT INTO posts (title, author_id, text_id, channel_id, comments_count, likes_count, created_at)
        VALUES (
            'Новость #' || (g + $i - 1),
            FLOOR(random() * total_authors + 1)::int,
            (g + $i - 1),
            (SELECT channel_id FROM channels ORDER BY random() LIMIT 1),
            0,
            FLOOR(random() * 500)::int,
            NOW() - ((g + $i - 1) || ' seconds')::interval
        );

        -- Связи с тегами
        INSERT INTO post_tags (post_id, tag_id)
        VALUES ((g + $i - 1), FLOOR(random() * total_tags + 1)::int)
        ON CONFLICT DO NOTHING;

        -- Генерация комментариев (0-5)
        comment_count := FLOOR(random() * 6)::int;
        FOR c IN 1..comment_count LOOP
            INSERT INTO comments (post_id, nickname, text, likes_count)
            VALUES (
                (g + $i - 1),
                'Пользователь #' || FLOOR(random()*10000 + 1)::int,
                'Комментарий к посту #' || (g + $i - 1),
                FLOOR(random() * 100)::int
            );
        END LOOP;

        -- Генерация медиа (0-3)
        media_count := FLOOR(random() * 4)::int;
        FOR c IN 1..media_count LOOP
            INSERT INTO media (post_id, media_content)
            VALUES (
                (g + $i - 1),
                'https://media.example.com/' || FLOOR(random()*10000 + 1)::int || '.jpg'
            );
        END LOOP;
    END LOOP;
END \$\$;
EOF
echo "Inserted $((i + BATCH_SIZE - 1)) / $TOTAL_POSTS posts with comments and media"
done

# --- 9. Ждём готовности C++ сервера ---
echo "🌍 Ждём запуска C++ сервера..."
until curl -s $CPP_SERVER_URL > /dev/null; do
    echo "Waiting for C++ server..."
    sleep 2
done
echo "✅ C++ сервер готов!"

# --- 10. Проверка данных ---
echo "🔎 Проверка: выводим первые 5 постов..."
docker compose exec -T db psql -U $DB_USER -d $DB_NAME -c "SELECT post_id, title, created_at FROM posts LIMIT 5;"

echo "🎉 Всё готово! Сервер работает на http://localhost:8080"
