CXX = g++
CXXFLAGS = -std=c++17 -I./cpp-httplib -Isrc -Wall -O2 -Isrc
LDFLAGS = -lpqxx -lpq -pthread -lhiredis -lredis++

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
	$(CXX) $(CXXFLAGS) -c $< -o $@

clean:
	rm -f $(OBJ) $(TARGET)

.PHONY: all clean