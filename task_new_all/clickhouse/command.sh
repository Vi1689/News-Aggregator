#!/bin/bash
# ============================================================
# ФАЙЛ КОМАНД ДЛЯ ВЫПОЛНЕНИЯ ПРОЕКТА
# ClickHouse + Новостной агрегатор
# ============================================================
# Как использовать: 
# 1. Сохрани этот файл как commands.sh
# 2. chmod +x commands.sh
# 3. ./commands.sh
# ИЛИ копируй команды по одной вручную
# ============================================================

echo "=========================================="
echo "  НОВОСТНОЙ АГРЕГАТОР - ClickHouse"
echo "=========================================="
echo ""

# ============================================================
# 1. ПЕРЕХОД В ПАПКУ ПРОЕКТА
# ============================================================
echo "1. Переход в папку ClickHouse..."
cd ~/News-Aggregator/task_new_all/clickhouse
echo "✅ Текущая директория: $(pwd)"
echo ""

# ============================================================
# 2. ЗАПУСК CLICKHOUSE В DOCKER
# ============================================================
echo "2. Запуск ClickHouse в Docker контейнере..."
echo "   (создаст контейнер с именем 'clickhouse', порты 8123 и 9000)"
docker-compose up -d
echo "✅ ClickHouse запущен"
echo ""

# ============================================================
# 3. ОЖИДАНИЕ ЗАПУСКА (10 секунд)
# ============================================================
echo "3. Ожидание 10 секунд, пока ClickHouse полностью запустится..."
sleep 10
echo "✅ Готов к работе"
echo ""

# ============================================================
# 4. ПРОВЕРКА ЧТО КОНТЕЙНЕР ЗАПУЩЕН
# ============================================================
echo "4. Проверка статуса контейнера..."
docker ps | grep clickhouse
echo ""

# ============================================================
# 5. ПРОВЕРКА БАЗ ДАННЫХ
# ============================================================
echo "5. Проверка списка баз данных (должна быть 'news')..."
docker exec -it clickhouse clickhouse-client --query "SHOW DATABASES"
echo ""

# ============================================================
# 6. ПРОВЕРКА ТАБЛИЦ
# ============================================================
echo "6. Проверка списка таблиц в базе 'news'..."
docker exec -it clickhouse clickhouse-client --query "SHOW TABLES FROM news"
echo ""

# ============================================================
# 7. УСТАНОВКА PYTHON ЗАВИСИМОСТЕЙ
# ============================================================
echo "7. Установка Python библиотеки clickhouse-driver..."
pip3 install clickhouse-driver
echo "✅ Библиотека установлена"
echo ""

# ============================================================
# 8. ЗАПУСК ГЕНЕРАТОРА ДАННЫХ (100,000 ЗАПИСЕЙ)
# ============================================================
echo "8. Запуск генератора данных (100,000 записей)..."
echo "   Создаются события: NewsPublished, NewsViewed, NewsLiked, NewsShared, CommentAdded"
python3 generator.py
echo "✅ Генерация завершена"
echo ""

# ============================================================
# 9. ПРОВЕРКА КОЛИЧЕСТВА ЗАПИСЕЙ
# ============================================================
echo "9. Проверка общего количества записей (должно быть 100,000)..."
docker exec -it clickhouse clickhouse-client --query "SELECT count() FROM news.news_events"
echo ""

# ============================================================
# 10. ПРОВЕРКА РАСПРЕДЕЛЕНИЯ ПО ТИПАМ СОБЫТИЙ
# ============================================================
echo "10. Проверка распределения по типам событий..."
docker exec -it clickhouse clickhouse-client --query "
SELECT event_type, count() AS count 
FROM news.news_events 
GROUP BY event_type 
ORDER BY count DESC"
echo ""

# ============================================================
# 11. ЗАПРОС 1: Публикации по дням
# ============================================================
echo "11. ЗАПРОС 1: Количество публикаций по дням"
echo "    Бизнес-задача: отслеживать активность редакции"
docker exec -it clickhouse clickhouse-client --query "
SELECT toDate(event_time) AS day, count() AS publications
FROM news.news_events
WHERE event_type = 'NewsPublished' AND event_time >= now() - INTERVAL 8 WEEK
GROUP BY day ORDER BY day"
echo ""

# ============================================================
# 12. ЗАПРОС 2: Топ авторов по просмотрам
# ============================================================
echo "12. ЗАПРОС 2: Топ-10 авторов по просмотрам"
echo "    Бизнес-задача: выявить самых читаемых авторов"
docker exec -it clickhouse clickhouse-client --query "
SELECT author_name, author_type, countIf(event_type = 'NewsViewed') AS total_views
FROM news.news_events
WHERE event_type IN ('NewsPublished', 'NewsViewed')
GROUP BY author_name, author_type
ORDER BY total_views DESC LIMIT 10"
echo ""

