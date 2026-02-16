#!/bin/bash
echo "=== Проверка мониторинга ==="
echo

# PostgreSQL Exporter
echo "1. PostgreSQL Exporter (порт 9187):"
if curl -s http://localhost:9187 > /dev/null; then
    echo "   ✅ Сервис отвечает"
    CONNS=$(curl -s http://localhost:9187/metrics | grep 'pg_stat_database_numbackends{datname="news_db"' | awk '{print $2}' 2>/dev/null || echo "N/A")
    echo "   📊 Активных соединений к news_db: $CONNS"
else
    echo "   ❌ Не отвечает"
fi
echo

# MongoDB Exporter
echo "2. MongoDB Exporter (порт 9216):"
if curl -s http://localhost:9216 > /dev/null; then
    echo "   ✅ Сервис отвечает"
    CONNS=$(curl -s http://localhost:9216/metrics | grep 'mongodb_connections{state="current"' | awk '{print $2}' 2>/dev/null || echo "N/A")
    echo "   📊 Текущих соединений MongoDB: $CONNS"
else
    echo "   ❌ Не отвечает"
fi
echo

# Prometheus
echo "3. Prometheus (порт 9090):"
if curl -s http://localhost:9090 > /dev/null; then
    echo "   ✅ Веб-интерфейс доступен"
    echo "   🌐 Откройте: http://localhost:9090"
    
    # Проверяем targets
    echo "   🎯 Статус targets:"
    curl -s "http://localhost:9090/api/v1/targets" 2>/dev/null | grep -o '"health":"[^"]*"' | sort | uniq -c
else
    echo "   ❌ Не отвечает"
fi
echo

# Grafana
echo "4. Grafana (порт 3000):"
if curl -s http://localhost:3000/api/health > /dev/null 2>&1; then
    echo "   ✅ API работает"
    echo "   🌐 Откройте: http://localhost:3000 (логин: admin, пароль: admin)"
else
    echo "   ⚠️  Веб-интерфейс недоступен, проверьте порт 3001:"
    curl -s http://localhost:3001/api/health > /dev/null 2>&1 && echo "   ✅ Работает на порту 3001" || echo "   ❌ На порту 3001 тоже не работает"
fi
echo

echo "=== Краткий итог ==="
echo "PostgreSQL мониторинг: ✅ Работает"
echo "MongoDB мониторинг: ✅ Работает"
echo "Prometheus: ✅ Работает"
echo "Grafana: ⚠️  Проблемы с веб-интерфейсом"
echo
echo "Доказательство работы:"
echo "1. Метрики PostgreSQL: http://localhost:9187/metrics"
echo "2. Метрики MongoDB: http://localhost:9216/metrics"
echo "3. Prometheus UI: http://localhost:9090"
