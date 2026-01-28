#!/bin/bash
echo "=== VK Researcher Debug Check ==="
echo "Time: $(date)"

# Проверяем, доступен ли сервер
echo -e "\n1. Checking server connectivity..."
curl -s -o /dev/null -w "HTTP Status: %{http_code}\n" http://server:8080/health

# Проверяем логи ресерчера
echo -e "\n2. Checking researcher logs..."
if [ -f "/var/log/vk-researcher/researcher_$(date +%Y-%m-%d).log" ]; then
    echo "Log file exists. Last 5 entries:"
    tail -5 "/var/log/vk-researcher/researcher_$(date +%Y-%m-%d).log"
else
    echo "No log file found"
fi

# Проверяем таблицы в БД
echo -e "\n3. Checking database tables..."
docker exec news-aggregator-db psql -U news_user -d news_db -c "SELECT COUNT(*) as posts_count FROM posts;"
docker exec news-aggregator-db psql -U news_user -d news_db -c "SELECT COUNT(*) as sources_count FROM sources;"
docker exec news-aggregator-db psql -U news_user -d news_db -c "SELECT COUNT(*) as channels_count FROM channels;"

echo -e "\n=== Check Complete ==="