#include "httplib.h"
// #include "/media/vitalii/medio/study/News-Aggregator/cpp-httplib/httplib.h"
#include <nlohmann/json.hpp>
#include <pqxx/pqxx>
#include <queue>
#include <string>
#include <sw/redis++/redis++.h>

using json = nlohmann::json;
using namespace sw::redis;

class PgPool {
public:
  PgPool(const std::string &conninfo, size_t pool_size = 4)
      : conninfo_(conninfo) {
    for (size_t i = 0; i < pool_size; ++i) {
      auto conn = std::make_shared<pqxx::connection>(conninfo_);
      if (!conn->is_open()) {
        throw std::runtime_error("Failed to open DB connection in pool");
      }
      pool_.push(conn);
    }
  }

  struct PConn {
    std::shared_ptr<pqxx::connection> conn;
    PgPool *pool;
    ~PConn() {
      if (conn && pool)
        pool->release(conn);
    }
  };

  PConn acquire() {
    std::unique_lock<std::mutex> lk(mutex_);
    cv_.wait(lk, [&] { return !pool_.empty(); });
    auto conn = pool_.front();
    pool_.pop();
    return PConn{conn, this};
  }

private:
  void release(std::shared_ptr<pqxx::connection> conn) {
    std::unique_lock<std::mutex> lk(mutex_);
    pool_.push(conn);
    lk.unlock();
    cv_.notify_one();
  }

  std::string conninfo_;
  std::queue<std::shared_ptr<pqxx::connection>> pool_;
  std::mutex mutex_;
  std::condition_variable cv_;
};

const std::string CONN_STR = "host=db "
                             "port=5432 "
                             "dbname=news_db "
                             "user=news_user "
                             "password=news_pass";

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
  PgPool pool(CONN_STR);
  Redis redis("tcp://redis:6379");

  // Добавление записи
  svr.Post(R"(/api/([A-Za-z_]+))",
           [&](const httplib::Request &req, httplib::Response &res) {
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

               auto pconn = pool.acquire();
               pqxx::work txn(*pconn.conn);

               std::string values;
               for (size_t i = 0; i < params.size(); ++i) {
                 if (i)
                   values += ", ";
                 if (params[i] == "__NULL__")
                   values += "NULL";
                 else
                   values += txn.quote(params[i]);
               }

               std::string sql_query = "INSERT INTO " + table + " (" + collist +
                                       ") VALUES (" + values + ")";
               txn.exec(sql_query);
               txn.commit();

               redis.del("cache:" + table);

               res.set_content("Item added\n", "text/plain");
             } catch (const std::exception &e) {
               res.status = 500;
               res.set_content(std::string("Error: ") + e.what(), "text/plain");
             }
           });

  // Получение всех записей
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

              auto pconn = pool.acquire();
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

              // 🔹 Сохранить результат в кэш Redis
              redis.set("cache:" + table, arr.dump(2));
            } catch (const std::exception &e) {
              res.status = 500;
              res.set_content(std::string("Error: ") + e.what(), "text/plain");
            }
          });

  // Получение одной записи
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

                  auto pconn = pool.acquire();
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

                  // 💾 Сохраняем в кэш
                  redis.set(cache_key, arr.dump(2));
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

              auto pconn = pool.acquire();
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

              // 🔹 Сохранить результат в кэш Redis
              redis.set("cache:" + table + ":" + id, arr.dump(2));
            } catch (const std::exception &e) {
              res.status = 500;
              res.set_content(std::string("Error: ") + e.what(), "text/plain");
            }
          });

  // Обновление записи
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

              auto pconn = pool.acquire();
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

  // Удаление записи
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

              auto pconn = pool.acquire();
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

          auto pconn = pool.acquire();
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
}