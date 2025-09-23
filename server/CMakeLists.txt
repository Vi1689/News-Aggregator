cmake_minimum_required(VERSION 3.10)
project(news_server)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -O2 -pthread")

include_directories(${CMAKE_SOURCE_DIR}/include)

add_executable(server src/main.cpp)

target_link_libraries(server PRIVATE pqxx pq)
