#pragma once
#include "../PgPool/PgPool.h"
#include "../utils/CacheManager.h"
#include <httplib.h>

class Handlers {
public:
  Handlers(PgPool &pool, CacheManager &cache);

  void setupRoutes(httplib::Server &svr);

private:
  PgPool &pool_;
  CacheManager &cache_;

  // Методы-обработчики
  void createHandler(const httplib::Request &req, httplib::Response &res);
  void readAllHandler(const httplib::Request &req, httplib::Response &res);
  void readOneHandler(const httplib::Request &req, httplib::Response &res);
  void updateHandler(const httplib::Request &req, httplib::Response &res);
  void deleteHandler(const httplib::Request &req, httplib::Response &res);

  // Вспомогательные методы
  void handlePostTags(const httplib::Request &req, httplib::Response &res,
                      bool isDelete = false);
};