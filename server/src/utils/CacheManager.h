#pragma once
#include <string>
#include <sw/redis++/redis++.h>

class CacheManager {
public:
  CacheManager(const std::string &redis_connection = "tcp://redis:6379");

  std::optional<std::string> get(const std::string &key);
  void setex(const std::string &key, int ttl, const std::string &value);
  void del(const std::string &key);
  void delPattern(const std::string &pattern);

private:
  sw::redis::Redis redis_;
};