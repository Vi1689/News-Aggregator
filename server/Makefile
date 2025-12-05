CXX = g++
INCLUDES = -I./cpp-httplib -Isrc -I/usr/local/include -I/usr/local/include/mongocxx/v_noabi -I/usr/local/include/bsoncxx/v_noabi
CXXFLAGS = -std=c++17 -Wall -O2
LDFLAGS = -lpqxx -lpq -pthread -lredis++ -lhiredis -lmongocxx -lbsoncxx -lmongoc-1.0 -lbson-1.0

SRC = src/models/Constants.cpp \
      src/utils/CacheManager.cpp \
      src/PgPool/PgPool.cpp \
      src/handlers/handlers.cpp \
      src/mongo/MongoManager.cpp \
      src/main.cpp
      
OBJ = $(SRC:.cpp=.o)
TARGET = server

all: $(TARGET)

$(TARGET): $(OBJ)
	$(CXX) $(OBJ) -o $(TARGET) $(LDFLAGS)

%.o: %.cpp
	$(CXX) $(CXXFLAGS) $(INCLUDES) -c $< -o $@

clean:
	rm -f $(OBJ) $(TARGET)

.PHONY: all clean