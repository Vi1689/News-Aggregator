#include "handlers.h"
#include "../models/Constants.h"
#include <iostream>
#include <nlohmann/json.hpp>
#include <pqxx/pqxx>

using json = nlohmann::json;

Handlers::Handlers(PgPool &pool, CacheManager &cache, MongoManager &mongo)
    : pool_(pool), cache_(cache), mongo_(mongo) {}

void Handlers::setupRoutes(httplib::Server &svr) {
  // ДОБАВЛЕНИЕ ЗАПИСИ
  svr.Post(R"(/api/([A-Za-z_]+))",
           [this](const httplib::Request &req, httplib::Response &res) {
             this->createHandler(req, res);
           });

  // ПОЛУЧЕНИЕ ВСЕХ ЗАПИСЕЙ
  svr.Get(R"(/api/([A-Za-z_]+))",
          [this](const httplib::Request &req, httplib::Response &res) {
            this->readAllHandler(req, res);
          });

  // ПОЛУЧЕНИЕ ОДНОЙ ЗАПИСИ
  svr.Get(R"(/api/([A-Za-z_]+)/([0-9]+)(?:/([0-9]+))?)",
          [this](const httplib::Request &req, httplib::Response &res) {
            this->readOneHandler(req, res);
          });

  // ОБНОВЛЕНИЕ ЗАПИСИ
  svr.Put(R"(/api/([A-Za-z_]+)/([0-9]+)(?:/([0-9]+))?)",
          [this](const httplib::Request &req, httplib::Response &res) {
            this->updateHandler(req, res);
          });

  // УДАЛЕНИЕ ЗАПИСИ
  svr.Delete(R"(/api/([A-Za-z_]+)/([0-9]+)(?:/([0-9]+))?)",
             [this](const httplib::Request &req, httplib::Response &res) {
               this->deleteHandler(req, res);
             });

  // 1. Расширенный поиск с фильтрами ($and, $or, $in, $nin, $gte, $lte)
  svr.Post("/api/mongo/search/advanced",
           [this](const httplib::Request &req, httplib::Response &res) {
             this->advancedSearchHandler(req, res);
           });

  // 2. Получить топ тегов (aggregation с $unwind, $group, $sort, $limit)
  svr.Get("/api/mongo/analytics/top-tags",
          [this](const httplib::Request &req, httplib::Response &res) {
            this->topTagsHandler(req, res);
          });

  // 3. Анализ вовлеченности (aggregation с $match, $project, $group)
  svr.Get("/api/mongo/analytics/engagement",
          [this](const httplib::Request &req, httplib::Response &res) {
            this->engagementAnalysisHandler(req, res);
          });

  // 4. История пользователя (aggregation с $lookup)
  svr.Get("/api/mongo/user/:user_id/history",
          [this](const httplib::Request &req, httplib::Response &res) {
            this->userHistoryHandler(req, res);
          });

  // 5. Топ посты из материализованного представления
  svr.Get("/api/mongo/top-posts",
          [this](const httplib::Request &req, httplib::Response &res) {
            this->topPostsViewHandler(req, res);
          });

  // 6. Обновление поста с различными операторами ($set, $inc, $push, $addToSet)
  svr.Post("/api/mongo/posts/:post_id/operations",
           [this](const httplib::Request &req, httplib::Response &res) {
             this->postOperationsHandler(req, res);
           });

  // 7. Производительность каналов
  svr.Get("/api/mongo/analytics/channels",
          [this](const httplib::Request &req, httplib::Response &res) {
            this->channelPerformanceHandler(req, res);
          });

  // 8. Материализация витрины (триггер пересчета)
  svr.Post("/api/mongo/materialize",
           [this](const httplib::Request &req, httplib::Response &res) {
             this->materializeViewHandler(req, res);
           });
}