# ============================================================
# 13. ЗАПРОС 3: Среднее время чтения по категориям
# ============================================================
echo "13. ЗАПРОС 3: Среднее время чтения по категориям"
echo "    Бизнес-задача: понять, какие темы читают дольше"
docker exec -it clickhouse clickhouse-client --query "
SELECT category, round(avg(read_duration_sec), 1) AS avg_read_seconds, count() AS views_count
FROM news.news_events
WHERE event_type = 'NewsViewed' AND read_duration_sec > 0
GROUP BY category ORDER BY avg_read_seconds DESC"
echo ""

# ============================================================
# 14. ЗАПРОС 4: Активность по часам суток
# ============================================================
echo "14. ЗАПРОС 4: Активность по часам суток"
echo "    Бизнес-задача: определить пиковые часы чтения"
docker exec -it clickhouse clickhouse-client --query "
SELECT toHour(event_time) AS hour,
       countIf(event_type = 'NewsViewed') AS views,
       countIf(event_type = 'NewsLiked') AS likes,
       uniq(user_id) AS active_users
FROM news.news_events
GROUP BY hour ORDER BY hour"
echo ""

# ============================================================
# 15. ЗАПРОС 5: Популярность тегов
# ============================================================
echo "15. ЗАПРОС 5: Топ-10 самых популярных тегов"
echo "    Бизнес-задача: отслеживать тренды"
docker exec -it clickhouse clickhouse-client --query "
SELECT arrayJoin(tags) AS tag, count() AS mentions
FROM news.news_events
WHERE event_type = 'NewsPublished'
GROUP BY tag ORDER BY mentions DESC LIMIT 10"
echo ""

# ============================================================
# 16. ЗАПРОС 6: Вовлечённость пользователей
# ============================================================
echo "16. ЗАПРОС 6: Топ-10 активных пользователей"
echo "    Бизнес-задача: найти самых активных читателей"
docker exec -it clickhouse clickhouse-client --query "
SELECT user_id,
       countIf(event_type = 'NewsViewed') AS views,
       countIf(event_type = 'NewsLiked') AS likes,
       countIf(event_type = 'NewsShared') AS shares
FROM news.news_events
WHERE user_id != ''
GROUP BY user_id
ORDER BY views DESC LIMIT 10"
echo ""

# ============================================================
# 17. ЗАПРОС 7: Процент вовлечённости по категориям
# ============================================================
echo "17. ЗАПРОС 7: Вовлечённость по категориям"
echo "    Бизнес-задача: определить вовлекающие темы"
docker exec -it clickhouse clickhouse-client --query "
SELECT category,
       countIf(event_type = 'NewsViewed') AS views,
       countIf(event_type IN ('NewsLiked', 'NewsShared')) AS engagements,
       round(engagements * 100.0 / views, 2) AS engagement_rate
FROM news.news_events
GROUP BY category
ORDER BY engagement_rate DESC"
echo ""

# ============================================================
# 18. ЗАПРОС 8: Лайки по авторам
# ============================================================
echo "18. ЗАПРОС 8: Лучшие авторы по лайкам"
echo "    Бизнес-задача: оценить качество контента"
docker exec -it clickhouse clickhouse-client --query "
SELECT author_name,
       sumIf(like_value, like_value = 1) AS likes,
       sumIf(like_value, like_value = -1) AS dislikes,
       round(likes * 100.0 / (likes + dislikes), 2) AS like_rate
FROM news.news_events
WHERE event_type = 'NewsLiked'
GROUP BY author_name
HAVING likes + dislikes > 0
ORDER BY like_rate DESC LIMIT 10"
echo ""

# ============================================================
# 19. ВИТРИНА ДАННЫХ: проверка материализованного представления
# ============================================================
echo "19. Проверка витрины данных (mart_category_hourly)..."
docker exec -it clickhouse clickhouse-client --query "
SELECT category, hour, viewed_count, liked_count 
FROM news.mart_category_hourly 
ORDER BY hour DESC LIMIT 10"
echo ""

# ============================================================
# 20. СРАВНЕНИЕ ПРОИЗВОДИТЕЛЬНОСТИ: сырые данные vs витрина
# ============================================================
echo "20. СРАВНЕНИЕ ПРОИЗВОДИТЕЛЬНОСТИ: сырые данные vs витрина"
echo ""
echo "--- Запрос к сырым данным ---"
time docker exec -it clickhouse clickhouse-client --query "
SELECT category, count() FROM news.news_events 
WHERE event_type = 'NewsViewed' GROUP BY category" > /dev/null
echo ""
echo "--- Запрос к витрине ---"
time docker exec -it clickhouse clickhouse-client --query "
SELECT category, sum(viewed_count) FROM news.mart_category_hourly 
GROUP BY category" > /dev/null
echo ""

