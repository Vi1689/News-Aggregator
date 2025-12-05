#include "MongoManager.h"
#include <algorithm>
#include <cstdlib>
#include <iostream>

using bsoncxx::builder::stream::close_array;
using bsoncxx::builder::stream::close_document;
using bsoncxx::builder::stream::document;
using bsoncxx::builder::stream::finalize;
using bsoncxx::builder::stream::open_array;
using bsoncxx::builder::stream::open_document;

mongocxx::instance MongoManager::instance{};

MongoManager::MongoManager(const std::string &uri) {
  const char *env_uri = std::getenv("MONGODB_URI");
  std::string connection_uri = env_uri ? env_uri : uri;

  std::cout << "Connecting to MongoDB: " << connection_uri << std::endl;

  try {
    client = mongocxx::client{mongocxx::uri{connection_uri}};
    db = client["news_aggregator"];
    createCollections();
    createIndexes();
    std::cout << "MongoDB connected successfully" << std::endl;
  } catch (const std::exception &e) {
    std::cerr << "MongoDB connection failed: " << e.what() << std::endl;
    throw;
  }
}

void MongoManager::createCollections() {
  try {
    // Создаем коллекции если их нет
    db.create_collection("posts");
    db.create_collection("user_interactions");
    db.create_collection("top_posts_view");
  } catch (...) {
    // Коллекции уже существуют
  }
}

void MongoManager::createIndexes() {
  auto posts = db["posts"];
  auto interactions = db["user_interactions"];
  auto top_posts = db["top_posts_view"];

  // ============ ИНДЕКСЫ ДЛЯ POSTS ============

  // 1. Текстовый индекс для полнотекстового поиска
  posts.create_index(document{} << "title" << "text" << "content" << "text"
                                << "tags" << "text" << finalize,
                     mongocxx::options::index{}.weights(
                         document{} << "title" << 10 << "content" << 5 << "tags"
                                    << 3 << finalize));

  // 2. Unique индекс для post_id
  posts.create_index(document{} << "post_id" << 1 << finalize,
                     mongocxx::options::index{}.unique(true));

  // 3. Unique индекс для дедубликации по content_hash
  posts.create_index(document{} << "content_hash" << 1 << finalize,
                     mongocxx::options::index{}.unique(true).sparse(true));

  // 4. Составной индекс для поиска по тегам и сортировке
  posts.create_index(document{} << "tags" << 1 << "stats.likes" << -1
                                << "created_at" << -1 << finalize);

  // 5. Индекс по массиву тегов (multikey index)
  posts.create_index(document{} << "tags" << 1 << finalize);

  // 6. Partial индекс для активных постов (с большим количеством лайков)
  posts.create_index(document{} << "stats.likes" << -1 << finalize,
                     mongocxx::options::index{}.partial_filter_expression(
                         document{} << "stats.likes" << open_document << "$gte"
                                    << 10 << close_document << finalize));

  // 7. TTL индекс для временных/старых постов (опционально)
  // Удаляет посты старше 1 года автоматически
  posts.create_index(
      document{} << "created_at" << 1 << finalize,
      mongocxx::options::index{}.expire_after(std::chrono::seconds(31536000)));

  // 8. Составной индекс для аналитики
  posts.create_index(document{} << "author_id" << 1 << "created_at" << -1
                                << finalize);

  // ============ ИНДЕКСЫ ДЛЯ USER_INTERACTIONS ============

  // 1. Compound index для поиска взаимодействий пользователя
  interactions.create_index(document{} << "user_id" << 1 << "timestamp" << -1
                                       << finalize);

  // 2. Index для поиска по посту
  interactions.create_index(document{} << "post_id" << 1 << finalize);

  // 3. TTL index - автоматически удаляет старые взаимодействия (90 дней)
  interactions.create_index(
      document{} << "timestamp" << 1 << finalize,
      mongocxx::options::index{}.expire_after(std::chrono::seconds(7776000)));

  // ============ ИНДЕКСЫ ДЛЯ TOP_POSTS_VIEW ============
  top_posts.create_index(document{} << "total_score" << -1 << finalize);

  std::cout << "MongoDB indexes created" << std::endl;
}

// ============ CRUD ОПЕРАЦИИ ============

