#!/bin/bash
echo "Заполнение данных через API (порт 8080)..."

echo "1. Создаем авторов:"
curl -X POST http://localhost:8080/api/authors \
  -H "Content-Type: application/json" \
  -d '{"name": "Иван Петров"}' -s | jq . 2>/dev/null || echo

echo "2. Создаем теги:"
curl -X POST http://localhost:8080/api/tags \
  -H "Content-Type: application/json" \
  -d '{"name": "Технологии", "slug": "tech"}' -s | jq . 2>/dev/null || echo

echo "3. Создаем пост:"
curl -X POST http://localhost:8080/api/posts \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Тестовая новость",
    "content": "Содержимое тестовой новости...",
    "summary": "Краткое описание",
    "author_id": 1,
    "tag_ids": [1]
  }' -s | jq . 2>/dev/null || echo

echo -e "\n=== ПРОВЕРКА ==="
echo "Авторы:"
curl -s http://localhost:8080/api/authors | jq . 2>/dev/null || curl -s http://localhost:8080/api/authors

echo -e "\nПосты:"
curl -s http://localhost:8080/api/posts | jq . 2>/dev/null || curl -s http://localhost:8080/api/posts
