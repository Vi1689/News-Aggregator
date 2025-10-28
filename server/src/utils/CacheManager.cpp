#include "CacheManager.h"

CacheManager::CacheManager(const std::string& redis_connection) 
    : redis_(redis_connection) {}

std::optional<std::string> CacheManager::get(const std::string& key) {
    return redis_.get(key);
}

void CacheManager::setex(const std::string& key, int ttl, const std::string& value) {
    redis_.setex(key, std::chrono::seconds(ttl), value);
}

void CacheManager::del(const std::string& key) {
    redis_.del(key);
}

void CacheManager::delPattern(const std::string& pattern) {
    // Будет реализовано при необходимости
}