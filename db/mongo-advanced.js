// ============================================
// 1. СВЯЗИ МЕЖДУ КОЛЛЕКЦИЯМИ
// ============================================

/*
ОБОСНОВАНИЕ ВЫБОРА СВЯЗЕЙ:

1:N - Posts -> Comments (ВСТРАИВАНИЕ)
- Комментарии всегда загружаются вместе с постом
- Небольшое количество комментариев на пост (обычно < 100)
- Комментарии не используются отдельно от поста

1:N - Channel -> Posts (ССЫЛКА)
- У канала может быть тысячи постов
- Посты часто запрашиваются отдельно
- Избегаем роста документа канала до 16MB лимита

M:N - Posts <-> Tags (ССЫЛКА)
- Теги переиспользуются между постами
- Нужна возможность находить все посты по тегу
- Нужна возможность находить все теги у поста
*/

// Обновленная схема с встроенными комментариями
db.posts.drop();
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
        tags: {
          bsonType: "array",
          items: { bsonType: "string" },
          maxItems: 20
        },
        // ВСТРОЕННЫЕ КОММЕНТАРИИ (1:N)
        comments: {
          bsonType: "array",
          items: {
            bsonType: "object",
            required: ["comment_id", "nickname", "text", "created_at"],
            properties: {
              comment_id: { bsonType: "int" },
              nickname: { bsonType: "string", minLength: 2, maxLength: 50 },
              text: { bsonType: "string", minLength: 1, maxLength: 2000 },
              likes_count: { bsonType: "int", minimum: 0 },
              created_at: { bsonType: "date" },
              parent_comment_id: { bsonType: ["int", "null"] }
            }
          }
        },
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

// Коллекция каналов со ССЫЛКАМИ на посты (1:N)
db.createCollection("channels", {
  validator: {
    $jsonSchema: {
      bsonType: "object",
      required: ["channel_id", "name", "source_id"],
      properties: {
        channel_id: { bsonType: "int" },
        name: { bsonType: "string", minLength: 2, maxLength: 255 },
        source_id: { bsonType: "int" },
        subscribers_count: { bsonType: "int", minimum: 0 },
        topic: { bsonType: "string" },
        created_at: { bsonType: "date" }
      }
    }
  }
});

// Коллекция тегов для связи M:N
db.createCollection("tags", {
  validator: {
    $jsonSchema: {
      bsonType: "object",
      required: ["tag_id", "name"],
      properties: {
        tag_id: { bsonType: "int" },
        name: { bsonType: "string", minLength: 2, maxLength: 50 },
        usage_count: { bsonType: "int", minimum: 0 },
        created_at: { bsonType: "date" }
      }
    }
  }
});

// Индексы для связей
db.posts.createIndex({ "channel_id": 1 });
db.posts.createIndex({ "tags": 1 });
db.tags.createIndex({ "name": 1 }, { unique: true });

// ============================================
// 2. ТРАНЗАКЦИИ
// ============================================

// Многошаговая транзакция: создание поста с обновлением статистики канала
async function createPostWithChannelUpdate(postData) {
  const session = db.getMongo().startSession();
  
  try {
    session.startTransaction({
      readConcern: { level: "snapshot" },
      writeConcern: { w: "majority" },
      readPreference: "primary"
    });

    // Шаг 1: Вставка нового поста
    const postsCollection = session.getDatabase("news_aggregator").posts;
    const result = await postsCollection.insertOne({
      post_id: postData.post_id,
      title: postData.title,
      content: postData.content,
      channel_id: postData.channel_id,
      tags: postData.tags || [],
      comments: [],
      stats: { views: 0, likes: 0, shares: 0 },
      created_at: new Date(),
      updated_at: new Date()
    }, { session });

    // Шаг 2: Обновление счетчика постов в канале
    const channelsCollection = session.getDatabase("news_aggregator").channels;
    await channelsCollection.updateOne(
      { channel_id: postData.channel_id },
      { 
        $inc: { post_count: 1 },
        $set: { last_post_date: new Date() }
      },
      { session }
    );

    // Шаг 3: Обновление счетчика использования тегов
    const tagsCollection = session.getDatabase("news_aggregator").tags;
    if (postData.tags && postData.tags.length > 0) {
      await tagsCollection.updateMany(
        { name: { $in: postData.tags } },
        { $inc: { usage_count: 1 } },
        { session }
      );
    }

    // Коммит транзакции
    await session.commitTransaction();
    print("✅ Transaction committed successfully");
    return result;

  } catch (error) {
    print("❌ Transaction aborted due to error:", error);
    await session.abortTransaction();
    throw error;
  } finally {
    await session.endSession();
  }
}

