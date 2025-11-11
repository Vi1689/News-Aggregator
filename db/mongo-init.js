// Инициализация MongoDB
db = db.getSiblingDB('news_aggregator');

// Создаем пользователя для приложения
db.createUser({
  user: "news_app",
  pwd: "app_password",
  roles: [
    {
      role: "readWrite",
      db: "news_aggregator"
    }
  ]
});

// Создаем коллекцию posts
db.createCollection("posts");

// Создаем текстовый индекс для поиска
db.posts.createIndex(
  {
    "title": "text",
    "content": "text", 
    "tags": "text"
  },
  {
    name: "text_search",
    weights: {
      "title": 10,
      "content": 5,
      "tags": 3
    },
    default_language: "russian"
  }
);

// Создаем индекс для дедубликации
db.posts.createIndex(
  { "content_hash": 1 },
  { name: "content_hash_idx" }
);

// Создаем индекс для быстрого поиска по post_id
db.posts.createIndex(
  { "post_id": 1 },
  { name: "post_id_idx", unique: true }
);

print("MongoDB initialized successfully");
print("- Created database: news_aggregator");
print("- Created user: news_app");
print("- Created collection: posts");
print("- Created indexes: text_search, content_hash_idx, post_id_idx");