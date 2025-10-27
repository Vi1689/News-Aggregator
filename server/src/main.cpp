// #include "/media/vitalii/medio/study/News-Aggregator/cpp-httplib/httplib.h"
#include "httplib.h"
#include <chrono>
#include <nlohmann/json.hpp>
#include <pqxx/pqxx>
#include <queue>
#include <string>
#include <sw/redis++/redis++.h>
#include <thread>

using json = nlohmann::json;
using namespace sw::redis;

// Модифицированный PgPool с поддержкой реплик
class PgPool {
public:
  PgPool(const std::vector<std::string> &conn_infos, size_t pool_size = 4) {
    for (const auto &conninfo : conn_infos) {
      for (size_t i = 0; i < pool_size; ++i) {
        try {
          auto conn = std::make_shared<pqxx::connection>(conninfo);
          if (!conn->is_open()) {
            std::cerr << "Failed to open DB connection: " << conninfo
                      << std::endl;
            continue;
          }
          // Проверяем, является ли соединение мастером
          try {
            pqxx::work txn(*conn);
            pqxx::result r = txn.exec("SELECT pg_is_in_recovery()");
            bool is_replica = r[0][0].as<bool>();
            if (is_replica) {
              std::cout << "Added REPLICA connection: " << conninfo
                        << std::endl;
              replica_pool_.push(conn);
            } else {
              std::cout << "Added MASTER connection: " << conninfo << std::endl;
              master_pool_.push(conn);
            }
          } catch (const std::exception &e) {
            std::cerr << "Error checking DB role: " << e.what() << std::endl;
            // Если не удалось определить роль, считаем репликой
            replica_pool_.push(conn);
          }
        } catch (const std::exception &e) {
          std::cerr << "Failed to create connection: " << e.what() << std::endl;
        }
      }
    }

    if (master_pool_.empty() && replica_pool_.empty()) {
      throw std::runtime_error("No valid database connections available");
    }
  }

  struct PConn {
    std::shared_ptr<pqxx::connection> conn;
    PgPool *pool;
    bool is_replica;
    ~PConn() {
      if (conn && pool)
        pool->release(conn, is_replica);
    }
  };

  PConn acquire(bool read_only = false) {
    std::unique_lock<std::mutex> lk(mutex_);

    // Для операций чтения сначала пытаемся использовать реплику
    if (read_only && !replica_pool_.empty()) {
      auto conn = replica_pool_.front();
      replica_pool_.pop();
      return PConn{conn, this, true};
    }

    // Для операций записи или если реплик нет - используем мастер
    if (!master_pool_.empty()) {
      auto conn = master_pool_.front();
      master_pool_.pop();
      return PConn{conn, this, false};
    }

    // Если мастера нет, но операция read-only - используем реплику
    if (read_only && !replica_pool_.empty()) {
      auto conn = replica_pool_.front();
      replica_pool_.pop();
      return PConn{conn, this, true};
    }

    // Ждем доступное соединение
    cv_.wait(lk, [&] {
      return (!master_pool_.empty()) || (read_only && !replica_pool_.empty());
    });

    if (!master_pool_.empty()) {
      auto conn = master_pool_.front();
      master_pool_.pop();
      return PConn{conn, this, false};
    } else {
      auto conn = replica_pool_.front();
      replica_pool_.pop();
      return PConn{conn, this, true};
    }
  }

  void health_check() {
    std::unique_lock<std::mutex> lk(mutex_);

    // Проверяем соединения мастера
    std::queue<std::shared_ptr<pqxx::connection>> new_master_pool;
    while (!master_pool_.empty()) {
      auto conn = master_pool_.front();
      master_pool_.pop();
      if (check_connection(conn)) {
        new_master_pool.push(conn);
      }
    }
    master_pool_ = std::move(new_master_pool);

    // Проверяем соединения реплик
    std::queue<std::shared_ptr<pqxx::connection>> new_replica_pool;
    while (!replica_pool_.empty()) {
      auto conn = replica_pool_.front();
      replica_pool_.pop();
      if (check_connection(conn)) {
        new_replica_pool.push(conn);
      }
    }
    replica_pool_ = std::move(new_replica_pool);

    cv_.notify_all();
  }

private:
  bool check_connection(std::shared_ptr<pqxx::connection> conn) {
    if (!conn->is_open()) {
      return false;
    }
    try {
      pqxx::work txn(*conn);
      txn.exec("SELECT 1");
      return true;
    } catch (...) {
      return false;
    }
  }

  void release(std::shared_ptr<pqxx::connection> conn, bool is_replica) {
    std::unique_lock<std::mutex> lk(mutex_);
    if (is_replica) {
      replica_pool_.push(conn);
    } else {
      master_pool_.push(conn);
    }
    lk.unlock();
    cv_.notify_one();
  }