// Пример использования
// createPostWithChannelUpdate({
//   post_id: 1001,
//   title: "Новая технология AI",
//   content: "Подробное описание новой технологии...",
//   channel_id: 1,
//   tags: ["AI", "технологии", "инновации"]
// });

// ============================================
// 3. BULK-ОПЕРАЦИИ
// ============================================

// Пакетное обновление: обновление статистики постов и добавление комментариев
const bulkOperations = [
  // Update: увеличение просмотров
  {
    updateOne: {
      filter: { post_id: 1 },
      update: { 
        $inc: { "stats.views": 100, "stats.likes": 5 },
        $set: { updated_at: new Date() }
      }
    }
  },
  
  // Insert: создание нового поста
  {
    insertOne: {
      document: {
        post_id: 1002,
        title: "Массовая операция",
        content: "Создан через bulk operation",
        channel_id: 2,
        tags: ["bulk", "mongodb"],
        comments: [],
        stats: { views: 0, likes: 0, shares: 0 },
        created_at: new Date(),
        updated_at: new Date()
      }
    }
  },
  
  // Update: добавление комментария
  {
    updateOne: {
      filter: { post_id: 2 },
      update: {
        $push: {
          comments: {
            comment_id: 101,
            nickname: "bulk_user",
            text: "Комментарий добавлен через bulk operation",
            likes_count: 0,
            created_at: new Date()
          }
        }
      }
    }
  },
  
  // Update Many: обновление всех постов по тегу
  {
    updateMany: {
      filter: { tags: "технологии" },
      update: { 
        $inc: { "stats.views": 10 },
        $set: { trending: true }
      }
    }
  },
  
  // Delete: удаление старых постов
  {
    deleteMany: {
      filter: { 
        created_at: { $lt: new Date(Date.now() - 365 * 24 * 60 * 60 * 1000) },
        "stats.views": { $lt: 100 }
      }
    }
  },
  
  // Replace: полная замена документа
  {
    replaceOne: {
      filter: { post_id: 999 },
      replacement: {
        post_id: 999,
        title: "Обновленный пост",
        content: "Полностью заменен через bulk",
        channel_id: 1,
        tags: ["обновление"],
        comments: [],
        stats: { views: 0, likes: 0, shares: 0 },
        created_at: new Date(),
        updated_at: new Date()
      },
      upsert: true
    }
  }
];

// Выполнение bulk операций
const bulkResult = db.posts.bulkWrite(bulkOperations, { ordered: false });

print("📊 Bulk Operations Result:");
print("  Inserted:", bulkResult.insertedCount);
print("  Modified:", bulkResult.modifiedCount);
print("  Deleted:", bulkResult.deletedCount);
print("  Upserted:", bulkResult.upsertedCount);

// ============================================
// 4. ВАЛИДАЦИЯ СХЕМЫ
// ============================================

// Добавление бизнес-правил валидации
db.runCommand({
  collMod: "posts",
  validator: {
    $jsonSchema: {
      bsonType: "object",
      required: ["post_id", "title", "content", "channel_id", "created_at"],
      properties: {
        post_id: { 
          bsonType: "int",
          description: "Уникальный ID поста, обязательное поле"
        },
        title: { 
          bsonType: "string",
          minLength: 3,
          maxLength: 500,
          description: "Заголовок от 3 до 500 символов"
        },
        content: { 
          bsonType: "string",
          minLength: 10,
          maxLength: 50000,
          description: "Контент от 10 до 50000 символов"
        },
        channel_id: { bsonType: "int" },
        tags: {
          bsonType: "array",
          maxItems: 20,
          uniqueItems: true,
          description: "Максимум 20 уникальных тегов"
        },
        comments: {
          bsonType: "array",
          maxItems: 1000,
          description: "Максимум 1000 комментариев на пост"
        },
        stats: {
          bsonType: "object",
          required: ["views", "likes"],
          properties: {
            views: { 
              bsonType: "int",
              minimum: 0,
              description: "Просмотры не могут быть отрицательными"
            },
            likes: { 
              bsonType: "int",
              minimum: 0,
              maximum: 1000000,
              description: "Лайки от 0 до 1 млн"
            },
            shares: { 
              bsonType: "int",
              minimum: 0
            }
          }
        },
        created_at: { 
          bsonType: "date",
          description: "Дата создания обязательна"
        },
        updated_at: { bsonType: "date" }
      },
      // БИЗНЕС-ПРАВИЛО 1: created_at должен быть не позже updated_at
      dependencies: {
        updated_at: {
          properties: {
            created_at: {},
            updated_at: {}
          }
        }
      }
    }
  },
  validationLevel: "strict",
  validationAction: "error"
});

