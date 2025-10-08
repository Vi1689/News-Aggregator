FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

WORKDIR /app

RUN apt-get update && apt-get install -y \
    build-essential \
    cmake \
    pkg-config \
    libpq-dev \
    libpqxx-dev \
    wget \
    git \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# header-only libs
RUN mkdir -p include
RUN wget -q -O include/httplib.h https://raw.githubusercontent.com/yhirose/cpp-httplib/master/httplib.h
RUN apt-get update && apt-get install -y \
    nlohmann-json3-dev
RUN wget -q -O include/json.hpp https://raw.githubusercontent.com/nlohmann/json/develop/single_include/nlohmann/json.hpp

# hiredis
RUN git clone --depth 1 https://github.com/redis/hiredis.git /tmp/hiredis \
    && cd /tmp/hiredis \
    && mkdir build && cd build && cmake .. && make -j$(nproc) && make install \
    && cd / && rm -rf /tmp/hiredis

# redis-plus-plus
RUN git clone --depth 1 https://github.com/sewenew/redis-plus-plus.git /tmp/redis-plus-plus \
    && cd /tmp/redis-plus-plus \
    && mkdir build && cd build \
    && cmake -DREDIS_PLUS_PLUS_BUILD_TEST=OFF -DREDIS_PLUS_PLUS_BUILD_SHARED=ON .. \
    && make -j$(nproc) && make install \
    && cd / && rm -rf /tmp/redis-plus-plus

# копируем код
COPY . .

RUN mkdir -p build && cd build && cmake .. && make -j$(nproc)

EXPOSE 8080

CMD ["./build/server"]
