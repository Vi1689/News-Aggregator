#!/bin/bash
# init-sharding.sh - Идемпотентный скрипт инициализации MongoDB Sharded Cluster

set -e

echo "🚀 Starting MongoDB Sharding Cluster Initialization..."
echo "⏳ Waiting for services to be ready..."
sleep 25

# ============================================
# ФУНКЦИИ ДЛЯ ПРОВЕРКИ СОСТОЯНИЯ
# ============================================

check_replica_set_status() {
    local host=$1
    local port=$2
    echo "Checking replica set on $host:$port..."
    
    # Пробуем несколько способов проверки
    local result=$(mongosh --host $host:$port --quiet --eval '
    try {
        var status = rs.status();
        if (status && status.ok === 1) {
            print("INITIALIZED");
        } else {
            print("NOT_INITIALIZED");
        }
    } catch (e) {
        if (e.codeName === "NotYetInitialized") {
            print("NOT_INITIALIZED");
        } else if (e.codeName === "Unauthorized") {
            print("AUTH_ERROR");
        } else {
            print("ERROR:" + e.codeName);
        }
    }
    ' 2>/dev/null)
    
    case "$result" in
        "INITIALIZED")
            echo "✅ Replica set already initialized on $host:$port"
            return 0
            ;;
        "NOT_INITIALIZED")
            echo "❌ Replica set not initialized on $host:$port"
            return 1
            ;;
        *)
            echo "⚠️ Unknown state on $host:$port: $result"
            return 1
            ;;
    esac
}

check_shard_added() {
    local shard_name=$1
    echo "Checking if shard $shard_name is already added..."
    
    if mongosh --host mongos1:27017 --quiet --eval "sh.status()" | grep -q "$shard_name"; then
        echo "✅ Shard $shard_name already added"
        return 0
    else
        echo "❌ Shard $shard_name not found"
        return 1
    fi
}

# ============================================
# ШАГ 1: ИНИЦИАЛИЗАЦИЯ CONFIG SERVER REPLICA SET
# ============================================

echo "📋 Step 1: Configuring Config Server Replica Set..."

if ! check_replica_set_status "config1" "27019"; then
    echo "Initializing config replica set..."
    mongosh --host config1:27019 --eval '
    try {
        rs.initiate({
            _id: "configRS",
            configsvr: true,
            members: [
                { _id: 0, host: "config1:27019" },
                { _id: 1, host: "config2:27019" },
                { _id: 2, host: "config3:27019" }
            ]
        })
        print("✅ Config replica set initialized");
    } catch (e) {
        if (e.codeName === "AlreadyInitialized") {
            print("ℹ️ Config replica set already initialized");
        } else {
            throw e;
        }
    }
    '
    echo "⏳ Waiting for config servers to sync..."
    sleep 15
else
    echo "Skipping config server initialization - already done"
fi

# ============================================
# ШАГ 2: ИНИЦИАЛИЗАЦИЯ SHARD 0 REPLICA SET
# ============================================

echo "📋 Step 2: Configuring Shard 0 Replica Set..."

if ! check_replica_set_status "shard0-primary" "27018"; then
    echo "Initializing shard0 replica set..."
    mongosh --host shard0-primary:27018 --eval '
    try {
        rs.initiate({
            _id: "shard0RS",
            members: [
                { _id: 0, host: "shard0-primary:27018" },
                { _id: 1, host: "shard0-secondary:27018" }
            ]
        })
        print("✅ Shard0 replica set initialized");
    } catch (e) {
        if (e.codeName === "AlreadyInitialized") {
            print("ℹ️ Shard0 replica set already initialized");
        } else {
            throw e;
        }
    }
    '
    sleep 10
else
    echo "Skipping shard0 initialization - already done"
fi

# ============================================
# ШАГ 3: ИНИЦИАЛИЗАЦИЯ SHARD 1 REPLICA SET
# ============================================

echo "📋 Step 3: Configuring Shard 1 Replica Set..."

