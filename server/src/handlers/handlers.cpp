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

// НОВЫЕ MONGO-РОУТЫ
    svr.Get("/api/search/posts", [this](const httplib::Request &req, httplib::Response &res) {
        this->searchPostsHandler(req, res);
    });
    
    svr.Get("/api/analytics/dashboard", [this](const httplib::Request &req, httplib::Response &res) {
        this->dashboardHandler(req, res);
    });
    
    svr.Get("/api/posts/(\\d+)/similar", [this](const httplib::Request &req, httplib::Response &res) {
        this->similarPostsHandler(req, res);
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
                std::string content_hash = std::to_string(
                    std::hash<std::string>{}(title + content)
                );
                
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
            
            if (data.contains("title")) title = data["title"];
            if (data.contains("content")) content = data["content"];
            if (data.contains("tags")) tags = data["tags"].get<std::vector<std::string>>();
            
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

   // УДАЛЯЕМ ИЗ ИНДЕКСА MONGODB ДЛЯ POSTS
        if (table == "posts") {
            int post_id = std::stoi(id);
            mongo_.removePostIndex(post_id);
        }

    if (table == "post_tags") {
      handlePostTags(req, res, true);
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

// ✅ НОВЫЙ HANDLER ДЛЯ ПОИСКА
void Handlers::searchPostsHandler(const httplib::Request &req, httplib::Response &res) {
    try {
        std::string query = req.get_param_value("q");
        if (query.empty()) {
            res.status = 400;
            res.set_content("Query parameter 'q' is required", "text/plain");
            return;
        }

        int limit = 20;
        if (req.has_param("limit")) {
            limit = std::stoi(req.get_param_value("limit"));
        }

        // Кэшируем результаты поиска
        std::string cache_key = "search:" + query + ":" + std::to_string(limit);
        auto cached = cache_.get(cache_key);
        if (cached) {
            res.set_content(*cached, "application/json");
            return;
        }

        // Ищем в MongoDB
        auto results = mongo_.searchPosts(query, limit);
        
        // Формируем ответ
        json response;
        response["query"] = query;
        response["results"] = json::array();
        
        for (const auto& result : results) {
            json item;
            item["id"] = result.id;
            item["title"] = result.title;
            item["preview"] = result.preview;
            item["relevance"] = result.relevance;
            item["matched_tags"] = result.matched_tags;
            response["results"].push_back(item);
        }

        std::string response_str = response.dump(2);
        res.set_content(response_str, "application/json");
        cache_.setex(cache_key, 300, response_str); // Кэш на 5 минут
        
    } catch (const std::exception &e) {
        res.status = 500;
        res.set_content(std::string("Search error: ") + e.what(), "text/plain");
    }
}

// ✅ НОВЫЙ HANDLER ДЛЯ ПОХОЖИХ ПОСТОВ
void Handlers::similarPostsHandler(const httplib::Request &req, httplib::Response &res) {
    try {
        int post_id = std::stoi(req.matches[1].str());
        
        std::string cache_key = "similar:" + std::to_string(post_id);
        auto cached = cache_.get(cache_key);
        if (cached) {
            res.set_content(*cached, "application/json");
            return;
        }

        auto similar_ids = mongo_.getSimilarPosts(post_id, 5);
        
        // Догружаем полные данные постов из PostgreSQL
        json response = json::array();
        for (int similar_id : similar_ids) {
            auto pconn = pool_.acquire(true);
            pqxx::work txn(*pconn.conn);
            
            std::string sql_query = "SELECT * FROM posts WHERE post_id = $1";
            pqxx::result r = txn.exec_params(sql_query, similar_id);
            
            for (const auto &row : r) {
                json obj;
                for (const auto &field : row) {
                    if (field.is_null())
                        obj[field.name()] = nullptr;
                    else
                        obj[field.name()] = field.c_str();
                }
                response.push_back(obj);
            }
        }

        std::string response_str = response.dump(2);
        res.set_content(response_str, "application/json");
        cache_.setex(cache_key, 600, response_str);
        
    } catch (const std::exception &e) {
        res.status = 500;
        res.set_content(std::string("Similar posts error: ") + e.what(), "text/plain");
    }
}

// ✅ НОВЫЙ HANDLER ДЛЯ ДАШБОРДА
void Handlers::dashboardHandler(const httplib::Request &req, httplib::Response &res) {
    try {
        auto cached = cache_.get("cache:dashboard");
        if (cached) {
            res.set_content(*cached, "application/json");
            return;
        }

        auto stats = mongo_.getDashboardStats();
        std::string stats_str = stats.dump(2);
        
        res.set_content(stats_str, "application/json");
        cache_.setex("cache:dashboard", 60, stats_str);
        
    } catch (const std::exception &e) {
        res.status = 500;
        res.set_content(std::string("Analytics error: ") + e.what(), "text/plain");
    }
}