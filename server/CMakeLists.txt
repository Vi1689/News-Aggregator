cmake_minimum_required(VERSION 3.16)
project(news_server)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_BUILD_TYPE Release)
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -O2 -pthread")

find_package(PkgConfig REQUIRED)
pkg_check_modules(PQXX REQUIRED libpqxx)
pkg_check_modules(PQ REQUIRED libpq)

# find redis++ via CMake config (installed by project build in Dockerfile)
find_package(redis++ REQUIRED)
find_package(Threads REQUIRED)

include_directories(
    ${CMAKE_SOURCE_DIR}/include
    ${PQXX_INCLUDE_DIRS}
    ${PQ_INCLUDE_DIRS}
)

file(GLOB_RECURSE SRC_FILES src/*.cpp)

add_executable(server ${SRC_FILES})

target_link_libraries(server PRIVATE
    ${PQXX_LIBRARIES}
    ${PQ_LIBRARIES}
    redis++::redis++
    Threads::Threads
)
