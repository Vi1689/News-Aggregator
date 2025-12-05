FROM gcc:14

RUN apt-get update && apt-get install -y \
    cmake make git curl wget \
    libpqxx-dev libpq-dev \
    nlohmann-json3-dev \
    libhiredis-dev libssl-dev \
    libsasl2-dev pkg-config build-essential \
    libboost-system-dev libboost-filesystem-dev libboost-thread-dev libboost-iostreams-dev \
 && rm -rf /var/lib/apt/lists/*

# MongoDB C Driver
RUN git clone https://github.com/mongodb/mongo-c-driver.git /tmp/mongo-c-driver \
 && cd /tmp/mongo-c-driver \
 && git checkout 1.24.4 \
 && mkdir -p cmake-build && cd cmake-build \
 && cmake -DENABLE_AUTOMATIC_INIT_AND_CLEANUP=OFF -DCMAKE_BUILD_TYPE=Release .. \
 && make -j$(nproc) && make install && ldconfig \
 && rm -rf /tmp/mongo-c-driver

# MongoDB C++ Driver
RUN git clone https://github.com/mongodb/mongo-cxx-driver.git /tmp/mongo-cxx-driver \
 && cd /tmp/mongo-cxx-driver \
 && git checkout r3.8.0 \
 && mkdir -p build && cd build \
 && cmake -DCMAKE_BUILD_TYPE=Release -DBSONCXX_POLY_USE_BOOST=1 .. \
 && make -j$(nproc) && make install && ldconfig \
 && rm -rf /tmp/mongo-cxx-driver

# 🔧 Исправляем симлинки
RUN set -eux; \
    if [ -L /usr/local/include/mongocxx ]; then rm /usr/local/include/mongocxx; fi; \
    if [ -L /usr/local/include/bsoncxx ]; then rm /usr/local/include/bsoncxx; fi; \
    if [ -d /usr/local/include/mongocxx/v_noabi ]; then ln -s /usr/local/include/mongocxx/v_noabi /usr/local/include/mongocxx; fi; \
    if [ -d /usr/local/include/bsoncxx/v_noabi ]; then ln -s /usr/local/include/bsoncxx/v_noabi /usr/local/include/bsoncxx; fi;

# Redis++
RUN git clone https://github.com/sewenew/redis-plus-plus.git /tmp/redis-plus-plus \
 && cd /tmp/redis-plus-plus \
 && mkdir build && cd build \
 && cmake -DREDIS_PLUS_PLUS_BUILD_TEST=OFF .. \
 && make -j$(nproc) && make install && ldconfig \
 && rm -rf /tmp/redis-plus-plus

WORKDIR /app

RUN git clone https://github.com/yhirose/cpp-httplib.git

COPY ./server/Makefile .
COPY ./server/src ./src

RUN make

EXPOSE 8080
CMD ["./server"]