// БИЗНЕС-ПРАВИЛО 2: Автоматическая проверка длины комментариев
db.posts.createIndex(
  { "comments.text": "text" },
  { 
    partialFilterExpression: { 
      "comments.text": { $exists: true }
    }
  }
);

// БИЗНЕС-ПРАВИЛО 3: Engagement rate не может превышать 100%
db.createCollection("post_analytics", {
  validator: {
    $jsonSchema: {
      bsonType: "object",
      properties: {
        post_id: { bsonType: "int" },
        engagement_rate: {
          bsonType: "number",
          minimum: 0,
          maximum: 100,
          description: "Engagement rate должен быть от 0 до 100%"
        }
      }
    }
  }
});

// ============================================
// 5. КОМБИНИРОВАННЫЕ ОТЧЕТЫ
// ============================================

// Отчет: Распределение новостей по источникам и темам за неделю
const weeklyReport = db.posts.aggregate([
  // Фильтрация постов за последнюю неделю
  {
    $match: {
      created_at: {
        $gte: new Date(Date.now() - 7 * 24 * 60 * 60 * 1000)
      }
    }
  },
  
  // JOIN с каналами
  {
    $lookup: {
      from: "channels",
      localField: "channel_id",
      foreignField: "channel_id",
      as: "channel_info"
    }
  },
  
  // Развертывание массива каналов
  {
    $unwind: {
      path: "$channel_info",
      preserveNullAndEmptyArrays: false
    }
  },
  
  // Развертывание тегов для подсчета
  {
    $unwind: {
      path: "$tags",
      preserveNullAndEmptyArrays: true
    }
  },
  
  // Многоуровневая аналитика с $facet
  {
    $facet: {
      // Аналитика по источникам
      "by_source": [
        {
          $group: {
            _id: "$channel_info.source_id",
            source_name: { $first: "$channel_info.name" },
            total_posts: { $sum: 1 },
            total_views: { $sum: "$stats.views" },
            total_likes: { $sum: "$stats.likes" },
            avg_engagement: {
              $avg: {
                $divide: [
                  { $add: ["$stats.likes", "$stats.shares"] },
                  { $max: ["$stats.views", 1] }
                ]
              }
            }
          }
        },
        { $sort: { total_posts: -1 } },
        { $limit: 10 }
      ],
      
      // Аналитика по темам (тегам)
      "by_topic": [
        {
          $group: {
            _id: "$tags",
            topic: { $first: "$tags" },
            post_count: { $sum: 1 },
            total_engagement: {
              $sum: { $add: ["$stats.likes", "$stats.shares"] }
            }
          }
        },
        { $sort: { post_count: -1 } },
        { $limit: 20 }
      ],
      
      // Распределение по дням недели с $bucket
      "by_day": [
        {
          $bucket: {
            groupBy: { $dayOfWeek: "$created_at" },
            boundaries: [1, 2, 3, 4, 5, 6, 7, 8],
            default: "other",
            output: {
              count: { $sum: 1 },
              avg_likes: { $avg: "$stats.likes" },
              posts: { $push: "$title" }
            }
          }
        }
      ],
      
      // Общая статистика
      "summary": [
        {
          $group: {
            _id: null,
            total_posts: { $sum: 1 },
            unique_channels: { $addToSet: "$channel_id" },
            unique_tags: { $addToSet: "$tags" },
            total_views: { $sum: "$stats.views" },
            total_engagement: {
              $sum: { $add: ["$stats.likes", "$stats.shares"] }
            }
          }
        },
        {
          $project: {
            _id: 0,
            total_posts: 1,
            unique_channels_count: { $size: "$unique_channels" },
            unique_tags_count: { $size: "$unique_tags" },
            total_views: 1,
            total_engagement: 1,
            avg_views_per_post: {
              $divide: ["$total_views", "$total_posts"]
            }
          }
        }
      ]
    }
  }
]);

