// ============================================
// 1. СОЗДАНИЕ ПОЛЬЗОВАТЕЛЕЙ В БАЗЕ ADMIN
// ============================================
print("🔐 Creating users in admin database...");

// Переключаемся на базу admin для создания пользователя с правами мониторинга
db = db.getSiblingDB('admin');

// Пользователь для мониторинга (создается в admin базе)
db.createUser({
  user: "monitor",
  pwd: "monitor_pass",
  roles: [
    { role: "clusterMonitor", db: "admin" },
    { role: "readAnyDatabase", db: "admin" }
  ]
});

print("✅ Monitor user created: monitor in admin database");

// Root администратор (если еще не создан)
if (!db.getUser("admin")) {
  db.createUser({
    user: "admin",
    pwd: "mongopass",
    roles: [{ role: "root", db: "admin" }]
  });
  print("✅ Admin user created: admin");
}


// Инициализация MongoDB для News Aggregator
db = db.getSiblingDB('news_aggregator');

// ============================================
// СОЗДАНИЕ ПОЛЬЗОВАТЕЛЯ
// ============================================
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

print("✅ User created: news_app");

// ============================================
// КОЛЛЕКЦИЯ 1 (Основная): POSTS
// ============================================
db.createCollection("posts", {
  validator: {
    $jsonSchema: {
      bsonType: "object",
      required: ["post_id", "title", "content", "created_at"],
      properties: {
        post_id: {
          bsonType: "int",
          description: "Unique post identifier from PostgreSQL"
        },
        title: {
          bsonType: "string",
          description: "Post title - required"
        },
        content: {
          bsonType: "string",
          description: "Post content - required"
        },
        content_hash: {
          bsonType: "string",
          description: "Hash for deduplication"
        },
        tags: {
          bsonType: "array",
          description: "Array of tags",
          items: {
            bsonType: "string"
          }
        },
        author_id: {
          bsonType: "int",
          description: "Author identifier"
        },
        channel_id: {
          bsonType: "int",
          description: "Channel identifier"
        },
        stats: {
          bsonType: "object",
          properties: {
            views: { bsonType: "int" },
            likes: { bsonType: "int" },
            comments: { bsonType: "int" }
          }
        },
        created_at: {
          bsonType: "date",
          description: "Creation timestamp"
        },
        updated_at: {
          bsonType: "date",
          description: "Last update timestamp"
        }
      }
    }
  }
});

print("✅ Collection created: posts");

// ============================================
// КОЛЛЕКЦИЯ 2 (Вспомогательная): USER_INTERACTIONS
// ============================================
db.createCollection("user_interactions", {
  validator: {
    $jsonSchema: {
      bsonType: "object",
      required: ["user_id", "post_id", "action", "timestamp"],
      properties: {
        user_id: {
          bsonType: "string",
          description: "User identifier"
        },
        post_id: {
          bsonType: "int",
          description: "Post identifier"
        },
        action: {
          bsonType: "string",
          enum: ["view", "like", "comment", "share"],
          description: "Type of interaction"
        },
        timestamp: {
          bsonType: "date",
          description: "When the interaction occurred"
        },
        metadata: {
          bsonType: "object",
          description: "Additional interaction data"
        }
      }
    }
  }
});

print("✅ Collection created: user_interactions");

// ============================================
// КОЛЛЕКЦИЯ 3 (Витрина): TOP_POSTS_VIEW
// ============================================
db.createCollection("top_posts_view");

print("✅ Collection created: top_posts_view (materialized view)");

// ============================================
// ИНДЕКСЫ ДЛЯ POSTS
// ============================================

// 1. Текстовый индекс для полнотекстового поиска (с весами)
db.posts.createIndex(
  {
    "title": "text",
    "content": "text", 
    "tags": "text"
  },
  {
    name: "text_search_idx",
    weights: {
      "title": 10,    // Заголовок важнее всего
      "content": 5,   // Контент средней важности
      "tags": 3       // Теги наименее важны
    },
    default_language: "russian"
  }
);

print("✅ Index created: text_search_idx (text)");

// 2. Unique индекс для post_id
db.posts.createIndex(
  { "post_id": 1 },
  { name: "post_id_unique_idx", unique: true }
);

print("✅ Index created: post_id_unique_idx (unique)");

// 3. Unique индекс для дедубликации по content_hash
db.posts.createIndex(
  { "content_hash": 1 },
  { name: "content_hash_idx", unique: true, sparse: true }
);

print("✅ Index created: content_hash_idx (unique, sparse)");

// 4. Составной индекс для поиска по тегам с сортировкой
db.posts.createIndex(
  { 
    "tags": 1,
    "stats.likes": -1,
    "created_at": -1 
  },
  { name: "tags_popularity_idx" }
);

print("✅ Index created: tags_popularity_idx (compound)");

// 5. Multikey индекс по массиву тегов
db.posts.createIndex(
  { "tags": 1 },
  { name: "tags_array_idx" }
);

print("✅ Index created: tags_array_idx (multikey)");

// 6. Partial индекс для популярных постов (likes >= 10)
db.posts.createIndex(
  { "stats.likes": -1 },
  { 
    name: "popular_posts_idx",
    partialFilterExpression: { "stats.likes": { $gte: 10 } }
  }
);