void Handlers::createHandler(const httplib::Request &req,
                             httplib::Response &res) {
  try {
    std::string table = req.matches[1];
    if (!constants::is_valid_table(table)) {
      res.status = 404;
      res.set_content("Table not found", "text/plain");
      return;
    }

    // ПРОВЕРКА ДУБЛИКАТОВ ТОЛЬКО ДЛЯ POSTS
    if (table == "posts") {
      auto data = json::parse(req.body);

      if (data.contains("title") && data.contains("content")) {
        std::string title = data["title"];
        std::string content = data["content"];

        // Генерируем хеш контента
        std::string content_hash =
            std::to_string(std::hash<std::string>{}(title + content));

        // Проверяем дубликат в MongoDB
        if (mongo_.isDuplicateContent(content_hash)) {
          res.status = 409;
          res.set_content("Duplicate post detected", "text/plain");
          return;
        }
      }
    }

    auto data = json::parse(req.body);

    std::vector<std::string> cols;
    std::vector<std::string> params;
    for (auto it = data.begin(); it != data.end(); ++it) {
      cols.push_back(it.key());
      if (it.value().is_null())
        params.push_back("__NULL__");
      else if (it.value().is_string())
        params.push_back(it.value().get<std::string>());
      else
        params.push_back(it.value().dump());
    }
    if (cols.empty()) {
      res.status = 400;
      res.set_content("No fields provided", "text/plain");
      return;
    }

    std::string placeholders;
    std::string collist;
    for (size_t i = 0; i < cols.size(); ++i) {
      if (i) {
        placeholders += ",";
        collist += ",";
      }
      placeholders += "$" + std::to_string(i + 1);
      collist += cols[i];
    }

    // Для записи используем только МАСТЕР (read_only = false)
    auto pconn = pool_.acquire(false);
    std::cout << "Using " << (pconn.is_replica ? "REPLICA" : "MASTER")
              << " for WRITE operation on table: " << table << std::endl;

    pqxx::work txn(*pconn.conn);

    std::string sql_query = "INSERT INTO " + table + " (" + collist +
                            ") VALUES (" + placeholders + ") RETURNING *";

    pqxx::params p;
    for (const auto &param : params) {
      if (param == "__NULL__") {
        p.append(std::monostate{});
      } else {
        p.append(param);
      }
    }

    pqxx::result r = txn.exec(sql_query, p);
    txn.commit();

    if (r.empty()) {
      res.status = 500;
      res.set_content("Failed to retrieve inserted item", "text/plain");
      return;
    }

    json obj;
    const auto &row = r[0];
    for (const auto &field : row) {
      if (field.is_null())
        obj[field.name()] = nullptr;
      else
        obj[field.name()] = field.c_str();
    }

    // ИНДЕКСИРУЕМ НОВЫЙ POST В MONGODB
    if (table == "posts") {
      int post_id = row["post_id"].as<int>();
      std::string title = data["title"];
      std::string content = data["content"];

      // Получаем теги если они есть
      std::vector<std::string> tags;
      if (data.contains("tags")) {
        tags = data["tags"].get<std::vector<std::string>>();
      }

      mongo_.indexPost(post_id, title, content, tags);
    }

    cache_.del("cache:" + table);

    res.set_content(obj.dump(2), "application/json");
  } catch (const std::runtime_error &e) {
    // Таймаут или проблемы с подключением к БД
    res.status = 503; // Service Unavailable
    res.set_content(std::string("Database temporarily unavailable: ") +
                        e.what(),
                    "text/plain");
  } catch (const std::exception &e) {
    res.status = 500;
    res.set_content(std::string("Error: ") + e.what(), "text/plain");
  }
}

void Handlers::readAllHandler(const httplib::Request &req,
                              httplib::Response &res) {
  try {
    std::string table = req.matches[1];
    if (!constants::is_valid_table(table)) {
      res.status = 404;
      res.set_content("Table not found", "text/plain");
      return;
    }

    auto cache_key = "cache:" + table;
    auto cached = cache_.get(cache_key);
    if (cached) {
      res.set_content(*cached, "application/json");
      return;
    }

    // Для чтения используем РЕПЛИКУ (read_only = true)
    auto pconn = pool_.acquire(true);
    std::cout << "Using " << (pconn.is_replica ? "REPLICA" : "MASTER")
              << " for READ operation on table: " << table << std::endl;

    pqxx::work txn(*pconn.conn);

    std::string sql_query = "SELECT * FROM " + table;
    pqxx::result r = txn.exec(sql_query);

    json arr = json::array();
    for (const auto &row : r) {
      json obj;
      for (const auto &field : row) {
        if (field.is_null())
          obj[field.name()] = nullptr;
        else
          obj[field.name()] = field.c_str();
      }
      arr.push_back(obj);
    }

    res.set_content(arr.dump(2), "application/json");
    cache_.setex("cache:" + table, 300, arr.dump(2)); // TTL 5 минут
  } catch (const std::runtime_error &e) {
    // Таймаут или проблемы с подключением к БД
    res.status = 503; // Service Unavailable
    res.set_content(std::string("Database temporarily unavailable: ") +
                        e.what(),
                    "text/plain");
  } catch (const std::exception &e) {
    res.status = 500;
    res.set_content(std::string("Error: ") + e.what(), "text/plain");
  }
}