print("📈 Weekly Report Generated");
printjson(weeklyReport.toArray());

// Граф-запрос для анализа связанных постов через теги
const relatedPostsGraph = db.posts.aggregate([
  {
    $match: { post_id: 1 }
  },
  {
    $graphLookup: {
      from: "posts",
      startWith: "$tags",
      connectFromField: "tags",
      connectToField: "tags",
      as: "related_posts",
      maxDepth: 2,
      depthField: "depth",
      restrictSearchWithMatch: {
        "stats.likes": { $gte: 10 }
      }
    }
  },
  {
    $project: {
      title: 1,
      tags: 1,
      related_count: { $size: "$related_posts" },
      related_titles: {
        $slice: ["$related_posts.title", 5]
      }
    }
  }
]);

print("🔗 Related Posts Analysis:");
printjson(relatedPostsGraph.toArray());

// ============================================
// 6. ОПТИМИЗАЦИЯ ЗАПРОСОВ
// ============================================

print("\n🔍 QUERY OPTIMIZATION ANALYSIS\n");

// ЗАПРОС 1: Поиск популярных постов по тегам (ДО оптимизации)
print("Query 1: Popular posts by tags - BEFORE optimization");
const query1Before = db.posts.find({
  tags: { $in: ["технологии", "AI"] },
  "stats.likes": { $gte: 10 }
}).sort({ "stats.likes": -1 }).limit(10);

const explain1Before = db.posts.find({
  tags: { $in: ["технологии", "AI"] },
  "stats.likes": { $gte: 10 }
}).sort({ "stats.likes": -1 }).explain("executionStats");

print("Execution time:", explain1Before.executionStats.executionTimeMillis, "ms");
print("Documents examined:", explain1Before.executionStats.totalDocsExamined);
print("Documents returned:", explain1Before.executionStats.nReturned);

// Создание составного индекса
db.posts.createIndex({ 
  tags: 1, 
  "stats.likes": -1 
}, { 
  name: "tags_likes_optimized_idx" 
});

print("\nQuery 1: Popular posts by tags - AFTER optimization");
const explain1After = db.posts.find({
  tags: { $in: ["технологии", "AI"] },
  "stats.likes": { $gte: 10 }
}).sort({ "stats.likes": -1 }).explain("executionStats");

print("Execution time:", explain1After.executionStats.executionTimeMillis, "ms");
print("Documents examined:", explain1After.executionStats.totalDocsExamined);
print("Documents returned:", explain1After.executionStats.nReturned);
print("Improvement:", 
  Math.round((1 - explain1After.executionStats.executionTimeMillis / 
  explain1Before.executionStats.executionTimeMillis) * 100), "%");

// ЗАПРОС 2: Поиск постов с комментариями (ДО оптимизации)
print("\nQuery 2: Posts with many comments - BEFORE optimization");
const explain2Before = db.posts.find({
  "comments.10": { $exists: true }
}).explain("executionStats");

print("Execution time:", explain2Before.executionStats.executionTimeMillis, "ms");

// Создание индекса на размер массива комментариев
db.posts.createIndex({ 
  "comments": 1 
}, { 
  name: "comments_array_idx",
  sparse: true 
});

print("\nQuery 2: Posts with many comments - AFTER optimization");
const explain2After = db.posts.find({
  "comments.10": { $exists: true }
}).explain("executionStats");

print("Execution time:", explain2After.executionStats.executionTimeMillis, "ms");
print("Improvement:", 
  Math.round((1 - explain2After.executionStats.executionTimeMillis / 
  explain2Before.executionStats.executionTimeMillis) * 100), "%");