if ! check_replica_set_status "shard1-primary" "27018"; then
    echo "Initializing shard1 replica set..."
    mongosh --host shard1-primary:27018 --eval '
    try {
        rs.initiate({
            _id: "shard1RS",
            members: [
                { _id: 0, host: "shard1-primary:27018" },
                { _id: 1, host: "shard1-secondary:27018" }
            ]
        })
        print("✅ Shard1 replica set initialized");
    } catch (e) {
        if (e.codeName === "AlreadyInitialized") {
            print("ℹ️ Shard1 replica set already initialized");
        } else {
            throw e;
        }
    }
    '
    sleep 10
else
    echo "Skipping shard1 initialization - already done"
fi

# ============================================
# ШАГ 4: ДОБАВЛЕНИЕ ШАРДОВ В КЛАСТЕР
# ============================================

echo "📋 Step 4: Adding Shards to Cluster..."
echo "⏳ Waiting for mongos to be ready..."
sleep 20

# Проверяем, что mongos доступен
until mongosh --host mongos1:27017 --quiet --eval "db.adminCommand('ping').ok" | grep -q "1"; do
    echo "Waiting for mongos1 to be ready..."
    sleep 5
done

# Добавляем shard0 если еще не добавлен
if ! check_shard_added "shard0RS"; then
    echo "Adding shard0 to cluster..."
    mongosh --host mongos1:27017 --eval '
    try {
        sh.addShard("shard0RS/shard0-primary:27018")
        print("✅ Shard0 added to cluster");
    } catch (e) {
        if (e.codeName === "OperationFailed") {
            print("ℹ️ Shard0 may already be added");
        } else {
            throw e;
        }
    }
    '
    sleep 5
fi

# Добавляем shard1 если еще не добавлен
if ! check_shard_added "shard1RS"; then
    echo "Adding shard1 to cluster..."
    mongosh --host mongos1:27017 --eval '
    try {
        sh.addShard("shard1RS/shard1-primary:27018")
        print("✅ Shard1 added to cluster");
    } catch (e) {
        if (e.codeName === "OperationFailed") {
            print("ℹ️ Shard1 may already be added");
        } else {
            throw e;
        }
    }
    '
    sleep 5
fi

# ============================================
# ШАГ 5: НАСТРОЙКА SHARDING ДЛЯ БАЗЫ ДАННЫХ
# ============================================

echo "📋 Step 5: Configuring database sharding..."

# Проверяем, включен ли уже sharding для базы данных
if mongosh --host mongos1:27017 --quiet --eval "sh.status().databases" | grep -q "news_aggregator"; then
    echo "ℹ️ Sharding already enabled for news_aggregator database"
else
    echo "Enabling sharding for news_aggregator database..."
    mongosh --host mongos1:27017 --eval '
    try {
        sh.enableSharding("news_aggregator")
        print("✅ Sharding enabled for news_aggregator");
    } catch (e) {
        print("ℹ️ Error enabling sharding:", e.message);
    }
    '
fi

sleep 5

# ============================================
# ШАГ 6: СОЗДАНИЕ ИНДЕКСОВ
# ============================================

echo "📋 Step 6: Creating indexes..."

mongosh --host mongos1:27017 --eval '
use news_aggregator

// Создаем индексы если они еще не существуют
try {
    if (!db.posts.getIndexes().some(idx => idx.name === "channel_id_hashed")) {
        db.posts.createIndex({ "channel_id": "hashed" }, { name: "channel_id_hashed" })
        print("✅ Created hashed index on channel_id");
    } else {
        print("ℹ️ Hashed index on channel_id already exists");
    }
    
    if (!db.posts.getIndexes().some(idx => idx.name === "created_at_1_post_id_1")) {
        db.posts.createIndex({ "created_at": 1, "post_id": 1 }, { name: "created_at_1_post_id_1" })
        print("✅ Created compound index on created_at and post_id");
    } else {
        print("ℹ️ Compound index already exists");
    }
} catch (e) {
    print("ℹ️ Error creating indexes:", e.message);
}
'

