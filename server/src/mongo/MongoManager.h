#pragma once
#include <mongocxx/client.hpp>
#include <mongocxx/instance.hpp>
#include <mongocxx/database.hpp>
#include <bsoncxx/builder/stream/document.hpp>
#include <bsoncxx/json.hpp>
#include <string>
#include <vector>
#include <nlohmann/json.hpp>

using json = nlohmann::json;

struct SearchResult {
    int id;
    std::string title;
    std::string preview;
    double relevance;
    std::vector<std::string> matched_tags;
};

class MongoManager {
private:
    static mongocxx::instance instance;
    mongocxx::client client;
    mongocxx::database db;

public:
    MongoManager(const std::string& uri = "mongodb://news_app:app_password@mongodb:27017/news_aggregator?authSource=news_aggregator");
    
    // 🔍 ПОИСК
    std::vector<SearchResult> searchPosts(const std::string& query, int limit = 20);
    std::vector<SearchResult> searchByTags(const std::vector<std::string>& tags);
    
    // 📊 АНАЛИТИКА
    json getDashboardStats();
    json getTopTags(int limit = 10);
    json getAuthorStats(int author_id);
    
    // 💡 РЕКОМЕНДАЦИИ
    std::vector<int> getSimilarPosts(int post_id, int limit = 5);
    
    // 🚫 ДЕДУБЛИКАЦИЯ
    bool isDuplicateContent(const std::string& content_hash);
    void indexPost(int post_id, const std::string& title, 
                   const std::string& content, const std::vector<std::string>& tags);
    
    // 📝 ОБНОВЛЕНИЕ ИНДЕКСА
    void updatePostIndex(int post_id, const std::string& title, 
                        const std::string& content, const std::vector<std::string>& tags);
    void removePostIndex(int post_id);
    
    // 🔧 СЛУЖЕБНЫЕ
    void createIndexes();
};