#include "PgPool/PgPool.h"
#include "handlers/handlers.h"
#include "models/Constants.h"
#include "utils/CacheManager.h"
#include <chrono>
#include <httplib.h>
#include "mongo/MongoManager.h"
#include <iostream>
#include <thread>

int main() {
  httplib::Server svr;

  // Инициализация компонентов
  PgPool pool(constants::CONN_STRINGS, constants::POOL_SIZE);
  CacheManager cache;
  MongoManager mongo;
  Handlers handlers(pool, cache, mongo);

  // Настройка маршрутов
  handlers.setupRoutes(svr);

  // Health check в отдельном потоке
  std::thread health_checker([&pool]() {
    while (true) {
      std::this_thread::sleep_for(std::chrono::seconds(30));
      try {
        pool.health_check();
        std::cout << "Health check completed" << std::endl;
      } catch (const std::exception &e) {
        std::cerr << "Health check error: " << e.what() << std::endl;
      }
    }
  });
  health_checker.detach();

  std::cout << "Server starting on 0.0.0.0:8080" << std::endl;
  svr.listen("0.0.0.0", 8080);
  return 0;
}