  std::queue<std::shared_ptr<pqxx::connection>> master_pool_;
  std::queue<std::shared_ptr<pqxx::connection>> replica_pool_;
  std::mutex mutex_;
  std::condition_variable cv_;
};

// Строки подключения к мастеру и реплике
const std::vector<std::string> CONN_STRINGS = {
    "host=db-master port=5432 dbname=news_db user=news_user password=news_pass",
    "host=db-replica port=5432 dbname=news_db user=news_user "
    "password=news_pass"};

// Остальные константы остаются без изменений
const std::unordered_map<std::string, std::string> pk_map = {
    {"users", "user_id"},       {"authors", "author_id"},
    {"news_texts", "text_id"},  {"sources", "source_id"},
    {"channels", "channel_id"}, {"posts", "post_id"},
    {"media", "media_id"},      {"tags", "tag_id"},
    {"comments", "comment_id"}};

const std::vector<std::string> valid_tables = {"users",
                                               "authors",
                                               "news_texts",
                                               "sources",
                                               "channels",
                                               "posts",
                                               "media",
                                               "tags",
                                               "post_tags",
                                               "comments",
                                               "top_authors",
                                               "active_users",
                                               "popular_tags",
                                               "posts_by_channel",
                                               "avg_comments_per_post",
                                               "posts_ranked",
                                               "comments_moving_avg",
                                               "cumulative_posts",
                                               "tag_rank",
                                               "user_activity_rank",
                                               "posts_with_authors",
                                               "comments_with_users",
                                               "posts_with_tags",
                                               "posts_authors_channels",
                                               "comments_posts_users",
                                               "posts_authors_tags",
                                               "full_post_info",
                                               "full_post_media"};

bool is_valid_table(const std::string &t) {
  for (const auto &x : valid_tables)
    if (x == t)
      return true;
  return false;
}

