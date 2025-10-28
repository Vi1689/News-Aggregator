#pragma once
#include <memory>
#include <pqxx/pqxx>

// Предварительное объявление класса PgPool
class PgPool;

struct PConn {
  std::shared_ptr<pqxx::connection> conn;
  PgPool *pool;
  bool is_replica;
  ~PConn();
};