void Handlers::readOneHandler(const httplib::Request &req,
                              httplib::Response &res) {
  try {
    std::string table = req.matches[1];
    if (!constants::is_valid_table(table)) {
      res.status = 404;
      res.set_content("Table not found", "text/plain");
      return;
    }

    if (table == "post_tags") {
      if (!req.matches[2].matched || !req.matches[3].matched) {
        res.status = 400;
        res.set_content("Need post_id and tag_id in path", "text/plain");
        return;
      }
      std::string post_id = req.matches[2].str();
      std::string tag_id = req.matches[3].str();
      try {
        std::string cache_key = "cache:post_tags:" + post_id + ":" + tag_id;
        auto cached = cache_.get(cache_key);
        if (cached) {
          res.set_content(*cached, "application/json");
          return;
        }

        // Для чтения используем РЕПЛИКУ
        auto pconn = pool_.acquire(true);
        pqxx::work txn(*pconn.conn);
        std::string sql_query =
            "SELECT * FROM " + table + " WHERE post_id=$1 AND tag_id=$2";
        pqxx::result r = txn.exec_params(sql_query, post_id, tag_id);

        json arr = json::array();
        for (const auto &row : r) {
          json obj;
          for (const auto &field : row) {
            if (field.is_null())
              obj[field.name()] = nullptr;
            else
              obj[field.name()] = field.c_str();
          }
          arr.push_back(obj);
        }

        res.set_content(arr.dump(2), "application/json");
        cache_.setex(cache_key, 600, arr.dump(2)); // TTL 10 минут
      } catch (const std::runtime_error &e) {
        res.status = 503;
        res.set_content(std::string("Database temporarily unavailable: ") +
                            e.what(),
                        "text/plain");
      } catch (const std::exception &e) {
        res.status = 500;
        res.set_content(e.what(), "text/plain");
      }
      return;
    }

    std::string id = req.matches[2].str();
    std::string cache_key = "cache:" + table + ":" + id;
    auto cached = cache_.get(cache_key);
    if (cached) {
      res.set_content(*cached, "application/json");
      return;
    }

    auto it = constants::pk_map.find(table);
    if (it == constants::pk_map.end()) {
      res.status = 400;
      res.set_content("Table has no simple PK", "text/plain");
      return;
    }
    std::string pk = it->second;

    // Для чтения используем РЕПЛИКУ
    auto pconn = pool_.acquire(true);
    pqxx::work txn(*pconn.conn);

    std::string sql_query = "SELECT * FROM " + table + " WHERE " + pk + " = $1";
    pqxx::result r = txn.exec_params(sql_query, id);

    json arr = json::array();
    for (const auto &row : r) {
      json obj;
      for (const auto &field : row) {
        if (field.is_null())
          obj[field.name()] = nullptr;
        else
          obj[field.name()] = field.c_str();
      }
      arr.push_back(obj);
    }

    res.set_content(arr.dump(2), "application/json");
    cache_.setex("cache:" + table + ":" + id, 600, arr.dump(2)); // TTL 10 минут
  } catch (const std::runtime_error &e) {
    // Таймаут или проблемы с подключением к БД
    res.status = 503; // Service Unavailable
    res.set_content(std::string("Database temporarily unavailable: ") +
                        e.what(),
                    "text/plain");
  } catch (const std::exception &e) {
    res.status = 500;
    res.set_content(std::string("Error: ") + e.what(), "text/plain");
  }
}

