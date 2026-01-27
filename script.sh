echo "=== ПРОВЕРКА ВСЕХ КОМПОНЕНТОВ ЧЕРЕЗ GO СЕРВЕР ==="

# 1. Health check
echo "1. Health:"
curl -s http://localhost:8080/health

# 2. PostgreSQL источники
echo -e "\n2. PostgreSQL источники:"
curl -s http://localhost:8080/api/sources

# 3. MongoDB поиск (должны быть данные от предыдущих тестов)
echo -e "\n3. MongoDB поиск:"
curl -s -X POST http://localhost:8080/api/mongo/search/advanced \
  -H "Content-Type: application/json" \
  -d '{}' | head -c 200

# 4. MongoDB аналитика
echo -e "\n4. MongoDB аналитика (топ теги):"
curl -s "http://localhost:8080/api/mongo/analytics/top-tags?limit=3"