void MongoManager::indexPost(int post_id, const std::string &title,
                             const std::string &content,
                             const std::vector<std::string> &tags) {
  auto posts = db["posts"];

  std::string content_hash =
      std::to_string(std::hash<std::string>{}(title + content));

  auto doc =
      document{} << "post_id" << post_id << "title" << title << "content"
                 << content << "content_hash" << content_hash << "tags"
                 << open_array <<
      [&tags](bsoncxx::builder::stream::array_context<> arr) {
        for (const auto &tag : tags)
          arr << tag;
      } << close_array
                 << "stats" << open_document << "views" << 0 << "likes" << 0
                 << "comments" << 0 << close_document << "created_at"
                 << bsoncxx::types::b_date{std::chrono::system_clock::now()}
                 << "updated_at"
                 << bsoncxx::types::b_date{std::chrono::system_clock::now()}
                 << finalize;

  posts.insert_one(doc.view());
}

void MongoManager::insertMany(const std::vector<json> &posts_data) {
  auto posts = db["posts"];
  std::vector<bsoncxx::document::value> docs;

  for (const auto &post : posts_data) {
    auto doc =
        document{} << "post_id" << post["post_id"].get<int>() << "title"
                   << post["title"].get<std::string>() << "content"
                   << post["content"].get<std::string>() << "tags" << open_array
                   <<
        [&post](bsoncxx::builder::stream::array_context<> arr) {
          if (post.contains("tags")) {
            for (const auto &tag : post["tags"]) {
              arr << tag.get<std::string>();
            }
          }
        } << close_array
                   << "stats" << open_document << "views" << 0 << "likes" << 0
                   << "comments" << 0 << close_document << "created_at"
                   << bsoncxx::types::b_date{std::chrono::system_clock::now()}
                   << finalize;

    docs.push_back(doc.extract());
  }

  if (!docs.empty()) {
    posts.insert_many(docs);
  }
}

void MongoManager::updatePostIndex(int post_id, const std::string &title,
                                   const std::string &content,
                                   const std::vector<std::string> &tags) {
  auto posts = db["posts"];

  std::string content_hash =
      std::to_string(std::hash<std::string>{}(title + content));

  // $set - обновление полей
  auto update_doc = document{}
                    << "$set" << open_document << "title" << title << "content"
                    << content << "content_hash" << content_hash << "tags"
                    << open_array <<
                    [&tags](bsoncxx::builder::stream::array_context<> arr) {
                      for (const auto &tag : tags)
                        arr << tag;
                    }
                    << close_array << "updated_at"
                    << bsoncxx::types::b_date{std::chrono::system_clock::now()}
                    << close_document << finalize;

  posts.update_one(document{} << "post_id" << post_id << finalize,
                   update_doc.view());
}

void MongoManager::incrementViewCount(int post_id) {
  auto posts = db["posts"];

  // $inc - инкремент счетчика
  posts.update_one(document{} << "post_id" << post_id << finalize,
                   document{} << "$inc" << open_document << "stats.views" << 1
                              << close_document << finalize);
}

void MongoManager::addTagToPost(int post_id, const std::string &tag) {
  auto posts = db["posts"];

  // $addToSet - добавление в массив без дубликатов
  posts.update_one(document{} << "post_id" << post_id << finalize,
                   document{} << "$addToSet" << open_document << "tags" << tag
                              << close_document << finalize);
}

void MongoManager::removeTagFromPost(int post_id, const std::string &tag) {
  auto posts = db["posts"];

  // $pull - удаление из массива
  posts.update_one(document{} << "post_id" << post_id << finalize,
                   document{} << "$pull" << open_document << "tags" << tag
                              << close_document << finalize);
}

void MongoManager::updatePostStats(int post_id, int likes_delta,
                                   int comments_delta) {
  auto posts = db["posts"];

  // $inc для нескольких полей сразу
  posts.update_one(document{} << "post_id" << post_id << finalize,
                   document{} << "$inc" << open_document << "stats.likes"
                              << likes_delta << "stats.comments"
                              << comments_delta << close_document << finalize);
}

