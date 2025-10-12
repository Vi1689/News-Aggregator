CXX = g++
CXXFLAGS = -std=c++17 -I./cpp-httplib -Wall -O2
LDFLAGS = -lpqxx -lpq -pthread -lhiredis -lredis++

SRC = $(wildcard src/*.cpp)
OBJ = $(SRC:.cpp=.o)
TARGET = server

all: $(TARGET)

$(TARGET): $(OBJ)
	$(CXX) $(OBJ) -o $(TARGET) $(LDFLAGS)

%.o: %.cpp
	$(CXX) $(CXXFLAGS) -c $< -o $@

clean:
	rm -f $(OBJ) $(TARGET)
