#pragma once
#include <string>
#include <unordered_map>
#include <vector>

namespace constants {
    extern const std::vector<std::string> CONN_STRINGS;
    extern const std::unordered_map<std::string, std::string> pk_map;
    extern const std::vector<std::string> valid_tables;
    extern const size_t POOL_SIZE;  // Добавляем константу размера пула
    
    bool is_valid_table(const std::string &t);
}