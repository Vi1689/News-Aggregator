#pragma once
#include <bsoncxx/builder/stream/document.hpp>
#include <bsoncxx/json.hpp>
#include <mongocxx/client.hpp>
#include <mongocxx/database.hpp>
#include <mongocxx/instance.hpp>
#include <nlohmann/json.hpp>
#include <string>
#include <vector>

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
  MongoManager(const std::string &uri =
                   "mongodb://news_app:app_password@mongodb:27017/"
                   "news_aggregator?authSource=news_aggregator");

  // 🔍 ПОИСК
  std::vector<SearchResult> searchPosts(const std::string &query,
                                        int limit = 20);
  std::vector<SearchResult> searchByTags(const std::vector<std::string> &tags);
  json advancedSearch(const json &filters, int limit = 20);

  // 📊 АНАЛИТИКА (Aggregation Pipelines)
  json getDashboardStats();
  json getTopTags(int limit = 10);
  json getAuthorStats(int author_id);
  json getPostEngagementAnalysis(int days = 30);
  json getChannelPerformance();

  // 💡 РЕКОМЕНДАЦИИ
  std::vector<int> getSimilarPosts(int post_id, int limit = 5);

  // 🚫 ДЕДУБЛИКАЦИЯ
  bool isDuplicateContent(const std::string &content_hash);

  // ✍️ CRUD ОПЕРАЦИИ
  void indexPost(int post_id, const std::string &title,
                 const std::string &content,
                 const std::vector<std::string> &tags);
  void insertMany(const std::vector<json> &posts);
  void updatePostIndex(int post_id, const std::string &title,
                       const std::string &content,
                       const std::vector<std::string> &tags);
  void incrementViewCount(int post_id);
  void addTagToPost(int post_id, const std::string &tag);
  void removeTagFromPost(int post_id, const std::string &tag);
  void updatePostStats(int post_id, int likes_delta, int comments_delta);
  void removePostIndex(int post_id);
  bool upsertPost(int post_id, const json &post_data);
  void replacePost(int post_id, const json &post_data);

  // 👤 ПОЛЬЗОВАТЕЛЬСКИЕ ВЗАИМОДЕЙСТВИЯ
  void recordUserInteraction(const std::string &user_id, int post_id,
                             const std::string &action);
  json getUserHistory(const std::string &user_id, int limit = 50);

  // 📈 МАТЕРИАЛИЗОВАННЫЕ ПРЕДСТАВЛЕНИЯ (Витрины)
  void materializeTopPostsView();
  json getTopPostsFromView(int limit = 10);

  // 🔧 СЛУЖЕБНЫЕ
  void createIndexes();
  void createCollections();
};