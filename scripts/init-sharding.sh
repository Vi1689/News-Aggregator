#!/bin/bash
# init-sharding.sh - Скрипт инициализации MongoDB Sharded Cluster

set -e

echo "🚀 Starting MongoDB Sharding Cluster Initialization..."

# Ждем запуска всех серверов
sleep 20

echo "📋 Step 1: Initializing Config Server Replica Set..."
mongosh --host config1:27019 --eval '
rs.initiate({
  _id: "configRS",
  configsvr: true,
  members: [
    { _id: 0, host: "config1:27019" },
    { _id: 1, host: "config2:27019" },
    { _id: 2, host: "config3:27019" }
  ]
})
'

echo "⏳ Waiting for config servers to sync..."
sleep 10

echo "📋 Step 2: Initializing Shard 0 Replica Set..."
mongosh --host shard0-primary:27018 --eval '
rs.initiate({
  _id: "shard0RS",
  members: [
    { _id: 0, host: "shard0-primary:27018" },
    { _id: 1, host: "shard0-secondary:27018" }
  ]
})
'

echo "📋 Step 3: Initializing Shard 1 Replica Set..."
mongosh --host shard1-primary:27018 --eval '
rs.initiate({
  _id: "shard1RS",
  members: [
    { _id: 0, host: "shard1-primary:27018" },
    { _id: 1, host: "shard1-secondary:27018" }
  ]
})
'

echo "⏳ Waiting for shards to elect primaries..."
sleep 15

echo "📋 Step 4: Adding Shards to Cluster..."
mongosh --host mongos1:27017 --eval '
sh.addShard("shard0RS/shard0-primary:27018,shard0-secondary:27018")
sh.addShard("shard1RS/shard1-primary:27018,shard1-secondary:27018")
'

sleep 5

echo "📋 Step 5: Enabling Sharding for Database..."
mongosh --host mongos1:27017 --eval '
sh.enableSharding("news_aggregator")
'

echo "📋 Step 6: Creating Indexes for Shard Keys..."
mongosh --host mongos1:27017 --eval '
use news_aggregator

// Хеш-индекс для channel_id
db.posts.createIndex({ "channel_id": "hashed" })

// Альтернативный индекс для range-based sharding
db.posts.createIndex({ "created_at": 1, "post_id": 1 })
'

echo "📋 Step 7: Sharding Collections..."
mongosh --host mongos1:27017 --eval '
use news_aggregator

// Шардирование posts по channel_id (hash)
sh.shardCollection("news_aggregator.posts", { "channel_id": "hashed" })

// Опционально: шардирование user_interactions
db.user_interactions.createIndex({ "user_id": "hashed" })
sh.shardCollection("news_aggregator.user_interactions", { "user_id": "hashed" })
'

echo "📋 Step 8: Creating Users and Setting Permissions..."
mongosh --host mongos1:27017 --eval '
use admin
db.createUser({
  user: "admin",
  pwd: "admin_password",
  roles: [ { role: "root", db: "admin" } ]
})

use news_aggregator
db.createUser({
  user: "news_app",
  pwd: "app_password",
  roles: [
    { role: "readWrite", db: "news_aggregator" },
    { role: "dbAdmin", db: "news_aggregator" }
  ]
})
'

echo "📋 Step 9: Inserting Test Data..."
mongosh --host mongos1:27017 --eval '
use news_aggregator

// Каналы
db.channels.insertMany([
  { channel_id: 1, name: "Tech News", source_id: 1, subscribers_count: 10000, topic: "технологии", post_count: 0 },
  { channel_id: 2, name: "AI Daily", source_id: 1, subscribers_count: 15000, topic: "AI", post_count: 0 },
  { channel_id: 3, name: "Science Hub", source_id: 2, subscribers_count: 8000, topic: "наука", post_count: 0 }
])

// Теги
db.tags.insertMany([
  { tag_id: 1, name: "технологии", usage_count: 0, created_at: new Date() },
  { tag_id: 2, name: "AI", usage_count: 0, created_at: new Date() },
  { tag_id: 3, name: "инновации", usage_count: 0, created_at: new Date() },
  { tag_id: 4, name: "наука", usage_count: 0, created_at: new Date() }
])

// Посты (распределятся по шардам)
for (let i = 1; i <= 1000; i++) {
  db.posts.insertOne({
    post_id: i,
    title: "Пост номер " + i,
    content: "Содержание поста " + i + " ".repeat(50),
    channel_id: (i % 3) + 1,
    author_id: (i % 10) + 1,
    tags: ["технологии", i % 2 === 0 ? "AI" : "инновации"],
    comments: [],
    stats: {
      views: Math.floor(Math.random() * 1000),
      likes: Math.floor(Math.random() * 100),
      shares: Math.floor(Math.random() * 20)
    },
    created_at: new Date(Date.now() - Math.random() * 30 * 24 * 60 * 60 * 1000),
    updated_at: new Date()
  })
}

print("✅ Inserted 1000 test posts")
'

echo "📋 Step 10: Verifying Shard Distribution..."
mongosh --host mongos1:27017 --eval '
use news_aggregator
db.posts.getShardDistribution()
'

echo "📋 Step 11: Checking Cluster Status..."
mongosh --host mongos1:27017 --eval '
sh.status()
'

echo ""
echo "✅ ============================================"
echo "✅ MongoDB Sharded Cluster Setup Complete!"
echo "✅ ============================================"
echo ""
echo "📊 Cluster Information:"
echo "  - Config Servers: 3 (configRS)"
echo "  - Shards: 2 (shard0RS, shard1RS)"
echo "  - Mongos Routers: 2"
echo "  - Sharded Collections: posts, user_interactions"
echo "  - Shard Key: channel_id (hashed)"
echo ""
echo "🔌 Connection Strings:"
echo "  - Mongos 1: mongodb://news_app:app_password@localhost:27017/news_aggregator"
echo "  - Mongos 2: mongodb://news_app:app_password@localhost:27026/news_aggregator"
echo ""
echo "📝 Useful Commands:"
echo "  - Check status: sh.status()"
echo "  - Check distribution: db.posts.getShardDistribution()"
echo "  - Check config: sh.getShardedDataDistribution()"
echo ""
echo "🎉 Ready to use!"