bool MongoManager::upsert(int post_id, const json &post_data) {
  auto posts = db["posts"];

  auto doc =
      document{} << "$set" << open_document << "post_id" << post_id << "title"
                 << post_data["title"].get<std::string>() << "content"
                 << post_data["content"].get<std::string>() << "updated_at"
                 << bsoncxx::types::b_date{std::chrono::system_clock::now()}
                 << close_document << "$setOnInsert" << open_document
                 << "created_at"
                 << bsoncxx::types::b_date{std::chrono::system_clock::now()}
                 << "stats" << open_document << "views" << 0 << "likes" << 0
                 << "comments" << 0 << close_document << close_document
                 << finalize;

  mongocxx::options::update opts;
  opts.upsert(true);

  auto result = posts.update_one(document{} << "post_id" << post_id << finalize,
                                 doc.view(), opts);

  return result && result->upserted_id();
}

void MongoManager::replacePost(int post_id, const json &post_data) {
  auto posts = db["posts"];

  auto doc =
      document{} << "post_id" << post_id << "title"
                 << post_data["title"].get<std::string>() << "content"
                 << post_data["content"].get<std::string>() << "tags"
                 << open_array <<
      [&post_data](bsoncxx::builder::stream::array_context<> arr) {
        if (post_data.contains("tags")) {
          for (const auto &tag : post_data["tags"]) {
            arr << tag.get<std::string>();
          }
        }
      } << close_array
                 << "stats" << open_document << "views" << 0 << "likes" << 0
                 << "comments" << 0 << close_document << "created_at"
                 << bsoncxx::types::b_date{std::chrono::system_clock::now()}
                 << finalize;

  posts.replace_one(document{} << "post_id" << post_id << finalize, doc.view());
}

void MongoManager::removePostIndex(int post_id) {
  auto posts = db["posts"];
  posts.delete_one(document{} << "post_id" << post_id << finalize);
}

// ============ ПОИСК С ФИЛЬТРАМИ ============

json MongoManager::advancedSearch(const json &filters, int limit) {
  auto posts = db["posts"];
  json results = json::array();

  // Строим фильтр для MongoDB
  auto filter_doc = document{};

  // $and/$or/$in/$nin операторы
  if (filters.contains("tags") && filters["tags"].is_array()) {
    filter_doc << "tags" << open_document << "$in" << open_array <<
        [&filters](bsoncxx::builder::stream::array_context<> arr) {
          for (const auto &tag : filters["tags"]) {
            arr << tag.get<std::string>();
          }
        } << close_array
               << close_document;
  }

  // $gte/$lte для диапазонов
  if (filters.contains("min_likes")) {
    filter_doc << "stats.likes" << open_document << "$gte"
               << filters["min_likes"].get<int>() << close_document;
  }

  if (filters.contains("exclude_tags") && filters["exclude_tags"].is_array()) {
    filter_doc << "tags" << open_document << "$nin" << open_array <<
        [&filters](bsoncxx::builder::stream::array_context<> arr) {
          for (const auto &tag : filters["exclude_tags"]) {
            arr << tag.get<std::string>();
          }
        } << close_array
               << close_document;
  }

  auto filter = filter_doc << finalize;

  // Проекция - выбор только нужных полей
  auto projection = document{} << "post_id" << 1 << "title" << 1 << "tags" << 1
                               << "stats" << 1 << "_id" << 0 << finalize;

  mongocxx::options::find opts;
  opts.projection(projection.view());
  opts.limit(limit);
  opts.sort(document{} << "stats.likes" << -1 << finalize);

  auto cursor = posts.find(filter.view(), opts);

  for (auto &&doc : cursor) {
    json item;
    item["id"] = doc["post_id"].get_int32();
    item["title"] = doc["title"].get_string().value.to_string();

    if (doc["stats"]) {
      auto stats = doc["stats"].get_document();
      item["likes"] = stats.view()["likes"].get_int32();
      item["views"] = stats.view()["views"].get_int32();
    }

    results.push_back(item);
  }

  return results;
}

// ============ AGGREGATION PIPELINES ============