void Handlers::updateHandler(const httplib::Request &req,
                             httplib::Response &res) {
  try {
    std::string table = req.matches[1];
    if (!constants::is_valid_table(table)) {
      res.status = 404;
      res.set_content("Table not found", "text/plain");
      return;
    }
    std::string id = req.matches[2].str();

    auto it = constants::pk_map.find(table);
    if (it == constants::pk_map.end()) {
      res.status = 400;
      res.set_content("Table has no simple PK", "text/plain");
      return;
    }
    std::string pk = it->second;

    auto data = json::parse(req.body);

    std::vector<std::string> cols;
    std::vector<std::string> params;
    for (auto it = data.begin(); it != data.end(); ++it) {
      cols.push_back(it.key());
      if (it.value().is_null())
        params.push_back("__NULL__");
      else if (it.value().is_string())
        params.push_back(it.value().get<std::string>());
      else
        params.push_back(it.value().dump());
    }

    if (cols.empty()) {
      res.status = 400;
      res.set_content("No fields provided", "text/plain");
      return;
    }

    // Для записи используем только МАСТЕР
    auto pconn = pool_.acquire(false);
    pqxx::work txn(*pconn.conn);

    std::string set_clause;
    for (size_t i = 0; i < cols.size(); ++i) {
      if (i)
        set_clause += ", ";
      set_clause += cols[i] + " = ";
      if (params[i] == "__NULL__")
        set_clause += "NULL";
      else
        set_clause += txn.quote(params[i]);
    }

    std::string sql_query = "UPDATE " + table + " SET " + set_clause +
                            " WHERE " + pk + " = " + txn.quote(id);

    txn.exec(sql_query);
    txn.commit();

    // ОБНОВЛЯЕМ ИНДЕКС В MONGODB ДЛЯ POSTS
    if (table == "posts") {
      auto data = json::parse(req.body);

      std::string title, content;
      std::vector<std::string> tags;

      if (data.contains("title"))
        title = data["title"];
      if (data.contains("content"))
        content = data["content"];
      if (data.contains("tags"))
        tags = data["tags"].get<std::vector<std::string>>();

      int post_id = std::stoi(id);
      mongo_.updatePostIndex(post_id, title, content, tags);
    }

    cache_.del("cache:" + table);
    cache_.del("cache:" + table + ":" + id);
    res.set_content("Item updated\n", "text/plain");
  } catch (const std::runtime_error &e) {
    // Таймаут или проблемы с подключением к БД
    res.status = 503; // Service Unavailable
    res.set_content(std::string("Database temporarily unavailable: ") +
                        e.what(),
                    "text/plain");
  } catch (const std::exception &e) {
    res.status = 500;
    res.set_content(std::string("Error: ") + e.what(), "text/plain");
  }
}

void Handlers::deleteHandler(const httplib::Request &req,
                             httplib::Response &res) {
  try {
    std::string table = req.matches[1];
    if (!constants::is_valid_table(table)) {
      res.status = 404;
      res.set_content("Table not found", "text/plain");
      return;
    }

    std::string id = req.matches[2].str();

    // УДАЛЯЕМ ИЗ ИНДЕКСА MONGODB ДЛЯ POSTS
    if (table == "posts") {
      int post_id = std::stoi(id);
      mongo_.removePostIndex(post_id);
    }

    if (table == "post_tags") {
      handlePostTags(req, res, true);
      return;
    }

    auto it = constants::pk_map.find(table);
    if (it == constants::pk_map.end()) {
      res.status = 400;
      res.set_content("Table has no simple PK", "text/plain");
      return;
    }
    std::string pk = it->second;

    // Для записи используем только МАСТЕР
    auto pconn = pool_.acquire(false);
    pqxx::work txn(*pconn.conn);

    std::string sql_query =
        "DELETE FROM " + table + " WHERE " + pk + " = " + txn.quote(id);
    txn.exec(sql_query);
    txn.commit();

    cache_.del("cache:" + table);
    cache_.del("cache:" + table + ":" + id);
    res.set_content("Item deleted\n", "text/plain");
  } catch (const std::runtime_error &e) {
    // Таймаут или проблемы с подключением к БД
    res.status = 503; // Service Unavailable
    res.set_content(std::string("Database temporarily unavailable: ") +
                        e.what(),
                    "text/plain");
  } catch (const std::exception &e) {
    res.status = 500;
    res.set_content(std::string("Error: ") + e.what(), "text/plain");
  }
}

