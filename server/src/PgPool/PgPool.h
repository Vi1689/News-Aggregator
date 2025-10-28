#pragma once
#include "../models/Types.h"
#include <condition_variable>
#include <memory>
#include <mutex>
#include <pqxx/pqxx>
#include <queue>
#include <string>
#include <vector>

class PgPool {
public:
  PgPool(const std::vector<std::string> &conn_infos, size_t pool_size = 4);
  PConn acquire(bool read_only = false);
  void health_check();
  void release(std::shared_ptr<pqxx::connection> conn, bool is_replica);

private:
  bool check_connection(std::shared_ptr<pqxx::connection> conn);

  std::queue<std::shared_ptr<pqxx::connection>> master_pool_;
  std::queue<std::shared_ptr<pqxx::connection>> replica_pool_;
  std::mutex mutex_;
  std::condition_variable cv_;
};