# ============================================
# ШАГ 7: SHARDING КОЛЛЕКЦИЙ
# ============================================

echo "📋 Step 7: Sharding collections..."

# Проверяем, шардирована ли уже коллекция posts
if mongosh --host mongos1:27017 --quiet --eval "sh.status().collections" 2>/dev/null | grep -q "news_aggregator.posts"; then
    echo "ℹ️ Collection posts is already sharded"
else
    echo "Sharding posts collection..."
    mongosh --host mongos1:27017 --eval '
    use news_aggregator
    try {
        sh.shardCollection("news_aggregator.posts", { "channel_id": "hashed" })
        print("✅ Posts collection sharded with hashed channel_id");
    } catch (e) {
        if (e.codeName === "AlreadyInitialized") {
            print("ℹ️ Collection already sharded");
        } else {
            print("⚠️ Error sharding collection:", e.message);
        }
    }
    '
fi

# ============================================
# ШАГ 8: СОЗДАНИЕ ПОЛЬЗОВАТЕЛЕЙ
# ============================================

echo "📋 Step 8: Creating users..."

mongosh --host mongos1:27017 --eval '
use admin

// Проверяем, существует ли пользователь admin
var adminExists = db.getUser("admin");
if (!adminExists) {
    db.createUser({
        user: "admin",
        pwd: "admin_password",
        roles: [ { role: "root", db: "admin" } ]
    })
    print("✅ Admin user created");
} else {
    print("ℹ️ Admin user already exists");
}

use news_aggregator

// Проверяем, существует ли пользователь news_app
var appUserExists = db.getUser("news_app");
if (!appUserExists) {
    db.createUser({
        user: "news_app",
        pwd: "app_password",
        roles: [
            { role: "readWrite", db: "news_aggregator" },
            { role: "dbAdmin", db: "news_aggregator" }
        ]
    })
    print("✅ Application user created");
} else {
    print("ℹ️ Application user already exists");
}
'

# ============================================
# ШАГ 9: ПРОВЕРКА И ВЫВОД ИНФОРМАЦИИ
# ============================================

echo "📋 Step 9: Verifying cluster setup..."

echo ""
echo "🔍 Checking cluster status..."
mongosh --host mongos1:27017 --eval '
print("=== CLUSTER STATUS ===");
sh.status();

print("\n=== SHARD DISTRIBUTION ===");
try {
    use news_aggregator;
    if (db.posts.countDocuments() > 0) {
        print("Posts collection contains " + db.posts.countDocuments() + " documents");
        db.posts.getShardDistribution();
    } else {
        print("Posts collection is empty");
    }
} catch (e) {
    print("Cannot check distribution yet:", e.message);
}

print("\n=== DATABASES ===");
show dbs;

print("\n=== CONNECTIONS ===");
db.adminCommand({ "currentOp": 1, "$all": true }).inprog.length;
'

echo ""
echo "✅ ============================================"
echo "✅ MongoDB Sharded Cluster Initialization Complete!"
echo "✅ ============================================"
echo ""
echo "📊 Summary:"
echo "  • Config Servers: 3-node replica set (configRS)"
echo "  • Shards: 2 shards (shard0RS, shard1RS)"
echo "  • Mongos Routers: 2 instances"
echo "  • Database: news_aggregator (sharded)"
echo "  • Sharded Collections: posts"
echo "  • Shard Key: channel_id (hashed)"
echo ""
echo "🔌 Connection Strings:"
echo "  • For application: mongodb://news_app:app_password@localhost:27027/news_aggregator"
echo "  • For admin: mongodb://admin:admin_password@localhost:27027/admin"
echo ""
echo "🔧 Commands to verify:"
echo "  1. docker exec -it mongos1 mongosh --port 27017"
echo "  2. sh.status()"
echo "  3. use news_aggregator; db.posts.getShardDistribution()"
echo ""
echo "🎉 Ready for production use!"