void Handlers::handlePostTags(const httplib::Request &req,
                              httplib::Response &res, bool isDelete) {
  if (!req.matches[2].matched || !req.matches[3].matched) {
    res.status = 400;
    res.set_content("Need post_id and tag_id in path", "text/plain");
    return;
  }

  std::string post_id = req.matches[2].str();
  std::string tag_id = req.matches[3].str();

  try {
    // Для записи используем только МАСТЕР
    auto pconn = pool_.acquire(false);
    pqxx::work txn(*pconn.conn);

    if (isDelete) {
      std::string sql_query =
          "DELETE FROM post_tags WHERE post_id=$1 AND tag_id=$2";
      txn.exec_params(sql_query, post_id, tag_id);
      txn.commit();

      res.set_content("Item deleted\n", "text/plain");
      cache_.del("cache:post_tags:" + post_id + ":" + tag_id);
      cache_.del("cache:posts:" + post_id);
    } else {
      // Для операций чтения post_tags
      std::string sql_query =
          "SELECT * FROM post_tags WHERE post_id=$1 AND tag_id=$2";
      pqxx::result r = txn.exec_params(sql_query, post_id, tag_id);

      json arr = json::array();
      for (const auto &row : r) {
        json obj;
        for (const auto &field : row) {
          if (field.is_null())
            obj[field.name()] = nullptr;
          else
            obj[field.name()] = field.c_str();
        }
        arr.push_back(obj);
      }

      res.set_content(arr.dump(2), "application/json");
    }
  } catch (const std::runtime_error &e) {
    // Таймаут или проблемы с подключением к БД
    res.status = 503; // Service Unavailable
    res.set_content(std::string("Database temporarily unavailable: ") +
                        e.what(),
                    "text/plain");
  } catch (const std::exception &e) {
    res.status = 500;
    res.set_content(e.what(), "text/plain");
  }
}

// 1. Расширенный поиск с фильтрами
void Handlers::advancedSearchHandler(const httplib::Request &req,
                                     httplib::Response &res) {
  try {
    auto filters = json::parse(req.body);

    std::string cache_key = "advanced_search:" + req.body;
    auto cached = cache_.get(cache_key);
    if (cached) {
      res.set_content(*cached, "application/json");
      return;
    }

    // Поиск с фильтрами: $in, $nin, $gte, $and, $or
    auto results = mongo_.advancedSearch(filters, 20);

    std::string response_str = results.dump(2);
    res.set_content(response_str, "application/json");
    cache_.setex(cache_key, 300, response_str);

  } catch (const std::exception &e) {
    res.status = 500;
    res.set_content(std::string("Advanced search error: ") + e.what(),
                    "text/plain");
  }
}

// 2. Топ тегов с aggregation
void Handlers::topTagsHandler(const httplib::Request &req,
                              httplib::Response &res) {
  try {
    int limit = 10;
    if (req.has_param("limit")) {
      limit = std::stoi(req.get_param_value("limit"));
    }

    auto cached = cache_.get("cache:top_tags:" + std::to_string(limit));
    if (cached) {
      res.set_content(*cached, "application/json");
      return;
    }

    // Aggregation pipeline: $unwind -> $group -> $sort -> $limit
    auto tags = mongo_.getTopTags(limit);
    std::string tags_str = tags.dump(2);

    res.set_content(tags_str, "application/json");
    cache_.setex("cache:top_tags:" + std::to_string(limit), 600, tags_str);

  } catch (const std::exception &e) {
    res.status = 500;
    res.set_content(std::string("Top tags error: ") + e.what(), "text/plain");
  }
}

// 3. Анализ вовлеченности
void Handlers::engagementAnalysisHandler(const httplib::Request &req,
                                         httplib::Response &res) {
  try {
    int days = 30;
    if (req.has_param("days")) {
      days = std::stoi(req.get_param_value("days"));
    }

    auto cached = cache_.get("cache:engagement:" + std::to_string(days));
    if (cached) {
      res.set_content(*cached, "application/json");
      return;
    }

    // Aggregation pipeline: $match -> $project -> $group
    auto analysis = mongo_.getPostEngagementAnalysis(days);
    std::string analysis_str = analysis.dump(2);

    res.set_content(analysis_str, "application/json");
    cache_.setex("cache:engagement:" + std::to_string(days), 300, analysis_str);

  } catch (const std::exception &e) {
    res.status = 500;
    res.set_content(std::string("Engagement analysis error: ") + e.what(),
                    "text/plain");
  }
}

// 4. История пользователя с $lookup
void Handlers::userHistoryHandler(const httplib::Request &req,
                                  httplib::Response &res) {
  try {
    std::string user_id = req.matches[1].str();

    int limit = 50;
    if (req.has_param("limit")) {
      limit = std::stoi(req.get_param_value("limit"));
    }

    std::string cache_key =
        "user_history:" + user_id + ":" + std::to_string(limit);
    auto cached = cache_.get(cache_key);
    if (cached) {
      res.set_content(*cached, "application/json");
      return;
    }

    // Aggregation with $lookup (N -> N join)
    auto history = mongo_.getUserHistory(user_id, limit);
    std::string history_str = history.dump(2);

    res.set_content(history_str, "application/json");
    cache_.setex(cache_key, 300, history_str);

  } catch (const std::exception &e) {
    res.status = 500;
    res.set_content(std::string("User history error: ") + e.what(),
                    "text/plain");
  }
}

