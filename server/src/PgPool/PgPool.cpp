#include "PgPool.h"
#include "../models/Constants.h"
#include <iostream>
#include <thread>
#include <chrono>

// Реализация деструктора PConn должна быть после полного определения PgPool
PConn::~PConn() {
    if (conn && pool)
        pool->release(conn, is_replica);
}

PgPool::PgPool(const std::vector<std::string> &conn_infos, size_t pool_size) {
    for (const auto &conninfo : conn_infos) {
        for (size_t i = 0; i < pool_size; ++i) {
            bool connected = false;
            int attempts = 0;
            const int max_attempts = 3;
            
            while (!connected && attempts < max_attempts) {
                try {
                    auto conn = std::make_shared<pqxx::connection>(conninfo);
                    if (!conn->is_open()) {
                        std::cerr << "Failed to open DB connection: " << conninfo << std::endl;
                        attempts++;
                        if (attempts < max_attempts) {
                            std::this_thread::sleep_for(std::chrono::seconds(2));
                        }
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
                        connected = true;
                    } catch (const std::exception &e) {
                        std::cerr << "Error checking DB role: " << e.what() << std::endl;
                        replica_pool_.push(conn);
                        connected = true;
                    }
                } catch (const std::exception &e) {
                    std::cerr << "Failed to create connection (attempt " << (attempts + 1) 
                              << "/" << max_attempts << "): " << e.what() << std::endl;
                    attempts++;
                    if (attempts < max_attempts) {
                        std::this_thread::sleep_for(std::chrono::seconds(2));
                    }
                }
            }
            
            if (!connected) {
                std::cerr << "Giving up on connection: " << conninfo << std::endl;
            }
        }
    }

    if (master_pool_.empty() && replica_pool_.empty()) {
        throw std::runtime_error("No valid database connections available");
    }
    
    std::cout << "PgPool initialized: " << master_pool_.size() 
              << " master connections, " << replica_pool_.size() 
              << " replica connections" << std::endl;
}

PConn PgPool::acquire(bool read_only) {
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
        
        // Если это read-only операция и реплик нет, логируем это
        if (read_only && replica_pool_.empty()) {
            std::cout << "No replica available, using MASTER for READ operation" << std::endl;
        }
        
        return PConn{conn, this, false};
    }

    // Если мастера нет, но операция read-only - используем реплику
    if (read_only && !replica_pool_.empty()) {
        auto conn = replica_pool_.front();
        replica_pool_.pop();
        return PConn{conn, this, true};
    }

    // Ждем доступное соединение с ТАЙМАУТОМ (10 секунд)
    auto timeout = std::chrono::seconds(10);
    if (!cv_.wait_for(lk, timeout, [&] {
        return (!master_pool_.empty()) || (read_only && !replica_pool_.empty());
    })) {
        // Таймаут - бросаем исключение с информацией о состоянии
        std::string error_msg = "Timeout waiting for database connection (";
        error_msg += read_only ? "READ" : "WRITE";
        error_msg += " operation). Pool status: ";
        error_msg += std::to_string(master_pool_.size()) + " master, ";
        error_msg += std::to_string(replica_pool_.size()) + " replica connections available";
        throw std::runtime_error(error_msg);
    }

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

    std::cout << "Starting health check..." << std::endl;

    // Сохраняем оригинальные строки подключения для переподключения
    static std::string master_conninfo = constants::CONN_STRINGS[0];
    static std::string replica_conninfo = constants::CONN_STRINGS.size() > 1 ? constants::CONN_STRINGS[1] : "";

    // Проверяем соединения мастера
    std::queue<std::shared_ptr<pqxx::connection>> new_master_pool;
    while (!master_pool_.empty()) {
        auto conn = master_pool_.front();
        master_pool_.pop();
        if (check_connection(conn)) {
            new_master_pool.push(conn);
        } else {
            std::cerr << "Master connection failed health check" << std::endl;
        }
    }
    
    // Если мастеров нет или их мало, пытаемся переподключиться
    if (new_master_pool.size() < constants::POOL_SIZE) {
        std::cout << "Master pool is low (" << new_master_pool.size() 
                  << "), attempting to reconnect..." << std::endl;
        
        int attempts = 0;
        const int max_attempts = 2;
        
        while (new_master_pool.size() < constants::POOL_SIZE && attempts < max_attempts) {
            try {
                auto new_conn = std::make_shared<pqxx::connection>(master_conninfo);
                if (new_conn->is_open()) {
                    pqxx::work txn(*new_conn);
                    pqxx::result r = txn.exec("SELECT pg_is_in_recovery()");
                    bool is_replica = r[0][0].as<bool>();
                    if (!is_replica) {
                        new_master_pool.push(new_conn);
                        std::cout << "Successfully reconnected to MASTER" << std::endl;
                    } else {
                        std::cerr << "Connected to replica instead of master" << std::endl;
                    }
                }
            } catch (const std::exception& e) {
                std::cerr << "Failed to reconnect to master (attempt " 
                          << (attempts + 1) << "): " << e.what() << std::endl;
                attempts++;
                if (attempts < max_attempts) {
                    std::this_thread::sleep_for(std::chrono::seconds(1));
                }
            }
        }
    }
    
    master_pool_ = std::move(new_master_pool);

    // Проверяем соединения реплик - с возможностью переподключения
    std::queue<std::shared_ptr<pqxx::connection>> new_replica_pool;
    
    // Сначала проверяем существующие соединения реплик
    while (!replica_pool_.empty()) {
        auto conn = replica_pool_.front();
        replica_pool_.pop();
        if (check_connection(conn)) {
            new_replica_pool.push(conn);
        } else {
            std::cerr << "Replica connection failed health check" << std::endl;
        }
    }
    
    // Если реплик нет или их мало, пытаемся переподключиться
    if (!replica_conninfo.empty() && new_replica_pool.size() < constants::POOL_SIZE) {
        std::cout << "Replica pool is low (" << new_replica_pool.size() 
                  << "), attempting to reconnect..." << std::endl;
        
        int attempts = 0;
        const int max_attempts = 2;
        
        while (new_replica_pool.size() < constants::POOL_SIZE && attempts < max_attempts) {
            try {
                auto new_conn = std::make_shared<pqxx::connection>(replica_conninfo);
                if (new_conn->is_open()) {
                    pqxx::work txn(*new_conn);
                    pqxx::result r = txn.exec("SELECT pg_is_in_recovery()");
                    bool is_replica = r[0][0].as<bool>();
                    if (is_replica) {
                        new_replica_pool.push(new_conn);
                        std::cout << "Successfully reconnected to REPLICA" << std::endl;
                    } else {
                        std::cerr << "Connected to master instead of replica" << std::endl;
                    }
                }
            } catch (const std::exception& e) {
                std::cerr << "Failed to reconnect to replica (attempt " 
                          << (attempts + 1) << "): " << e.what() << std::endl;
                attempts++;
                if (attempts < max_attempts) {
                    std::this_thread::sleep_for(std::chrono::seconds(1));
                }
            }
        }
    }
    
    replica_pool_ = std::move(new_replica_pool);

    // Логируем состояние пула
    std::cout << "Health check completed: " << master_pool_.size() << " master, " 
              << replica_pool_.size() << " replica connections" << std::endl;
    
    cv_.notify_all();
}

bool PgPool::check_connection(std::shared_ptr<pqxx::connection> conn) {
    if (!conn->is_open()) {
        return false;
    }
    try {
        pqxx::work txn(*conn);
        txn.exec("SELECT 1");
        return true;
    } catch (const std::exception& e) {
        std::cerr << "Connection check failed: " << e.what() << std::endl;
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