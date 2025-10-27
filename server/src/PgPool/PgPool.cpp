#include "PgPool.h"
#include <iostream>

// Реализация деструктора PConn должна быть после полного определения PgPool
PConn::~PConn() {
    if (conn && pool)
        pool->release(conn, is_replica);
}

PgPool::PgPool(const std::vector<std::string> &conn_infos, size_t pool_size) {
    for (const auto &conninfo : conn_infos) {
        for (size_t i = 0; i < pool_size; ++i) {
            try {
                auto conn = std::make_shared<pqxx::connection>(conninfo);
                if (!conn->is_open()) {
                    std::cerr << "Failed to open DB connection: " << conninfo << std::endl;
                    continue;
                }
                
                try {
                    pqxx::work txn(*conn);
                    pqxx::result r = txn.exec("SELECT pg_is_in_recovery()");
                    bool is_replica = r[0][0].as<bool>();
                    if (is_replica) {
                        std::cout << "Added REPLICA connection: " << conninfo << std::endl;
                        replica_pool_.push(conn);
                    } else {
                        std::cout << "Added MASTER connection: " << conninfo << std::endl;
                        master_pool_.push(conn);
                    }
                } catch (const std::exception &e) {
                    std::cerr << "Error checking DB role: " << e.what() << std::endl;
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

PConn PgPool::acquire(bool read_only) {
    std::unique_lock<std::mutex> lk(mutex_);

    if (read_only && !replica_pool_.empty()) {
        auto conn = replica_pool_.front();
        replica_pool_.pop();
        return PConn{conn, this, true};
    }

    if (!master_pool_.empty()) {
        auto conn = master_pool_.front();
        master_pool_.pop();
        return PConn{conn, this, false};
    }

    if (read_only && !replica_pool_.empty()) {
        auto conn = replica_pool_.front();
        replica_pool_.pop();
        return PConn{conn, this, true};
    }

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

void PgPool::health_check() {
    std::unique_lock<std::mutex> lk(mutex_);

    std::queue<std::shared_ptr<pqxx::connection>> new_master_pool;
    while (!master_pool_.empty()) {
        auto conn = master_pool_.front();
        master_pool_.pop();
        if (check_connection(conn)) {
            new_master_pool.push(conn);
        }
    }
    master_pool_ = std::move(new_master_pool);

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

bool PgPool::check_connection(std::shared_ptr<pqxx::connection> conn) {
    if (!conn->is_open()) return false;
    try {
        pqxx::work txn(*conn);
        txn.exec("SELECT 1");
        return true;
    } catch (...) {
        return false;
    }
}

void PgPool::release(std::shared_ptr<pqxx::connection> conn, bool is_replica) {
    std::unique_lock<std::mutex> lk(mutex_);
    if (is_replica) {
        replica_pool_.push(conn);
    } else {
        master_pool_.push(conn);
    }
    lk.unlock();
    cv_.notify_one();
}