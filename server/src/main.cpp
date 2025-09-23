// server/src/main.cpp
#include <iostream>
#include <string>
#include <unordered_map>
#include <vector>
#include <cstdlib>
#include <mutex>
#include <condition_variable>
#include <queue>
#include <memory>
#include <sstream>

#include "httplib.h"
#include "json.hpp"
#include <pqxx/pqxx>

using json = nlohmann::json;
using namespace httplib;

// ---------- Простая обёртка для пула соединений ----------
class PgPool {
public:
    PgPool(const std::string &conninfo, size_t pool_size = 4)
        : conninfo_(conninfo)
    {
        for (size_t i = 0; i < pool_size; ++i) {
            auto conn = std::make_shared<pqxx::connection>(conninfo_);
            if (!conn->is_open()) {
                throw std::runtime_error("Failed to open DB connection in pool");
            }
            pool_.push(conn);
        }
    }

    // RAII wrapper: возвращает connection and returns it to pool on destruction
    struct PConn {
        std::shared_ptr<pqxx::connection> conn;
        PgPool *pool;
        ~PConn() {
            if (conn && pool) pool->release(conn);
        }
    };

    PConn acquire() {
        std::unique_lock<std::mutex> lk(mutex_);
        cv_.wait(lk, [&]{ return !pool_.empty(); });
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

// ---------- Помощники ----------
std::string env_or(const char* name, const char* def) {
    const char* v = std::getenv(name);
    return v ? std::string(v) : std::string(def);
}

std::string make_conn_str() {
    std::string host = env_or("DB_HOST","localhost");
    std::string port = env_or("DB_PORT","5432");
    std::string db   = env_or("DB_NAME","news_db");
    std::string user = env_or("DB_USER","news_user");
    std::string pass = env_or("DB_PASS","news_pass");

    std::ostringstream ss;
    ss << "host=" << host << " port=" << port << " dbname=" << db << " user=" << user << " password=" << pass;
    return ss.str();
}

json row_to_json(const pqxx::row &row) {
    json obj = json::object();
    for (const auto &f : row) {
        std::string name = f.name();
        if (f.is_null()) obj[name] = nullptr;
        else obj[name] = f.c_str();
    }
    return obj;
}

// Заданные таблицы и PK
const std::unordered_map<std::string, std::string> pk_map = {
    {"users","user_id"},
    {"authors","author_id"},
    {"news_texts","text_id"},
    {"sources","source_id"},
    {"channels","channel_id"},
    {"posts","post_id"},
    {"media","media_id"},
    {"tags","tag_id"},
    {"comments","comment_id"}
};

const std::vector<std::string> valid_tables = {
    "users","authors","news_texts","sources","channels","posts","media","tags","post_tags","comments"
};

bool is_valid_table(const std::string &t) {
    for (auto &x : valid_tables) if (x==t) return true;
    return false;
}

// Подставляем параметры в SQL, экранируя через tx.quote(...)
std::string substitute_params(const std::string &sql_template, const std::vector<std::string> &params, pqxx::work &tx) {
    std::string res = sql_template;
    // заменяем $N на экранированное значение; делаем от большего к меньшему чтобы не повредить $1 при замене $10
    for (int i = (int)params.size(); i >= 1; --i) {
        std::string placeholder = "$" + std::to_string(i);
        std::string val = params[i-1];
        std::string quoted;
        if (val == "__NULL__") {
            quoted = "NULL";
        } else {
            quoted = tx.quote(val);
        }
        size_t pos = 0;
        while ((pos = res.find(placeholder, pos)) != std::string::npos) {
            res.replace(pos, placeholder.size(), quoted);
            pos += quoted.size();
        }
    }
    return res;
}

// Выполнить SQL с подстановкой параметров, возвращает результат
pqxx::result exec_with_params_pool(PgPool::PConn &pconn, const std::string &sql_template, const std::vector<std::string> &params) {
    pqxx::work tx(*pconn.conn);
    std::string sql = substitute_params(sql_template, params, tx);
    pqxx::result r = tx.exec(sql);
    tx.commit();
    return r;
}

// Отчеты (SQL)
std::unordered_map<std::string,std::string> reports_sql() {
    using namespace std;
    unordered_map<string,string> m;

    // агрегации
    m["agg_posts_per_channel"] =
        "SELECT c.channel_id, c.name AS channel_name, COUNT(p.post_id) AS posts_count "
        "FROM channels c LEFT JOIN posts p ON p.channel_id = c.channel_id "
        "GROUP BY c.channel_id, c.name ORDER BY posts_count DESC;";

    m["agg_comments_per_post"] =
        "SELECT p.post_id, p.title, COUNT(c.comment_id) AS comments_count "
        "FROM posts p LEFT JOIN comments c ON c.post_id = p.post_id "
        "GROUP BY p.post_id, p.title ORDER BY comments_count DESC;";

    m["agg_likes_per_author"] =
        "SELECT a.author_id, a.name AS author_name, COALESCE(SUM(p.likes_count),0) AS total_likes "
        "FROM authors a LEFT JOIN posts p ON p.author_id = a.author_id "
        "GROUP BY a.author_id, a.name ORDER BY total_likes DESC;";

    m["agg_avg_comments_per_channel"] =
        "SELECT c.channel_id, c.name AS channel_name, AVG(p.comments_count) AS avg_comments "
        "FROM channels c LEFT JOIN posts p ON p.channel_id = c.channel_id "
        "GROUP BY c.channel_id, c.name ORDER BY avg_comments DESC;";

    // оконные
    m["win_rank_posts_by_likes"] =
        "SELECT post_id, title, channel_id, likes_count, "
        "RANK() OVER (PARTITION BY channel_id ORDER BY likes_count DESC) AS like_rank "
        "FROM posts;";

    m["win_running_likes_per_author"] =
        "SELECT post_id, author_id, created_at, likes_count, "
        "SUM(likes_count) OVER (PARTITION BY author_id ORDER BY created_at "
        "ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) AS running_likes "
        "FROM posts;";

    m["win_comment_rownum_per_post"] =
        "SELECT comment_id, post_id, nickname, created_at, "
        "ROW_NUMBER() OVER (PARTITION BY post_id ORDER BY created_at DESC) AS rn "
        "FROM comments;";

    m["win_lag_likes_per_author"] =
        "SELECT post_id, author_id, created_at, likes_count, "
        "LAG(likes_count) OVER (PARTITION BY author_id ORDER BY created_at) AS prev_likes, "
        "likes_count - COALESCE(LAG(likes_count) OVER (PARTITION BY author_id ORDER BY created_at),0) AS diff_likes "
        "FROM posts;";

    // joins 2-table
    m["join_posts_authors"] =
        "SELECT p.post_id, p.title, a.author_id, a.name AS author_name "
        "FROM posts p JOIN authors a ON p.author_id = a.author_id;";

    m["join_posts_channels"] =
        "SELECT p.post_id, p.title, c.channel_id, c.name AS channel_name "
        "FROM posts p JOIN channels c ON p.channel_id = c.channel_id;";

    // joins 3-table (4 queries)
    m["join_posts_authors_channels"] =
        "SELECT p.post_id, p.title, a.name AS author_name, c.name AS channel_name "
        "FROM posts p JOIN authors a ON p.author_id = a.author_id "
        "JOIN channels c ON p.channel_id = c.channel_id;";

    m["join_posts_posttags_tags"] =
        "SELECT p.post_id, p.title, t.tag_id, t.name AS tag_name "
        "FROM posts p JOIN post_tags pt ON p.post_id = pt.post_id "
        "JOIN tags t ON pt.tag_id = t.tag_id;";

    m["join_comments_posts_authors"] =
        "SELECT c.comment_id, c.text AS comment_text, p.post_id, p.title, a.author_id, a.name AS author_name "
        "FROM comments c JOIN posts p ON c.post_id = p.post_id "
        "JOIN authors a ON p.author_id = a.author_id;";

    m["join_posts_media_channels"] =
        "SELECT p.post_id, p.title, m.media_id, m.media_content, c.channel_id, c.name AS channel_name "
        "FROM posts p JOIN media m ON m.post_id = p.post_id "
        "JOIN channels c ON p.channel_id = c.channel_id;";

    // join 4-table
    m["join_posts_authors_channels_sources"] =
        "SELECT p.post_id, p.title, a.author_id, a.name AS author_name, c.channel_id, c.name AS channel_name, s.source_id, s.name AS source_name "
        "FROM posts p JOIN authors a ON p.author_id = a.author_id "
        "JOIN channels c ON p.channel_id = c.channel_id "
        "JOIN sources s ON c.source_id = s.source_id;";

    // join 5-table
    m["join_posts_authors_channels_sources_media"] =
        "SELECT p.post_id, p.title, a.name AS author_name, c.name AS channel_name, s.name AS source_name, m.media_id, m.media_content "
        "FROM posts p JOIN authors a ON p.author_id = a.author_id "
        "JOIN channels c ON p.channel_id = c.channel_id "
        "JOIN sources s ON c.source_id = s.source_id "
        "LEFT JOIN media m ON m.post_id = p.post_id;";

    return m;
}

// Выполнить простой отчёт (без параметров)
json run_simple_report(PgPool &pool, const std::string &sql) {
    auto pconn = pool.acquire();
    pqxx::work tx(*pconn.conn);
    pqxx::result r = tx.exec(sql);
    tx.commit();
    json arr = json::array();
    for (const auto &row : r) arr.push_back(row_to_json(row));
    return arr;
}

// ---------- Основной HTTP сервер ----------
int main() {
    // создаём пул
    std::string connstr = make_conn_str();
    size_t pool_size = 4;
    try {
        // пробуем 6 попыток подключения (db может ещё подниматься)
        size_t attempts = 6;
        for (size_t i=0;i<attempts;i++) {
            try {
                PgPool pool(connstr, pool_size);
                // Если успешно — переход в основной цикл с этим пулом
                Server svr;

                // health
                svr.Get("/health", [](const Request&, Response& res) {
                    res.set_content("ok", "text/plain");
                });

                // список таблиц и PK доступны в глобальной области
                auto reports = reports_sql();

                // --- CREATE (POST /api/<table>) ---
                svr.Post(R"(/api/([A-Za-z_]+))", [&](const Request& req, Response& res) {
                    std::string table = req.matches[1];
                    if (!is_valid_table(table)) { res.status = 404; res.set_content("Table not found", "text/plain"); return; }

                    json body;
                    try { body = json::parse(req.body); }
                    catch(...) { res.status = 400; res.set_content("Invalid JSON", "text/plain"); return; }

                    auto pconn = pool.acquire();

                    if (table == "post_tags") {
                        if (!body.contains("post_id") || !body.contains("tag_id")) { res.status = 400; res.set_content("post_id and tag_id required", "text/plain"); return; }
                        std::string sql = "INSERT INTO post_tags (post_id, tag_id) VALUES ($1,$2) RETURNING post_id, tag_id;";
                        std::vector<std::string> params = { body["post_id"].is_null() ? "__NULL__" : body["post_id"].dump(),
                                                            body["tag_id"].is_null() ? "__NULL__" : body["tag_id"].dump() };
                        try {
                            pqxx::result r = exec_with_params_pool(pconn, sql, params);
                            json out = json::array();
                            for (const auto &row : r) out.push_back(row_to_json(row));
                            res.set_content(out.dump(), "application/json");
                        } catch(const std::exception &e){ res.status=500; res.set_content(e.what(),"text/plain"); }
                        return;
                    }

                    // generic insert
                    std::vector<std::string> cols;
                    std::vector<std::string> params;
                    for (auto it = body.begin(); it != body.end(); ++it) {
                        cols.push_back(it.key());
                        if (it.value().is_null()) params.push_back("__NULL__");
                        else if (it.value().is_string()) params.push_back(it.value().get<std::string>());
                        else params.push_back(it.value().dump());
                    }
                    if (cols.empty()) { res.status=400; res.set_content("No fields provided","text/plain"); return; }

                    std::string placeholders;
                    for (size_t i = 0; i < cols.size(); ++i) {
                        if (i) placeholders += ",";
                        placeholders += "$" + std::to_string(i+1);
                    }
                    std::string collist;
                    for (size_t i = 0; i < cols.size(); ++i) {
                        if (i) collist += ",";
                        collist += cols[i];
                    }

                    std::string sql = "INSERT INTO " + table + " (" + collist + ") VALUES (" + placeholders + ") RETURNING *;";
                    try {
                        pqxx::result r = exec_with_params_pool(pconn, sql, params);
                        json out = json::array();
                        for (const auto &row : r) out.push_back(row_to_json(row));
                        res.set_content(out.dump(), "application/json");
                    } catch(const std::exception &e) { res.status=500; res.set_content(std::string("DB error: ") + e.what(), "text/plain"); }
                });

                // --- READ single (GET /api/<table>/<id>) or composite for post_tags ---
                svr.Get(R"(/api/([A-Za-z_]+)/([0-9]+)(?:/([0-9]+))?)", [&](const Request& req, Response& res) {
                    std::string table = req.matches[1];
                    if (!is_valid_table(table)) { res.status = 404; res.set_content("Table not found", "text/plain"); return; }
                    auto pconn = pool.acquire();

                    if (table == "post_tags") {
                        if (!req.matches[2].matched || !req.matches[3].matched) { res.status=400; res.set_content("Need post_id and tag_id in path","text/plain"); return; }
                        std::string post_id = req.matches[2].str();
                        std::string tag_id = req.matches[3].str();
                        try {
                            std::string sql = "SELECT * FROM post_tags WHERE post_id=$1 AND tag_id=$2;";
                            pqxx::result r = exec_with_params_pool(pconn, sql, {post_id, tag_id});
                            if (r.empty()) { res.status = 404; res.set_content("Not found","text/plain"); return; }
                            res.set_content(row_to_json(r[0]).dump(), "application/json");
                        } catch(const std::exception &e) { res.status=500; res.set_content(e.what(),"text/plain"); }
                        return;
                    }

                    std::string id = req.matches[2].str();
                    auto it = pk_map.find(table);
                    if (it == pk_map.end()) { res.status=400; res.set_content("Table has no simple PK", "text/plain"); return; }
                    std::string pk = it->second;
                    try {
                        std::string sql = "SELECT * FROM " + table + " WHERE " + pk + "=$1;";
                        pqxx::result r = exec_with_params_pool(pconn, sql, {id});
                        if (r.empty()) { res.status = 404; res.set_content("Not found","text/plain"); return; }
                        res.set_content(row_to_json(r[0]).dump(), "application/json");
                    } catch(const std::exception &e) { res.status=500; res.set_content(e.what(),"text/plain"); }
                });

                // --- LIST (GET /api/<table>?limit=&offset=...) ---
                svr.Get(R"(/api/([A-Za-z_]+))", [&](const Request& req, Response& res) {
                    std::string table = req.matches[1];
                    if (!is_valid_table(table)) { res.status = 404; res.set_content("Table not found", "text/plain"); return; }
                    auto pconn = pool.acquire();

                    int limit = 100; int offset = 0;
                    if (req.has_param("limit")) limit = std::stoi(req.get_param_value("limit"));
                    if (req.has_param("offset")) offset = std::stoi(req.get_param_value("offset"));

                    if (table == "post_tags") {
                        std::string sql = "SELECT * FROM post_tags";
                        std::vector<std::string> params;
                        std::vector<std::string> conds;
                        if (req.has_param("post_id")) { conds.push_back("post_id=$1"); params.push_back(req.get_param_value("post_id")); }
                        if (req.has_param("tag_id"))  {
                            if (params.empty()) { conds.push_back("tag_id=$1"); params.push_back(req.get_param_value("tag_id")); }
                            else { conds.push_back("tag_id=$2"); params.push_back(req.get_param_value("tag_id")); }
                        }
                        if (!conds.empty()) {
                            sql += " WHERE ";
                            for (size_t i=0;i<conds.size();++i) {
                                if (i) sql += " AND ";
                                sql += conds[i];
                            }
                        }
                        sql += " LIMIT " + std::to_string(limit) + " OFFSET " + std::to_string(offset) + ";";
                        try {
                            pqxx::result r = params.empty() ? exec_with_params_pool(pconn, sql, {}) : exec_with_params_pool(pconn, sql, params);
                            json arr = json::array();
                            for (const auto &row : r) arr.push_back(row_to_json(row));
                            res.set_content(arr.dump(), "application/json");
                        } catch(const std::exception &e) { res.status=500; res.set_content(e.what(),"text/plain"); }
                        return;
                    }

                    std::string sql = "SELECT * FROM " + table + " LIMIT " + std::to_string(limit) + " OFFSET " + std::to_string(offset) + ";";
                    try {
                        pqxx::result r = exec_with_params_pool(pconn, sql, {});
                        json arr = json::array();
                        for (const auto &row : r) arr.push_back(row_to_json(row));
                        res.set_content(arr.dump(), "application/json");
                    } catch(const std::exception &e) { res.status=500; res.set_content(e.what(),"text/plain"); }
                });

                // --- UPDATE (PUT /api/<table>/<id>) ---
                svr.Put(R"(/api/([A-Za-z_]+)/([0-9]+)(?:/([0-9]+))?)", [&](const Request& req, Response& res) {
                    std::string table = req.matches[1];
                    if (!is_valid_table(table)) { res.status = 404; res.set_content("Table not found", "text/plain"); return; }
                    auto pconn = pool.acquire();

                    if (table == "post_tags") { res.status=400; res.set_content("post_tags has no update operation (use delete+insert)", "text/plain"); return; }

                    auto it = pk_map.find(table);
                    if (it == pk_map.end()) { res.status=400; res.set_content("Table has no simple PK", "text/plain"); return; }
                    std::string pk = it->second;
                    std::string id = req.matches[2].str();

                    json body;
                    try { body = json::parse(req.body); }
                    catch(...) { res.status=400; res.set_content("Invalid JSON", "text/plain"); return; }

                    std::vector<std::string> sets;
                    std::vector<std::string> params;
                    for (auto it = body.begin(); it != body.end(); ++it) {
                        if (it.key() == pk) continue;
                        sets.push_back(it.key() + "=$" + std::to_string(params.size() + 1));
                        if (it.value().is_null()) params.push_back("__NULL__");
                        else if (it.value().is_string()) params.push_back(it.value().get<std::string>());
                        else params.push_back(it.value().dump());
                    }
                    if (sets.empty()) { res.status=400; res.set_content("No fields to update", "text/plain"); return; }

                    std::string set_clause;
                    for (size_t i=0;i<sets.size();++i) { if (i) set_clause += ", "; set_clause += sets[i]; }
                    params.push_back(id);
                    std::string sql = "UPDATE " + table + " SET " + set_clause + " WHERE " + pk + "=$" + std::to_string(params.size()) + " RETURNING *;";

                    try {
                        pqxx::result r = exec_with_params_pool(pconn, sql, params);
                        if (r.empty()) { res.status=404; res.set_content("Not found", "text/plain"); return; }
                        res.set_content(row_to_json(r[0]).dump(), "application/json");
                    } catch(const std::exception &e) { res.status=500; res.set_content(e.what(),"text/plain"); }
                });

                // --- DELETE (DELETE /api/<table>/<id>) ---
                svr.Delete(R"(/api/([A-Za-z_]+)/([0-9]+)(?:/([0-9]+))?)", [&](const Request& req, Response& res) {
                    std::string table = req.matches[1];
                    if (!is_valid_table(table)) { res.status = 404; res.set_content("Table not found", "text/plain"); return; }
                    auto pconn = pool.acquire();

                    if (table == "post_tags") {
                        if (!req.matches[2].matched || !req.matches[3].matched) { res.status=400; res.set_content("Need post_id and tag_id in path","text/plain"); return; }
                        std::string post_id = req.matches[2].str();
                        std::string tag_id  = req.matches[3].str();
                        try {
                            std::string sql = "DELETE FROM post_tags WHERE post_id=$1 AND tag_id=$2 RETURNING *;";
                            pqxx::result r = exec_with_params_pool(pconn, sql, {post_id, tag_id});
                            if (r.empty()) { res.status=404; res.set_content("Not found","text/plain"); return; }
                            res.set_content(row_to_json(r[0]).dump(),"application/json");
                        } catch(const std::exception &e) { res.status=500; res.set_content(e.what(),"text/plain"); }
                        return;
                    }

                    auto it = pk_map.find(table);
                    if (it == pk_map.end()) { res.status=400; res.set_content("Table has no simple PK", "text/plain"); return; }
                    std::string pk = it->second;
                    std::string id = req.matches[2].str();

                    try {
                        std::string sql = "DELETE FROM " + table + " WHERE " + pk + "=$1 RETURNING *;";
                        pqxx::result r = exec_with_params_pool(pconn, sql, {id});
                        if (r.empty()) { res.status = 404; res.set_content("Not found","text/plain"); return; }
                        res.set_content(row_to_json(r[0]).dump(), "application/json");
                    } catch(const std::exception &e) { res.status=500; res.set_content(e.what(),"text/plain"); }
                });

                // --- Reports endpoint ---
                svr.Get(R"(/api/reports/([A-Za-z0-9_\-]+))", [&](const Request& req, Response& res) {
                    std::string name = req.matches[1];
                    auto it = reports.find(name);
                    if (it == reports.end()) { res.status = 404; res.set_content("Report not found", "text/plain"); return; }
                    try {
                        json out = run_simple_report(pool, it->second);
                        res.set_content(out.dump(), "application/json");
                    } catch(const std::exception &e) { res.status=500; res.set_content(e.what(),"text/plain"); }
                });

                std::cout << "Starting server at 0.0.0.0:8080\n";
                svr.listen("0.0.0.0", 8080);
                return 0;
            } catch (const std::exception &e) {
                std::cerr << "DB connection attempt failed: " << e.what() << "\n";
                if (i+1 < attempts) {
                    std::cerr << "Retrying in 3s...\n";
                    std::this_thread::sleep_for(std::chrono::seconds(3));
                    continue;
                } else {
                    std::cerr << "Failed to initialize DB pool, exiting.\n";
                    return 2;
                }
            }
        }
    } catch (const std::exception &e) {
        std::cerr << "Fatal error: " << e.what() << "\n";
        return 1;
    }

    return 0;
}