// 5. Топ посты из материализованного представления
void Handlers::topPostsViewHandler(const httplib::Request &req,
                                   httplib::Response &res) {
  try {
    int limit = 10;
    if (req.has_param("limit")) {
      limit = std::stoi(req.get_param_value("limit"));
    }

    auto cached = cache_.get("cache:top_posts_view:" + std::to_string(limit));
    if (cached) {
      res.set_content(*cached, "application/json");
      return;
    }

    auto top_posts = mongo_.getTopPostsFromView(limit);
    std::string response_str = top_posts.dump(2);

    res.set_content(response_str, "application/json");
    cache_.setex("cache:top_posts_view:" + std::to_string(limit), 120,
                 response_str);

  } catch (const std::exception &e) {
    res.status = 500;
    res.set_content(std::string("Top posts view error: ") + e.what(),
                    "text/plain");
  }
}

// 6. Операции над постом ($set, $inc, $push, $addToSet, $pull)
void Handlers::postOperationsHandler(const httplib::Request &req,
                                     httplib::Response &res) {
  try {
    int post_id = std::stoi(req.matches[1].str());
    auto operations = json::parse(req.body);

    std::string operation_type = operations["operation"];

    if (operation_type == "increment_views") {
      // $inc оператор
      mongo_.incrementViewCount(post_id);
      res.set_content(json{{"message", "Views incremented"}}.dump(),
                      "application/json");

    } else if (operation_type == "add_tag") {
      // $addToSet оператор
      std::string tag = operations["tag"];
      mongo_.addTagToPost(post_id, tag);
      res.set_content(json{{"message", "Tag added"}}.dump(),
                      "application/json");

    } else if (operation_type == "remove_tag") {
      // $pull оператор
      std::string tag = operations["tag"];
      mongo_.removeTagFromPost(post_id, tag);
      res.set_content(json{{"message", "Tag removed"}}.dump(),
                      "application/json");

    } else if (operation_type == "update_stats") {
      // $inc для множественных полей
      int likes_delta = operations.value("likes_delta", 0);
      int comments_delta = operations.value("comments_delta", 0);
      mongo_.updatePostStats(post_id, likes_delta, comments_delta);
      res.set_content(json{{"message", "Stats updated"}}.dump(),
                      "application/json");

    } else if (operation_type == "upsert") {
      // upsert операция
      bool was_inserted = mongo_.upsertPost(post_id, operations["data"]);
      res.set_content(
          json{{"message", was_inserted ? "Post created" : "Post updated"},
               {"was_inserted", was_inserted}}
              .dump(),
          "application/json");

    } else {
      res.status = 400;
      res.set_content("Unknown operation type", "text/plain");
      return;
    }

    // Инвалидируем кеш
    cache_.del("cache:posts:" + std::to_string(post_id));

  } catch (const std::exception &e) {
    res.status = 500;
    res.set_content(std::string("Operation error: ") + e.what(), "text/plain");
  }
}

// 7. Производительность каналов
void Handlers::channelPerformanceHandler(const httplib::Request &req,
                                         httplib::Response &res) {
  try {
    auto cached = cache_.get("cache:channel_performance");
    if (cached) {
      res.set_content(*cached, "application/json");
      return;
    }

    auto performance = mongo_.getChannelPerformance();
    std::string response_str = performance.dump(2);

    res.set_content(response_str, "application/json");
    cache_.setex("cache:channel_performance", 600, response_str);

  } catch (const std::exception &e) {
    res.status = 500;
    res.set_content(std::string("Channel performance error: ") + e.what(),
                    "text/plain");
  }
}

// 8. Материализация витрины
void Handlers::materializeViewHandler(const httplib::Request &req,
                                      httplib::Response &res) {
  try {
    mongo_.materializeTopPostsView();

    // Инвалидируем кеш витрины
    cache_.del("cache:top_posts_view:*");

    res.set_content(
        json{{"message", "View materialized successfully"},
             {"timestamp",
              std::chrono::system_clock::now().time_since_epoch().count()}}
            .dump(),
        "application/json");

  } catch (const std::exception &e) {
    res.status = 500;
    res.set_content(std::string("Materialization error: ") + e.what(),
                    "text/plain");
  }
}