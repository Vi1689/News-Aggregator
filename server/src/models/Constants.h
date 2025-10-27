#pragma once
#include <string>
#include <unordered_map>
#include <vector>

namespace constants {
    extern const std::vector<std::string> CONN_STRINGS;
    extern const std::unordered_map<std::string, std::string> pk_map;
    extern const std::vector<std::string> valid_tables;
    
    bool is_valid_table(const std::string &t);
}