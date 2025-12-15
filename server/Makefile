CXX = g++

# Пути к заголовочным файлам
INCLUDES = -I/tmp/cpp-httplib \
           -Isrc \
           -I/usr/local/include \
           -I/usr/local/include/mongocxx/v_noabi \
           -I/usr/local/include/bsoncxx/v_noabi \
           -I/usr/local/include/libmongoc-1.0 \
           -I/usr/local/include/libbson-1.0

# Флаги компиляции
CXXFLAGS = -std=c++17 -Wall -O2

# Пути к библиотекам
LDFLAGS = -L/usr/local/lib \
          -lpqxx -lpq \
          -pthread \
          -lredis++ -lhiredis \
          -lmongocxx -lbsoncxx \
          -lmongoc-1.0 -lbson-1.0 \
          -lssl -lcrypto \
          -lsasl2 \
          -lresolv

# Исходные файлы
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
	@echo "Linking $(TARGET)..."
	$(CXX) $(OBJ) -o $(TARGET) $(LDFLAGS)
	@echo "Build successful!"

%.o: %.cpp
	@echo "Compiling $<..."
	$(CXX) $(CXXFLAGS) $(INCLUDES) -c $< -o $@

clean:
	rm -f $(OBJ) $(TARGET)
	@echo "Clean complete!"

test:
	@echo "Testing MongoDB connection..."
	@pkg-config --cflags --libs libmongocxx || echo "mongocxx not found in pkg-config"
	@echo "Include paths:"
	@echo "$(INCLUDES)"

.PHONY: all clean test