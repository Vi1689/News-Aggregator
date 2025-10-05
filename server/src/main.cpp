#include <iostream>
#include <pqxx/pqxx>
#include <sw/redis++/redis++.h>
#include <httplib.h>
#include <mutex>
#include <queue>
#include <memory>
#include <sstream>
#include <cstdlib>

// ==========================
// Connection Pool for PostgreSQL
// ==========================
class ConnectionPool {
public:
    ConnectionPool(const std::string& conn_str, size_t pool_size = 10)
        : conn_str_(conn_str) {
        for (size_t i = 0; i < pool_size; ++i)
            pool_.push(std::make_shared<pqxx::connection>(conn_str_));
    }

    std::shared_ptr<pqxx::connection> acquire() {
        std::unique_lock<std::mutex> lock(mtx_);
        if (pool_.empty())
            return std::make_shared<pqxx::connection>(conn_str_);
        auto conn = pool_.front();
        pool_.pop();
        return conn;
    }

    void release(std::shared_ptr<pqxx::connection> conn) {
        std::unique_lock<std::mutex> lock(mtx_);
        pool_.push(conn);
    }

private:
    std::string conn_str_;
    std::queue<std::shared_ptr<pqxx::connection>> pool_;
    std::mutex mtx_;
};

// ==========================
// Utility
// ==========================
std::string env_or(const char* name, const char* def) {
    const char* v = std::getenv(name);
    return v ? v : def;
}

// ==========================
// Server main
// ==========================
int main() {
    try {
        // --- Connect to PostgreSQL ---
        std::string db_conn =
            "host=" + env_or("DB_HOST", "localhost") +
            " port=" + env_or("DB_PORT", "5432") +
            " dbname=" + env_or("DB_NAME", "news_db") +
            " user=" + env_or("DB_USER", "news_user") +
            " password=" + env_or("DB_PASS", "news_pass");

        ConnectionPool pool(db_conn, 8);
        std::cout << "[DB] Connection pool initialized\n";

        // --- Connect to Redis ---
        std::ostringstream redis_uri;
        redis_uri << "tcp://" << env_or("REDIS_HOST", "localhost")
                  << ":" << env_or("REDIS_PORT", "6379");
        sw::redis::Redis redis(redis_uri.str());
        std::cout << "[Redis] Connected\n";

        // --- Start HTTP server ---
        httplib::Server svr;

        // ==========================
        // CRUD: POSTS
        // ==========================

        // GET /api/posts
        svr.Get("/api/posts", [&](const httplib::Request&, httplib::Response& res) {
            auto conn = pool.acquire();
            pqxx::work txn(*conn);
            pqxx::result r = txn.exec("SELECT post_id, title, likes_count, created_at FROM posts ORDER BY post_id DESC");
            std::ostringstream json;
            json << "[";
            bool first = true;
            for (auto row : r) {
                if (!first) json << ",";
                first = false;
                json << "{"
                     << "\"id\":" << row["post_id"].as<int>() << ","
                     << "\"title\":\"" << row["title"].as<std::string>() << "\","
                     << "\"likes\":" << row["likes_count"].as<int>() << ","
                     << "\"created_at\":\"" << row["created_at"].as<std::string>() << "\"}";
            }
            json << "]";
            res.set_content(json.str(), "application/json");
            pool.release(conn);
        });

        // POST /api/posts
        svr.Post("/api/posts", [&](const httplib::Request& req, httplib::Response& res) {
            auto conn = pool.acquire();
            pqxx::work txn(*conn);
            auto params = httplib::Params(req.body);
            std::string title = params["title"];
            int author_id = std::stoi(params["author_id"]);
            int text_id = std::stoi(params["text_id"]);
            int channel_id = std::stoi(params["channel_id"]);
            txn.exec_params("INSERT INTO posts (title, author_id, text_id, channel_id) VALUES ($1, $2, $3, $4)",
                            title, author_id, text_id, channel_id);
            txn.commit();
            res.set_content("{\"status\":\"created\"}", "application/json");
            pool.release(conn);
        });

        // PUT /api/posts/:id
        svr.Put(R"(/api/posts/(\d+))", [&](const httplib::Request& req, httplib::Response& res) {
            int id = std::stoi(req.matches[1]);
            auto conn = pool.acquire();
            pqxx::work txn(*conn);
            auto params = httplib::Params(req.body);
            std::string title = params["title"];
            txn.exec_params("UPDATE posts SET title=$1 WHERE post_id=$2", title, id);
            txn.commit();
            res.set_content("{\"status\":\"updated\"}", "application/json");
            pool.release(conn);
        });

        // DELETE /api/posts/:id
        svr.Delete(R"(/api/posts/(\d+))", [&](const httplib::Request& req, httplib::Response& res) {
            int id = std::stoi(req.matches[1]);
            auto conn = pool.acquire();
            pqxx::work txn(*conn);
            txn.exec_params("DELETE FROM posts WHERE post_id=$1", id);
            txn.commit();
            res.set_content("{\"status\":\"deleted\"}", "application/json");
            pool.release(conn);
        });

        // ==========================
        // Business Case: Aggregation Example
        // ==========================
        svr.Get("/api/analytics/top-tags", [&](const httplib::Request&, httplib::Response& res) {
            auto conn = pool.acquire();
            pqxx::work txn(*conn);
            pqxx::result r = txn.exec(
                "SELECT t.name, COUNT(*) AS tag_usage "
                "FROM post_tags pt "
                "JOIN tags t ON pt.tag_id = t.tag_id "
                "GROUP BY t.name "
                "ORDER BY tag_usage DESC "
                "LIMIT 5;"
            );

            std::ostringstream json;
            json << "[";
            bool first = true;
            for (auto row : r) {
                if (!first) json << ",";
                first = false;
                json << "{"
                     << "\"tag\":\"" << row["name"].as<std::string>() << "\","
                     << "\"count\":" << row["tag_usage"].as<int>() << "}";
            }
            json << "]";
            res.set_content(json.str(), "application/json");
            pool.release(conn);
        });

        // ==========================
        // Like post (Redis example)
        // ==========================
        svr.Post(R"(/api/posts/(\d+)/like)", [&](const httplib::Request& req, httplib::Response& res) {
            int post_id = std::stoi(req.matches[1]);
            std::string key = "post:" + std::to_string(post_id) + ":likes";

            long long new_likes = redis.incr(key);

            // Sync to Postgres
            auto conn = pool.acquire();
            pqxx::work txn(*conn);
            txn.exec_params("UPDATE posts SET likes_count=$1 WHERE post_id=$2", new_likes, post_id);
            txn.commit();
            pool.release(conn);

            res.set_content("{\"new_likes\":" + std::to_string(new_likes) + "}", "application/json");
        });

        std::cout << "[Server] Running on port 8080\n";
        svr.listen("0.0.0.0", 8080);

    } catch (const std::exception& e) {
        std::cerr << "Fatal error: " << e.what() << "\n";
        return 1;
    }
}
