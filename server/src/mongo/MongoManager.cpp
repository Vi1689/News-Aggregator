#include "MongoManager.h"
#include <iostream>
#include <algorithm>

// Статический инстанс
mongocxx::instance MongoManager::instance{};

MongoManager::MongoManager(const std::string& uri) {
    // Получаем URI из переменных окружения или используем переданный
    const char* env_uri = std::getenv("MONGODB_URI");
    std::string connection_uri = env_uri ? env_uri : uri;
    
    std::cout << "Connecting to MongoDB: " << connection_uri << std::endl;
    
    try {
        client = mongocxx::client{mongocxx::uri{connection_uri}};
        db = client["news_aggregator"];
        createIndexes();
        std::cout << "MongoDB connected successfully" << std::endl;
    } catch (const std::exception& e) {
        std::cerr << "MongoDB connection failed: " << e.what() << std::endl;
        throw;
    }
}

void MongoManager::createIndexes() {
    auto posts = db["posts"];
    
    // Текстовый индекс для поиска
    bsoncxx::builder::stream::document text_index_builder;
    text_index_builder << "title" << "text" 
                      << "content" << "text"
                      << "tags" << "text";
    
    mongocxx::options::index index_options{};
    index_options.weights(bsoncxx::builder::stream::document{}
        << "title" << 10
        << "content" << 5  
        << "tags" << 3
        << bsoncxx::builder::stream::finalize);
    
    posts.create_index(text_index_builder.view(), index_options);
    
    // Индекс для дедубликации
    bsoncxx::builder::stream::document hash_index_builder;
    hash_index_builder << "content_hash" << 1;
    posts.create_index(hash_index_builder.view());
    
    std::cout << "MongoDB indexes created" << std::endl;
}

bool MongoManager::isDuplicateContent(const std::string& content_hash) {
    auto posts = db["posts"];
    auto result = posts.find_one(
        bsoncxx::builder::stream::document{}
            << "content_hash" << content_hash
            << bsoncxx::builder::stream::finalize
    );
    return static_cast<bool>(result);
}

void MongoManager::indexPost(int post_id, const std::string& title, 
                           const std::string& content, const std::vector<std::string>& tags) {
    auto posts = db["posts"];
    
    // Генерируем хеш контента
    std::string content_hash = std::to_string(
        std::hash<std::string>{}(title + content)
    );
    
    auto doc = bsoncxx::builder::stream::document{}
        << "post_id" << post_id
        << "title" << title
        << "content" << content
        << "content_hash" << content_hash
        << "tags" << bsoncxx::builder::stream::open_array
            << [&tags](bsoncxx::builder::stream::array_context<> arr) {
                for (const auto& tag : tags) {
                    arr << tag;
                }
            }
        << bsoncxx::builder::stream::close_array
        << "indexed_at" << bsoncxx::types::b_date{std::chrono::system_clock::now()}
        << bsoncxx::builder::stream::finalize;
    
    posts.insert_one(doc.view());
}

std::vector<SearchResult> MongoManager::searchPosts(const std::string& query, int limit) {
    auto posts = db["posts"];
    std::vector<SearchResult> results;
    
    try {
        auto cursor = posts.find(
            bsoncxx::builder::stream::document{}
                << "$text" << bsoncxx::builder::stream::open_document
                    << "$search" << query
                    << "$language" << "russian"
                << bsoncxx::builder::stream::close_document
                << bsoncxx::builder::stream::finalize,
            mongocxx::options::find{}.projection(
                bsoncxx::builder::stream::document{}
                    << "post_id" << 1
                    << "title" << 1
                    << "content" << 1
                    << "tags" << 1
                    << "score" << bsoncxx::builder::stream::open_document
                        << "$meta" << "textScore"
                    << bsoncxx::builder::stream::close_document
                    << bsoncxx::builder::stream::finalize
            ).sort(
                bsoncxx::builder::stream::document{}
                    << "score" << bsoncxx::builder::stream::open_document
                        << "$meta" << "textScore"
                    << bsoncxx::builder::stream::close_document
                    << bsoncxx::builder::stream::finalize
            ).limit(limit)
        );
        
        for (auto&& doc : cursor) {
            SearchResult result;
            result.id = doc["post_id"].get_int32();
            result.title = doc["title"].get_string().value.to_string();
            
            // Создаем превью контента
            std::string content = doc["content"].get_string().value.to_string();
            result.preview = content.substr(0, 200) + "...";
            
            if (doc["score"]) {
                result.relevance = doc["score"].get_double();
            }
            
            // Получаем теги
            if (doc["tags"] && doc["tags"].type() == bsoncxx::type::k_array) {
                auto tags_array = doc["tags"].get_array().value;
                for (auto&& tag : tags_array) {
                    result.matched_tags.push_back(tag.get_string().value.to_string());
                }
            }
            
            results.push_back(result);
        }
    } catch (const std::exception& e) {
        std::cerr << "MongoDB search error: " << e.what() << std::endl;
    }
    
    return results;
}

void MongoManager::updatePostIndex(int post_id, const std::string& title, 
                                 const std::string& content, const std::vector<std::string>& tags) {
    auto posts = db["posts"];
    
    std::string content_hash = std::to_string(
        std::hash<std::string>{}(title + content)
    );
    
    auto doc = bsoncxx::builder::stream::document{}
        << "$set" << bsoncxx::builder::stream::open_document
            << "title" << title
            << "content" << content
            << "content_hash" << content_hash
            << "tags" << bsoncxx::builder::stream::open_array
                << [&tags](bsoncxx::builder::stream::array_context<> arr) {
                    for (const auto& tag : tags) {
                        arr << tag;
                    }
                }
            << bsoncxx::builder::stream::close_array
            << "updated_at" << bsoncxx::types::b_date{std::chrono::system_clock::now()}
        << bsoncxx::builder::stream::close_document
        << bsoncxx::builder::stream::finalize;
    
    posts.update_one(
        bsoncxx::builder::stream::document{} 
            << "post_id" << post_id 
            << bsoncxx::builder::stream::finalize,
        doc.view()
    );
}

void MongoManager::removePostIndex(int post_id) {
    auto posts = db["posts"];
    posts.delete_one(
        bsoncxx::builder::stream::document{} 
            << "post_id" << post_id 
            << bsoncxx::builder::stream::finalize
    );
}