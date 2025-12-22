// Подключись через mongosh к mongos: docker exec -it mongos1 mongosh --port 27017

db = db.getSiblingDB('news_aggregator');

// ================= СОЗДАНИЕ ПОЛЬЗОВАТЕЛЕЙ =================
db.createUser({
  user: "news_app",
  pwd: "app_password",
  roles: [
    { role: "readWrite", db: "news_aggregator" },
    { role: "dbAdmin", db: "news_aggregator" }
  ]
});

db.createUser({
  user: "admin",
  pwd: "admin_password",
  roles: [ { role: "root", db: "admin" } ]
});

print("✅ Users created");

// ================= СОЗДАНИЕ КОЛЛЕКЦИЙ =================
db.createCollection("posts", {
  validator: {
    $jsonSchema: {
      bsonType: "object",
      required: ["post_id", "title", "content", "channel_id", "created_at"],
      properties: {
        post_id: { bsonType: "int" },
        title: { bsonType: "string", minLength: 3, maxLength: 500 },
        content: { bsonType: "string", minLength: 10 },
        channel_id: { bsonType: "int" },
        author_id: { bsonType: "int" },
        tags: { bsonType: "array", items: { bsonType: "string" } },
        stats: {
          bsonType: "object",
          properties: {
            views: { bsonType: "int", minimum: 0 },
            likes: { bsonType: "int", minimum: 0 },
            shares: { bsonType: "int", minimum: 0 }
          }
        },
        created_at: { bsonType: "date" },
        updated_at: { bsonType: "date" }
      }
    }
  }
});

db.createCollection("user_interactions");
db.createCollection("channels");
db.createCollection("tags");
db.createCollection("top_posts_view");
db.createCollection("cached_channel_reports");

print("✅ Collections created");

// ================= ИНДЕКСЫ =================
// Для шардированных коллекций индексы создаются ПОСЛЕ шардинга

print("⚠️ Note: Indexes will be created AFTER sharding is configured");