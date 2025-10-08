#include <iostream>
#include <string>
#include <cstdlib>
#include <pqxx/pqxx>
#include <sw/redis++/redis++.h>
#include <httplib.h>
#include <nlohmann/json.hpp>

using namespace sw::redis;

int main() {
    try {
        // Получаем настройки из переменных окружения
        const char* db_host = std::getenv("DB_HOST");
        const char* db_port = std::getenv("DB_PORT");
        const char* db_name = std::getenv("DB_NAME");
        const char* db_user = std::getenv("DB_USER");
        const char* db_pass = std::getenv("DB_PASS");

        const char* redis_host = std::getenv("REDIS_HOST");
        const char* redis_port = std::getenv("REDIS_PORT");

        // PostgreSQL соединение
        std::string conn_str = "host=" + std::string(db_host) +
                               " port=" + std::string(db_port) +
                               " dbname=" + std::string(db_name) +
                               " user=" + std::string(db_user) +
                               " password=" + std::string(db_pass);
        pqxx::connection pg_conn(conn_str);

        // Redis соединение
        auto redis = Redis("tcp://" + std::string(redis_host) + ":" + std::string(redis_port));

        httplib::Server svr;

        // POST /api/posts - создаем пост
        svr.Post("/api/posts", [&](const httplib::Request& req, httplib::Response& res){
            try {
                auto j = nlohmann::json::parse(req.body);
                std::string title = j.value("title", "");
                int author_id = j.value("author_id", 0);
                int channel_id = j.value("channel_id", 0);

                pqxx::work txn(pg_conn);
                txn.exec_params(
                    "INSERT INTO posts (title, author_id, channel_id) VALUES ($1, $2, $3) RETURNING post_id",
                    title,
                    author_id == 0 ? pqxx::null{} : author_id,
                    channel_id == 0 ? pqxx::null{} : channel_id
                );
                auto result = txn.exec("SELECT currval(pg_get_serial_sequence('posts','post_id'))");
                int post_id = result[0][0].as<int>();
                txn.commit();

                // Кэширование в Redis
                redis.set("post:" + std::to_string(post_id), title);

                res.set_content("{\"status\":\"created\",\"post_id\":" + std::to_string(post_id) + "}", "application/json");
            } catch (const std::exception& e) {
                res.status = 500;
                res.set_content("{\"error\":\"" + std::string(e.what()) + "\"}", "application/json");
            }
        });

        // GET /api/posts/:id - получаем пост
        svr.Get(R"(/api/posts/(\d+))", [&](const httplib::Request& req, httplib::Response& res){
            try {
                int post_id = std::stoi(req.matches[1]);

                // Сначала проверяем в Redis
                auto val = redis.get("post:" + std::to_string(post_id));
                if (val) {
                    res.set_content("{\"post_id\":" + std::to_string(post_id) + ",\"title\":\"" + *val + "\"}", "application/json");
                    return;
                }

                // Иначе из PostgreSQL
                pqxx::work txn(pg_conn);
                auto result = txn.exec_params("SELECT title FROM posts WHERE post_id=$1", post_id);
                if (result.size() == 0) {
                    res.status = 404;
                    res.set_content("{\"error\":\"Post not found\"}", "application/json");
                    return;
                }
                std::string title = result[0][0].as<std::string>();
                // Кэшируем
                redis.set("post:" + std::to_string(post_id), title);

                res.set_content("{\"post_id\":" + std::to_string(post_id) + ",\"title\":\"" + title + "\"}", "application/json");
            } catch (const std::exception& e) {
                res.status = 500;
                res.set_content("{\"error\":\"" + std::string(e.what()) + "\"}", "application/json");
            }
        });

        std::cout << "Server running at 0.0.0.0:8080\n";
        svr.listen("0.0.0.0", 8080);

    } catch (const std::exception &e) {
        std::cerr << "Fatal: " << e.what() << "\n";
        return 1;
    }
}