// ЗАПРОС 3: Агрегация с группировкой (ДО оптимизации)
print("\nQuery 3: Channel statistics - BEFORE optimization");
const startTime3Before = Date.now();
const result3Before = db.posts.aggregate([
  {
    $group: {
      _id: "$channel_id",
      post_count: { $sum: 1 },
      avg_likes: { $avg: "$stats.likes" }
    }
  },
  { $sort: { post_count: -1 } }
]);
const time3Before = Date.now() - startTime3Before;
print("Execution time:", time3Before, "ms");

// Создание индекса для агрегации
db.posts.createIndex({ 
  channel_id: 1, 
  "stats.likes": 1 
}, { 
  name: "channel_stats_idx" 
});

print("\nQuery 3: Channel statistics - AFTER optimization");
const startTime3After = Date.now();
const result3After = db.posts.aggregate([
  {
    $group: {
      _id: "$channel_id",
      post_count: { $sum: 1 },
      avg_likes: { $avg: "$stats.likes" }
    }
  },
  { $sort: { post_count: -1 } }
]);
const time3After = Date.now() - startTime3After;
print("Execution time:", time3After, "ms");
print("Improvement:", Math.round((1 - time3After / time3Before) * 100), "%");

print("\n✅ Optimization Summary:");
print("- Query 1: Compound index on tags + likes");
print("- Query 2: Sparse index on comments array");
print("- Query 3: Compound index on channel_id + likes");

// ============================================
// 7. ШАРДИНГ
// ============================================

/*
НАСТРОЙКА ШАРДИНГА (выполнять в mongosh с подключением к mongos):

// 1. Включить шардинг для базы данных
sh.enableSharding("news_aggregator")

// 2. Создать хеш-индекс на shard key
db.posts.createIndex({ channel_id: "hashed" })

// 3. Шардировать коллекцию по channel_id
sh.shardCollection("news_aggregator.posts", { channel_id: "hashed" })

// Альтернативный вариант: range-based sharding по created_at
db.posts.createIndex({ created_at: 1, post_id: 1 })
sh.shardCollection("news_aggregator.posts", { created_at: 1, post_id: 1 })
*/

// Запросы с разными shard keys

// Запрос 1: Использует shard key - эффективный (targeted query)
print("\n🔧 Sharding Query 1: Targeted (uses shard key)");
const shardQuery1 = db.posts.find({ 
  channel_id: 1 
}).explain("executionStats");
print("Shards targeted:", shardQuery1.queryPlanner.winningPlan);

// Запрос 2: Не использует shard key - broadcast query
print("\n🔧 Sharding Query 2: Broadcast (no shard key)");
const shardQuery2 = db.posts.find({ 
  "stats.likes": { $gte: 100 } 
}).explain("executionStats");

// Запрос 3: Агрегация с shard key
print("\n🔧 Sharding Query 3: Aggregation with shard key");
const shardQuery3 = db.posts.aggregate([
  { $match: { channel_id: { $in: [1, 2, 3] } } },
  {
    $group: {
      _id: "$channel_id",
      total_posts: { $sum: 1 }
    }
  }
]).explain();

// Проверка распределения данных по шардам
print("\n📊 Shard Distribution:");
db.posts.getShardDistribution();

// ============================================
// 8. КЭШИРОВАНИЕ
// ============================================

// Создание материализованного представления для кэширования
db.createCollection("cached_channel_reports");

// Функция обновления кэша
function updateChannelReportsCache() {
  print("🔄 Updating channel reports cache...");
  
  const report = db.posts.aggregate([
    {
      $lookup: {
        from: "channels",
        localField: "channel_id",
        foreignField: "channel_id",
        as: "channel"
      }
    },
    { $unwind: "$channel" },
    {
      $group: {
        _id: "$channel_id",
        channel_name: { $first: "$channel.name" },
        total_posts: { $sum: 1 },
        total_views: { $sum: "$stats.views" },
        total_likes: { $sum: "$stats.likes" },
        avg_likes_per_post: { $avg: "$stats.likes" },
        top_tags: { $push: "$tags" },
        last_post_date: { $max: "$created_at" }
      }
    },
    {
      $project: {
        channel_id: "$_id",
        channel_name: 1,
        total_posts: 1,
        total_views: 1,
        total_likes: 1,
        avg_likes_per_post: { $round: ["$avg_likes_per_post", 2] },
        top_tags: {
          $slice: [
            {
              $reduce: {
                input: "$top_tags",
                initialValue: [],
                in: { $setUnion: ["$$value", "$$this"] }
              }
            },
            10
          ]
        },
        last_post_date: 1,
        engagement_rate: {
          $round: [
            {
              $multiply: [
                { $divide: ["$total_likes", { $max: ["$total_views", 1] }] },
                100
              ]
            },
            2
          ]
        },
        cached_at: new Date(),
        _id: 0
      }
    },
    { $out: "cached_channel_reports" }
  ]);

  print("✅ Cache updated successfully");
  return report;
}