print("✅ Index created: popular_posts_idx (partial)");

// 7. TTL индекс - автоматически удаляет посты старше 1 года
db.posts.createIndex(
  { "created_at": 1 },
  { 
    name: "posts_ttl_idx",
    expireAfterSeconds: 31536000  // 365 дней
  }
);

print("✅ Index created: posts_ttl_idx (TTL - 365 days)");

// 8. Составной индекс для аналитики автора
db.posts.createIndex(
  { 
    "author_id": 1,
    "created_at": -1 
  },
  { name: "author_analytics_idx" }
);

print("✅ Index created: author_analytics_idx (compound)");

// 9. Индекс для статистики
db.posts.createIndex(
  { 
    "stats.views": -1,
    "stats.likes": -1,
    "stats.comments": -1
  },
  { name: "stats_analytics_idx" }
);

print("✅ Index created: stats_analytics_idx (compound)");

// ============================================
// ИНДЕКСЫ ДЛЯ USER_INTERACTIONS
// ============================================

// 1. Compound index для истории пользователя
db.user_interactions.createIndex(
  { 
    "user_id": 1,
    "timestamp": -1 
  },
  { name: "user_history_idx" }
);

print("✅ Index created: user_history_idx (compound)");

// 2. Index для поиска по посту
db.user_interactions.createIndex(
  { "post_id": 1 },
  { name: "post_interactions_idx" }
);

print("✅ Index created: post_interactions_idx");

// 3. TTL index - автоматически удаляет взаимодействия старше 90 дней
db.user_interactions.createIndex(
  { "timestamp": 1 },
  { 
    name: "interactions_ttl_idx",
    expireAfterSeconds: 7776000  // 90 дней
  }
);

print("✅ Index created: interactions_ttl_idx (TTL - 90 days)");

// 4. Compound index для аналитики действий
db.user_interactions.createIndex(
  { 
    "action": 1,
    "timestamp": -1 
  },
  { name: "action_analytics_idx" }
);

print("✅ Index created: action_analytics_idx (compound)");

// ============================================
// ИНДЕКСЫ ДЛЯ TOP_POSTS_VIEW
// ============================================

db.top_posts_view.createIndex(
  { "total_score": -1 },
  { name: "total_score_idx" }
);

print("✅ Index created: total_score_idx");

db.top_posts_view.createIndex(
  { "post_id": 1 },
  { name: "view_post_id_idx" }
);

print("✅ Index created: view_post_id_idx");

// ============================================
// ТЕСТОВЫЕ ДАННЫЕ
// ============================================

print("\n📝 Inserting test data...");

// Добавляем несколько тестовых постов
db.posts.insertMany([
  {
    post_id: 1,
    title: "Первая новость о технологиях",
    content: "Это пример контента первой новости о технологиях и инновациях",
    content_hash: "hash_" + Math.random(),
    tags: ["технологии", "инновации", "новости"],
    author_id: 1,
    channel_id: 1,
    stats: { views: 150, likes: 25, comments: 5 },
    created_at: new Date(),
    updated_at: new Date()
  },
  {
    post_id: 2,
    title: "Спортивные достижения",
    content: "Обзор последних спортивных событий и рекордов",
    content_hash: "hash_" + Math.random(),
    tags: ["спорт", "новости", "рекорды"],
    author_id: 2,
    channel_id: 1,
    stats: { views: 200, likes: 40, comments: 8 },
    created_at: new Date(),
    updated_at: new Date()
  },
  {
    post_id: 3,
    title: "Политические новости",
    content: "Важные политические события недели",
    content_hash: "hash_" + Math.random(),
    tags: ["политика", "новости"],
    author_id: 1,
    channel_id: 2,
    stats: { views: 300, likes: 15, comments: 12 },
    created_at: new Date(),
    updated_at: new Date()
  }
]);

print("✅ Test posts inserted: 3");

// Добавляем тестовые взаимодействия
db.user_interactions.insertMany([
  {
    user_id: "user_1",
    post_id: 1,
    action: "view",
    timestamp: new Date(),
    metadata: { device: "mobile" }
  },
  {
    user_id: "user_1",
    post_id: 1,
    action: "like",
    timestamp: new Date(),
    metadata: { device: "mobile" }
  },
  {
    user_id: "user_2",
    post_id: 2,
    action: "view",
    timestamp: new Date(),
    metadata: { device: "desktop" }
  }
]);

print("✅ Test interactions inserted: 3");

// ============================================
// ФИНАЛЬНАЯ ПРОВЕРКА
// ============================================

print("\n📊 Database Statistics:");
print("Posts count: " + db.posts.countDocuments());
print("Interactions count: " + db.user_interactions.countDocuments());
print("\n✨ MongoDB initialized successfully!");
print("============================================");
print("Database: news_aggregator");
print("User: news_app");
print("Collections: 3 (posts, user_interactions, top_posts_view)");
print("Indexes: 15 total");
print("  - posts: 9 indexes (text, unique, compound, partial, TTL)");
print("  - user_interactions: 4 indexes (compound, TTL)");
print("  - top_posts_view: 2 indexes");
print("============================================");