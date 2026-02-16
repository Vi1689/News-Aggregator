#!/bin/bash
echo "========================================="
echo "🚀 Neo4j Auto-Initialization Script"
echo "========================================="

# Даем Neo4j время полностью запуститься
echo "⏳ Waiting 30 seconds for Neo4j to fully start..."
sleep 30

# Максимальное количество попыток
MAX_RETRIES=20
RETRY_COUNT=0

echo "🔌 Checking Neo4j connection..."
until cypher-shell -u neo4j -p test12345 "RETURN 1" > /dev/null 2>&1; do
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -ge $MAX_RETRIES ]; then
        echo "❌ Cannot connect to Neo4j after $MAX_RETRIES attempts"
        exit 1
    fi
    echo "   Waiting for Neo4j to accept connections... ($RETRY_COUNT/$MAX_RETRIES)"
    sleep 3
done

echo "✅ Connected to Neo4j"

# Проверяем, нужно ли инициализировать
echo "🔍 Checking if initialization is needed..."
if cypher-shell -u neo4j -p test12345 "SHOW USERS" 2>/dev/null | grep -q "reader"; then
    echo "✅ Database already initialized, skipping..."
    exit 0
fi

# Выполняем инициализацию
echo "📝 Running initialization script..."
if [ -f "/init/init_neo4j.cypher" ]; then
    echo "   Found script at /init/init_neo4j.cypher"
    
    # Выполняем скрипт и сохраняем вывод
    OUTPUT=$(cypher-shell -u neo4j -p test12345 -f /init/init_neo4j.cypher 2>&1)
    RESULT=$?
    
    if [ $RESULT -eq 0 ]; then
        echo "✅ Initialization completed successfully!"
        echo "📊 Summary:"
        echo "$OUTPUT" | grep -E "Added|Created|constraint|index" || true
    else
        echo "❌ Initialization failed with error:"
        echo "$OUTPUT"
        exit 1
    fi
else
    echo "❌ Init script not found at /init/init_neo4j.cypher"
    ls -la /init/
    exit 1
fi

echo "========================================="
echo "✅ Neo4j is ready to use!"