int main() {
  httplib::Server svr;
  PgPool pool(CONN_STRINGS);
  Redis redis("tcp://redis:6379");

  // Запускаем health check в отдельном потоке
  std::thread health_checker([&pool]() {
    while (true) {
      std::this_thread::sleep_for(std::chrono::seconds(30));
      try {
        pool.health_check();
        std::cout << "Health check completed" << std::endl;
      } catch (const std::exception &e) {
        std::cerr << "Health check error: " << e.what() << std::endl;
      }
    }
  });
  health_checker.detach();

  // ДОБАВЛЕНИЕ ЗАПИСИ - используем только МАСТЕР
  svr.Post(R"(/api/([A-Za-z_]+))", [&](const httplib::Request &req,
                                       httplib::Response &res) {
    try {
      std::string table = req.matches[1];
      if (!is_valid_table(table)) {
        res.status = 404;
        res.set_content("Table not found", "text/plain");
        return;
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

      // ВАЖНО: для записи используем только МАСТЕР (read_only = false)
      auto pconn = pool.acquire(false);
      std::cout << "Using " << (pconn.is_replica ? "REPLICA" : "MASTER")
                << " for WRITE operation" << std::endl;

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

      redis.del("cache:" + table);

      res.set_content(obj.dump(2), "application/json");
    } catch (const std::exception &e) {
      res.status = 500;
      res.set_content(std::string("Error: ") + e.what(), "text/plain");
    }
  });

  // ПОЛУЧЕНИЕ ВСЕХ ЗАПИСЕЙ - используем РЕПЛИКУ если возможно
  svr.Get(R"(/api/([A-Za-z_]+))",
          [&](const httplib::Request &req, httplib::Response &res) {
            try {
              std::string table = req.matches[1];
              if (!is_valid_table(table)) {
                res.status = 404;
                res.set_content("Table not found", "text/plain");
                return;
              }

              auto cache_key = "cache:" + table;
              auto cached = redis.get(cache_key);
              if (cached) {
                res.set_content(*cached, "application/json");
                return;
              }

              // ВАЖНО: для чтения используем РЕПЛИКУ (read_only = true)
              auto pconn = pool.acquire(true);
              std::cout << "Using " << (pconn.is_replica ? "REPLICA" : "MASTER")
                        << " for READ operation" << std::endl;

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
              redis.setex("cache:" + table, 300, arr.dump(2)); // TTL 5 минут
            } catch (const std::exception &e) {
              res.status = 500;
              res.set_content(std::string("Error: ") + e.what(), "text/plain");
            }
          });

  // ПОЛУЧЕНИЕ ОДНОЙ ЗАПИСИ - используем РЕПЛИКУ если возможно
  svr.Get(R"(/api/([A-Za-z_]+)/([0-9]+)(?:/([0-9]+))?)",
          [&](const httplib::Request &req, httplib::Response &res) {
            try {
              std::string table = req.matches[1];
              if (!is_valid_table(table)) {
                res.status = 404;
                res.set_content("Table not found", "text/plain");
                return;
              }

              if (table == "post_tags") {
                if (!req.matches[2].matched || !req.matches[3].matched) {
                  res.status = 400;
                  res.set_content("Need post_id and tag_id in path",
                                  "text/plain");
                  return;
                }
                std::string post_id = req.matches[2].str();
                std::string tag_id = req.matches[3].str();
                try {
                  std::string cache_key =
                      "cache:post_tags:" + post_id + ":" + tag_id;
                  auto cached = redis.get(cache_key);
                  if (cached) {
                    res.set_content(*cached, "application/json");
                    return;
                  }

                  // Для чтения используем РЕПЛИКУ
                  auto pconn = pool.acquire(true);
                  pqxx::work txn(*pconn.conn);
                  std::string sql_query = "SELECT * FROM " + table +
                                          " WHERE post_id=$1 AND tag_id=$2";
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
                  redis.setex(cache_key, 600, arr.dump(2)); // TTL 10 минут
                } catch (const std::exception &e) {
                  res.status = 500;
                  res.set_content(e.what(), "text/plain");
                }
                return;
              }

              std::string id = req.matches[2].str();
              std::string cache_key = "cache:" + table + ":" + id;
              auto cached = redis.get(cache_key);
              if (cached) {
                res.set_content(*cached, "application/json");
                return;
              }

              auto it = pk_map.find(table);
              if (it == pk_map.end()) {
                res.status = 400;
                res.set_content("Table has no simple PK", "text/plain");
                return;
              }
              std::string pk = it->second;

              // Для чтения используем РЕПЛИКУ
              auto pconn = pool.acquire(true);
              pqxx::work txn(*pconn.conn);

              std::string sql_query =
                  "SELECT * FROM " + table + " WHERE " + pk + " = $1";
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
              redis.setex("cache:" + table + ":" + id, 600, arr.dump(2)); // TTL 10 минут
            } catch (const std::exception &e) {
              res.status = 500;
              res.set_content(std::string("Error: ") + e.what(), "text/plain");
            }
          });

  // ОБНОВЛЕНИЕ ЗАПИСИ - используем только МАСТЕР
  svr.Put(R"(/api/([A-Za-z_]+)/([0-9]+)(?:/([0-9]+))?)",
          [&](const httplib::Request &req, httplib::Response &res) {
            try {
              std::string table = req.matches[1];
              if (!is_valid_table(table)) {
                res.status = 404;
                res.set_content("Table not found", "text/plain");
                return;
              }
              std::string id = req.matches[2].str();

              auto it = pk_map.find(table);
              if (it == pk_map.end()) {
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
              auto pconn = pool.acquire(false);
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

              redis.del("cache:" + table);
              redis.del("cache:" + table + ":" + id);
              res.set_content("Item updated\n", "text/plain");
            } catch (const std::exception &e) {
              res.status = 500;
              res.set_content(std::string("Error: ") + e.what(), "text/plain");
            }
          });

  // УДАЛЕНИЕ ЗАПИСИ - используем только МАСТЕР
  svr.Delete(
      R"(/api/([A-Za-z_]+)/([0-9]+)(?:/([0-9]+))?)",
      [&](const httplib::Request &req, httplib::Response &res) {
        try {
          std::string table = req.matches[1];
          if (!is_valid_table(table)) {
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
              // Для записи используем только МАСТЕР
              auto pconn = pool.acquire(false);
              pqxx::work txn(*pconn.conn);
              std::string sql_query =
                  "DELETE FROM " + table + " WHERE post_id=$1 AND tag_id=$2";
              txn.exec_params(sql_query, post_id, tag_id);
              txn.commit();

              res.set_content("Item deleted\n", "text/plain");
              redis.del("cache:post_tags:" + post_id + ":" + tag_id);
              redis.del("cache:posts:" + post_id);
            } catch (const std::exception &e) {
              res.status = 500;
              res.set_content(e.what(), "text/plain");
            }
            return;
          }

          std::string id = req.matches[2].str();
          auto it = pk_map.find(table);
          if (it == pk_map.end()) {
            res.status = 400;
            res.set_content("Table has no simple PK", "text/plain");
            return;
          }
          std::string pk = it->second;

          // Для записи используем только МАСТЕР
          auto pconn = pool.acquire(false);
          pqxx::work txn(*pconn.conn);

          std::string sql_query =
              "DELETE FROM " + table + " WHERE " + pk + " = " + txn.quote(id);
          txn.exec(sql_query);
          txn.commit();

          redis.del("cache:" + table);
          redis.del("cache:" + table + ":" + id);
          res.set_content("Item deleted\n", "text/plain");
        } catch (const std::exception &e) {
          res.status = 500;
          res.set_content(std::string("Error: ") + e.what(), "text/plain");
        }
      });

  svr.listen("0.0.0.0", 8080);
  return 0;
}