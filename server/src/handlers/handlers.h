#pragma once
#include "../PgPool/PgPool.h"
#include "../mongo/MongoManager.h"
#include "../utils/CacheManager.h"
#include <httplib.h>

class Handlers {
public:
  Handlers(PgPool &pool, CacheManager &cache, MongoManager &mongo);

  void setupRoutes(httplib::Server &svr);

private:
  PgPool &pool_;
  CacheManager &cache_;
  MongoManager &mongo_;

  // Методы-обработчики
  void createHandler(const httplib::Request &req, httplib::Response &res);
  void readAllHandler(const httplib::Request &req, httplib::Response &res);
  void readOneHandler(const httplib::Request &req, httplib::Response &res);
  void updateHandler(const httplib::Request &req, httplib::Response &res);
  void deleteHandler(const httplib::Request &req, httplib::Response &res);

  // Вспомогательные методы
  void handlePostTags(const httplib::Request &req, httplib::Response &res,
                      bool isDelete = false);

  void advancedSearchHandler(const httplib::Request &req,
                             httplib::Response &res);
  void topTagsHandler(const httplib::Request &req, httplib::Response &res);
  void engagementAnalysisHandler(const httplib::Request &req,
                                 httplib::Response &res);
  void userHistoryHandler(const httplib::Request &req, httplib::Response &res);
  void topPostsViewHandler(const httplib::Request &req, httplib::Response &res);
  void postOperationsHandler(const httplib::Request &req,
                             httplib::Response &res);
  void channelPerformanceHandler(const httplib::Request &req,
                                 httplib::Response &res);
  void materializeViewHandler(const httplib::Request &req,
                              httplib::Response &res);
};