json MongoManager::getDashboardStats() {
  auto posts = db["posts"];

  // Pipeline 1: Общая статистика с группировкой
  mongocxx::pipeline pipeline;

  // $match - фильтрация
  pipeline.match(document{}
                 << "created_at" << open_document << "$gte"
                 << bsoncxx::types::b_date{std::chrono::system_clock::now() -
                                           std::chrono::hours(24 * 30)}
                 << close_document << finalize);

  // $project - выбор полей
  pipeline.project(document{} << "post_id" << 1 << "stats" << 1 << "tags" << 1
                              << "created_at" << 1 << finalize);

  // $group - агрегация
  pipeline.group(document{}
                 << "_id" << bsoncxx::types::b_null{} << "total_posts"
                 << open_document << "$sum" << 1 << close_document
                 << "total_likes" << open_document << "$sum" << "$stats.likes"
                 << close_document << "total_views" << open_document << "$sum"
                 << "$stats.views" << close_document << "avg_likes"
                 << open_document << "$avg" << "$stats.likes" << close_document
                 << finalize);

  auto cursor = posts.aggregate(pipeline);

  json stats;
  for (auto &&doc : cursor) {
    stats["total_posts"] = doc["total_posts"].get_int32();
    stats["total_likes"] = doc["total_likes"].get_int32();
    stats["total_views"] = doc["total_views"].get_int32();
    stats["avg_likes"] = doc["avg_likes"].get_double();
  }

  return stats;
}

json MongoManager::getTopTags(int limit) {
  auto posts = db["posts"];

  // Pipeline 2: Топ тегов с $unwind
  mongocxx::pipeline pipeline;

  // $unwind - разворачиваем массив tags
  pipeline.unwind("$tags");

  // $group - группируем по тегу
  pipeline.group(document{} << "_id" << "$tags" << "count" << open_document
                            << "$sum" << 1 << close_document << "total_likes"
                            << open_document << "$sum" << "$stats.likes"
                            << close_document << finalize);

  // $sort - сортировка
  pipeline.sort(document{} << "count" << -1 << finalize);

  // $limit - ограничение
  pipeline.limit(limit);

  // $project - финальная проекция
  pipeline.project(document{} << "tag" << "$_id" << "count" << 1
                              << "total_likes" << 1 << "_id" << 0 << finalize);

  auto cursor = posts.aggregate(pipeline);

  json tags = json::array();
  for (auto &&doc : cursor) {
    json tag;
    tag["name"] = doc["tag"].get_string().value.to_string();
    tag["count"] = doc["count"].get_int32();
    tag["total_likes"] = doc["total_likes"].get_int32();
    tags.push_back(tag);
  }

  return tags;
}

json MongoManager::getPostEngagementAnalysis(int days) {
  auto posts = db["posts"];

  // Pipeline 3: Анализ вовлеченности с временными интервалами
  mongocxx::pipeline pipeline;

  pipeline.match(document{}
                 << "created_at" << open_document << "$gte"
                 << bsoncxx::types::b_date{std::chrono::system_clock::now() -
                                           std::chrono::hours(24 * days)}
                 << close_document << finalize);

  // Добавляем вычисляемое поле - engagement rate
  pipeline.add_fields(document{}
                      << "engagement_rate" << open_document << "$divide"
                      << open_array << open_document << "$add" << open_array
                      << "$stats.likes" << "$stats.comments" << close_array
                      << close_document << open_document << "$max" << open_array
                      << "$stats.views" << 1 << close_array << close_document
                      << close_array << close_document << finalize);

  pipeline.group(document{} << "_id" << bsoncxx::types::b_null{}
                            << "avg_engagement" << open_document << "$avg"
                            << "$engagement_rate" << close_document
                            << "max_engagement" << open_document << "$max"
                            << "$engagement_rate" << close_document
                            << "posts_analyzed" << open_document << "$sum" << 1
                            << close_document << finalize);

  auto cursor = posts.aggregate(pipeline);

  json analysis;
  for (auto &&doc : cursor) {
    analysis["avg_engagement"] = doc["avg_engagement"].get_double();
    analysis["max_engagement"] = doc["max_engagement"].get_double();
    analysis["posts_analyzed"] = doc["posts_analyzed"].get_int32();
  }

  return analysis;
}