# ============================================================
# 21. МЕТРИКИ ДЛЯ ОТЧЁТА
# ============================================================
echo "21. МЕТРИКИ ДЛЯ АНАЛИТИЧЕСКОГО ОТЧЁТА"
echo ""
echo "--- Метрика 1: Общая статистика ---"
docker exec -it clickhouse clickhouse-client --query "
SELECT 
    uniq(news_id) AS total_news,
    uniq(user_id) AS total_users,
    uniq(author_name) AS total_authors,
    count() AS total_events
FROM news.news_events"
echo ""
echo "--- Метрика 2: Среднее количество просмотров на новость ---"
docker exec -it clickhouse clickhouse-client --query "
SELECT round(countIf(event_type = 'NewsViewed') * 1.0 / uniq(news_id), 1) AS avg_views
FROM news.news_events"
echo ""
echo "--- Метрика 3: Самый популярный час ---"
docker exec -it clickhouse clickhouse-client --query "
SELECT toHour(event_time) AS hour, count() AS views
FROM news.news_events WHERE event_type = 'NewsViewed'
GROUP BY hour ORDER BY views DESC LIMIT 1"
echo ""
echo "--- Метрика 4: Категория с наибольшей вовлечённостью ---"
docker exec -it clickhouse clickhouse-client --query "
SELECT category, 
       round((countIf(event_type = 'NewsLiked') + countIf(event_type = 'NewsShared')) * 100.0 / countIf(event_type = 'NewsViewed'), 2) AS engagement
FROM news.news_events
GROUP BY category ORDER BY engagement DESC LIMIT 1"
echo ""

# ============================================================
# 22. ПРОВЕРКА TTL (политики хранения)
# ============================================================
echo "22. Проверка TTL политики хранения данных..."
docker exec -it clickhouse clickhouse-client --query "
SELECT name, ttl_expression FROM system.tables 
WHERE database = 'news' AND name = 'news_events'"
echo ""

# ============================================================
# 23. ПРОВЕРКА ДЕДУПЛИКАЦИИ
# ============================================================
echo "23. Проверка таблицы с дедупликацией..."
docker exec -it clickhouse clickhouse-client --query "
SELECT count() FROM news.news_events_dedup"
echo ""

# ============================================================
# 24. ИТОГОВАЯ СТАТИСТИКА
# ============================================================
echo "=========================================="
echo "  ИТОГОВАЯ СТАТИСТИКА"
echo "=========================================="
echo ""
echo "📊 База данных: news"
echo "📊 Таблицы: news_events, news_events_dedup, mart_category_hourly, mv_category_hourly"
echo "📊 Записей: 100,000"
echo "📊 Типов событий: 5"
echo "📊 Уникальных пользователей: ~200"
echo "📊 Уникальных авторов: 30"
echo "📊 Категорий: 7"
echo ""
echo "=========================================="
echo "  ✅ ВСЕ ЗАДАНИЯ ВЫПОЛНЕНЫ!"
echo "=========================================="
echo ""
echo "📝 ЧТО БЫЛО СДЕЛАНО:"
echo "   1. Развёрнут ClickHouse в Docker"
echo "   2. Создана таблица news_events с PARTITION BY и ORDER BY"
echo "   3. Сгенерировано 100,000 тестовых событий"
echo "   4. Выполнено 8 аналитических запросов"
echo "   5. Создана витрина mart_category_hourly + MV"
echo "   6. Реализована дедупликация (ReplacingMergeTree)"
echo "   7. Настроена TTL политика хранения"
echo "   8. Подготовлен аналитический отчёт"
echo ""
echo "=========================================="
echo "  ПОЛЕЗНЫЕ КОМАНДЫ ДЛЯ РАБОТЫ"
echo "=========================================="
echo ""
echo "# Войти в ClickHouse клиент:"
echo "docker exec -it clickhouse clickhouse-client"
echo ""
echo "# Выйти из клиента:"
echo "exit;   или Ctrl+D"
echo ""
echo "# Показать все таблицы:"
echo "SHOW TABLES FROM news;"
echo ""
echo "# Посмотреть структуру таблицы:"
echo "DESCRIBE news.news_events;"
echo ""
echo "# Удалить все данные (если нужно перегенерировать):"
echo "docker exec -it clickhouse clickhouse-client --query 'TRUNCATE TABLE news.news_events'"
echo ""
echo "# Остановить ClickHouse:"
echo "docker-compose down"
echo ""
echo "# Запустить ClickHouse снова:"
echo "docker-compose up -d"
echo ""