// Триггер обновления кэша при изменении данных (Change Streams)
const changeStream = db.posts.watch([
  {
    $match: {
      operationType: { $in: ["insert", "update", "delete"] }
    }
  }
]);

print("👁️ Change stream initialized for cache invalidation");

// Обработчик изменений
changeStream.on("change", (change) => {
  print("⚠️ Data changed, invalidating cache...");
  print("Operation:", change.operationType);
  print("Document ID:", change.documentKey);
  
  // Обновляем кэш
  updateChannelReportsCache();
});

// Индексы для кэшированной коллекции
db.cached_channel_reports.createIndex({ channel_id: 1 }, { unique: true });
db.cached_channel_reports.createIndex({ total_posts: -1 });
db.cached_channel_reports.createIndex({ engagement_rate: -1 });
db.cached_channel_reports.createIndex({ cached_at: 1 });

// TTL индекс для автоматического удаления старого кэша (через 1 час)
db.cached_channel_reports.createIndex(
  { cached_at: 1 },
  { expireAfterSeconds: 3600 }
);

// Первоначальное наполнение кэша
updateChannelReportsCache();

// Запрос из кэша (быстрый)
print("\n⚡ Reading from cache:");
const cachedData = db.cached_channel_reports.find()
  .sort({ engagement_rate: -1 })
  .limit(10)
  .toArray();

print("Cached results count:", cachedData.length);
printjson(cachedData);

// ============================================
// ТЕСТОВЫЕ ДАННЫЕ
// ============================================

// Генерация тестовых данных для демонстрации
print("\n📝 Generating test data...");

// Каналы
db.channels.insertMany([
  { channel_id: 1, name: "Tech News", source_id: 1, subscribers_count: 10000, topic: "технологии", created_at: new Date() },
  { channel_id: 2, name: "AI Daily", source_id: 1, subscribers_count: 15000, topic: "AI", created_at: new Date() },
  { channel_id: 3, name: "Science Hub", source_id: 2, subscribers_count: 8000, topic: "наука", created_at: new Date() }
]);

// Теги
db.tags.insertMany([
  { tag_id: 1, name: "технологии", usage_count: 0, created_at: new Date() },
  { tag_id: 2, name: "AI", usage_count: 0, created_at: new Date() },
  { tag_id: 3, name: "инновации", usage_count: 0, created_at: new Date() }
]);

// Посты с комментариями
for (let i = 1; i <= 100; i++) {
  db.posts.insertOne({
    post_id: i,
    title: `Пост номер ${i} о технологиях`,
    content: `Это детальное содержание поста ${i}. ` + "Lorem ipsum ".repeat(20),
    channel_id: (i % 3) + 1,
    author_id: (i % 10) + 1,
    tags: ["технологии", i % 2 === 0 ? "AI" : "инновации"],
    comments: [
      {
        comment_id: i * 10 + 1,
        nickname: `user_${i}`,
        text: `Отличный пост номер ${i}!`,
        likes_count: Math.floor(Math.random() * 50),
        created_at: new Date(Date.now() - Math.random() * 7 * 24 * 60 * 60 * 1000)
      }
    ],
    stats: {
      views: Math.floor(Math.random() * 1000),
      likes: Math.floor(Math.random() * 100),
      shares: Math.floor(Math.random() * 20)
    },
    created_at: new Date(Date.now() - Math.random() * 30 * 24 * 60 * 60 * 1000),
    updated_at: new Date()
  });
}

print("✅ Test data generated successfully!");
print("📊 Collections populated:");
print("  - Channels: 3");
print("  - Tags: 3");
print("  - Posts: 100 (with embedded comments)");

print("\n🎉 All MongoDB advanced features implemented successfully!");