json MongoManager::getChannelPerformance() {
  auto posts = db["posts"];

  // Pipeline 4: Производительность каналов с $lookup (1 -> N связь)
  mongocxx::pipeline pipeline;

  // Группируем по channel_id (если он есть в документах)
  pipeline.group(document{}
                 << "_id" << "$channel_id" << "post_count" << open_document
                 << "$sum" << 1 << close_document << "total_likes"
                 << open_document << "$sum" << "$stats.likes" << close_document
                 << "total_views" << open_document << "$sum" << "$stats.views"
                 << close_document << "avg_likes_per_post" << open_document
                 << "$avg" << "$stats.likes" << close_document << finalize);

  pipeline.sort(document{} << "total_likes" << -1 << finalize);
  pipeline.limit(10);

  auto cursor = posts.aggregate(pipeline);

  json channels = json::array();
  for (auto &&doc : cursor) {
    json channel;
    if (doc["_id"].type() != bsoncxx::type::k_null) {
      channel["channel_id"] = doc["_id"].get_int32();
    } else {
      channel["channel_id"] = nullptr;
    }
    channel["post_count"] = doc["post_count"].get_int32();
    channel["total_likes"] = doc["total_likes"].get_int32();
    channel["total_views"] = doc["total_views"].get_int32();
    channel["avg_likes_per_post"] = doc["avg_likes_per_post"].get_double();
    channels.push_back(channel);
  }

  return channels;
}

// ============ ПОЛЬЗОВАТЕЛЬСКИЕ ВЗАИМОДЕЙСТВИЯ ============

void MongoManager::recordUserInteraction(const std::string &user_id,
                                         int post_id,
                                         const std::string &action) {
  auto interactions = db["user_interactions"];

  auto doc =
      document{} << "user_id" << user_id << "post_id" << post_id << "action"
                 << action << "timestamp"
                 << bsoncxx::types::b_date{std::chrono::system_clock::now()}
                 << finalize;

  interactions.insert_one(doc.view());
}

json MongoManager::getUserHistory(const std::string &user_id, int limit) {
  auto interactions = db["user_interactions"];

  mongocxx::pipeline pipeline;

  pipeline.match(document{} << "user_id" << user_id << finalize);

  // $lookup - соединяем с коллекцией posts (N -> N связь через post_id)
  pipeline.lookup(document{} << "from" << "posts" << "localField" << "post_id"
                             << "foreignField" << "post_id" << "as"
                             << "post_details" << finalize);

  // $unwind для post_details
  pipeline.unwind("$post_details");

  pipeline.sort(document{} << "timestamp" << -1 << finalize);
  pipeline.limit(limit);

  pipeline.project(document{} << "action" << 1 << "timestamp" << 1 << "post_id"
                              << 1 << "post_title" << "$post_details.title"
                              << "_id" << 0 << finalize);

  auto cursor = interactions.aggregate(pipeline);

  json history = json::array();
  for (auto &&doc : cursor) {
    json item;
    item["action"] = doc["action"].get_string().value.to_string();
    item["post_id"] = doc["post_id"].get_int32();
    item["post_title"] = doc["post_title"].get_string().value.to_string();
    history.push_back(item);
  }

  return history;
}

// ============ МАТЕРИАЛИЗОВАННОЕ ПРЕДСТАВЛЕНИЕ ============

void MongoManager::materializeTopPostsView() {
  auto posts = db["posts"];
  auto top_posts_view = db["top_posts_view"];

  // Очищаем старую витрину
  top_posts_view.delete_many({});

  // Pipeline 5: Создаем витрину топовых постов
  mongocxx::pipeline pipeline;

  pipeline.match(document{}
                 << "created_at" << open_document << "$gte"
                 << bsoncxx::types::b_date{std::chrono::system_clock::now() -
                                           std::chrono::hours(24 * 7)}
                 << close_document << finalize);

  // Вычисляем общий score
  pipeline.add_fields(
      document{} << "total_score" << open_document << "$add" << open_array
                 << open_document << "$multiply" << open_array << "$stats.likes"
                 << 3 << close_array << close_document << open_document
                 << "$multiply" << open_array << "$stats.comments" << 2
                 << close_array << close_document << "$stats.views"
                 << close_array << close_document << finalize);

  pipeline.sort(document{} << "total_score" << -1 << finalize);
  pipeline.limit(100);

  pipeline.out("top_posts_view");

  posts.aggregate(pipeline);

  std::cout << "Top posts view materialized" << std::endl;
}

json MongoManager::getTopPostsFromView(int limit) {
  auto top_posts_view = db["top_posts_view"];

  mongocxx::options::find opts;
  opts.sort(document{} << "total_score" << -1 << finalize);
  opts.limit(limit);

  auto cursor = top_posts_view.find({}, opts);

  json posts = json::array();
  for (auto &&doc : cursor) {
    json post;
    post["post_id"] = doc["post_id"].get_int32();
    post["title"] = doc["title"].get_string().value.to_string();
    post["total_score"] = doc["total_score"].get_double();

    if (doc["stats"]) {
      auto stats = doc["stats"].get_document();
      post["likes"] = stats.view()["likes"].get_int32();
      post["views"] = stats.view()["views"].get_int32();
      post["comments"] = stats.view()["comments"].get_int32();
    }

    posts.push_back(post);
  }

  return posts;
}

// ============ ОСТАЛЬНЫЕ МЕТОДЫ (из предыдущей версии) ============

bool MongoManager::isDuplicateContent(const std::string &content_hash) {
  auto posts = db["posts"];
  auto result =
      posts.find_one(document{} << "content_hash" << content_hash << finalize);
  return static_cast<bool>(result);
}

std::vector<SearchResult> MongoManager::searchPosts(const std::string &query,
                                                    int limit) {
  auto posts = db["posts"];
  std::vector<SearchResult> results;

  try {
    auto cursor = posts.find(
        document{} << "$text" << open_document << "$search" << query
                   << "$language" << "russian" << close_document << finalize,
        mongocxx::options::find{}
            .projection(document{} << "post_id" << 1 << "title" << 1
                                   << "content" << 1 << "tags" << 1 << "score"
                                   << open_document << "$meta" << "textScore"
                                   << close_document << finalize)
            .sort(document{} << "score" << open_document << "$meta"
                             << "textScore" << close_document << finalize)
            .limit(limit));

    for (auto &&doc : cursor) {
      SearchResult result;
      result.id = doc["post_id"].get_int32();
      result.title = doc["title"].get_string().value.to_string();

      std::string content = doc["content"].get_string().value.to_string();
      result.preview = content.substr(0, 200) + "...";

      if (doc["score"]) {
        result.relevance = doc["score"].get_double();
      }

      if (doc["tags"] && doc["tags"].type() == bsoncxx::type::k_array) {
        auto tags_array = doc["tags"].get_array().value;
        for (auto &&tag : tags_array) {
          result.matched_tags.push_back(tag.get_string().value.to_string());
        }
      }

      results.push_back(result);
    }
  } catch (const std::exception &e) {
    std::cerr << "MongoDB search error: " << e.what() << std::endl;
  }

  return results;
}

std::vector<int> MongoManager::getSimilarPosts(int post_id, int limit) {
  auto posts = db["posts"];
  std::vector<int> similar_ids;

  // Получаем теги текущего поста
  auto current_post =
      posts.find_one(document{} << "post_id" << post_id << finalize);

  if (!current_post)
    return similar_ids;

  auto tags_element = current_post->view()["tags"];
  if (!tags_element || tags_element.type() != bsoncxx::type::k_array) {
    return similar_ids;
  }

  auto tags_array = tags_element.get_array().value;
  std::vector<std::string> tags;
  for (auto &&tag : tags_array) {
    tags.push_back(tag.get_string().value.to_string());
  }

  // Ищем похожие посты по тегам
  auto cursor = posts.find(
      document{} << "post_id" << open_document << "$ne" << post_id
                 << close_document << "tags" << open_document << "$in"
                 << open_array <<
          [&tags](bsoncxx::builder::stream::array_context<> arr) {
            for (const auto &tag : tags)
              arr << tag;
          } << close_array
                 << close_document << finalize,
      mongocxx::options::find{}
          .sort(document{} << "stats.likes" << -1 << finalize)
          .limit(limit));

  for (auto &&doc : cursor) {
    similar_ids.push_back(doc["post_id"].get_int32());
  }

  return similar_ids;
}

json MongoManager::getAuthorStats(int author_id) {
  // Заглушка - можно расширить
  json stats;
  stats["author_id"] = author_id;
  return stats;
}

std::vector<SearchResult>
MongoManager::searchByTags(const std::vector<std::string> &tags) {
  // Заглушка - можно расширить
  return std::vector<SearchResult>();
}