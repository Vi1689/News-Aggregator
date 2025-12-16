# Используем официальный образ Ubuntu
FROM ubuntu:20.04

# Отключаем интерактивные запросы при установке пакетов
ENV DEBIAN_FRONTEND=noninteractive

# Устанавливаем базовые зависимости
RUN apt-get update && apt-get install -y \
    cmake make git curl wget \
    build-essential pkg-config \
    libpqxx-dev libpq-dev \
    nlohmann-json3-dev \
    libhiredis-dev libssl-dev \
    libsasl2-dev libsasl2-modules \
    libboost-system-dev libboost-filesystem-dev \
    libboost-thread-dev libboost-iostreams-dev \
    python3 python3-pip \
    zlib1g-dev libzstd-dev libsnappy-dev \
 && rm -rf /var/lib/apt/lists/*

# Устанавливаем более новую версию CMake (для MongoDB)
RUN wget https://github.com/Kitware/CMake/releases/download/v3.25.0/cmake-3.25.0-linux-x86_64.sh && \
    chmod +x cmake-3.25.0-linux-x86_64.sh && \
    ./cmake-3.25.0-linux-x86_64.sh --skip-license --prefix=/usr/local && \
    rm cmake-3.25.0-linux-x86_64.sh

# Устанавливаем MongoDB C Driver
RUN git clone https://github.com/mongodb/mongo-c-driver.git /tmp/mongo-c-driver && \
    cd /tmp/mongo-c-driver && \
    git checkout 1.24.4 && \
    mkdir -p cmake-build && cd cmake-build && \
    cmake \
      -DENABLE_AUTOMATIC_INIT_AND_CLEANUP=OFF \
      -DCMAKE_BUILD_TYPE=Release \
      -DCMAKE_INSTALL_PREFIX=/usr/local \
      .. && \
    make -j$(nproc) && \
    make install && \
    ldconfig && \
    rm -rf /tmp/mongo-c-driver

# Устанавливаем MongoDB C++ Driver
RUN git clone https://github.com/mongodb/mongo-cxx-driver.git /tmp/mongo-cxx-driver && \
    cd /tmp/mongo-cxx-driver && \
    git checkout r3.10.1 && \
    mkdir -p build && cd build && \
    cmake \
      -DCMAKE_BUILD_TYPE=Release \
      -DCMAKE_INSTALL_PREFIX=/usr/local \
      -DCMAKE_CXX_STANDARD=17 \
      -DBSONCXX_POLY_USE_BOOST=1 \
      -DMONGOCXX_ENABLE_SLOW_TESTS=OFF \
      .. && \
    make -j$(nproc) && \
    make install && \
    ldconfig && \
    rm -rf /tmp/mongo-cxx-driver

# Устанавливаем Redis++
RUN git clone https://github.com/sewenew/redis-plus-plus.git /tmp/redis-plus-plus && \
    cd /tmp/redis-plus-plus && \
    mkdir build && cd build && \
    cmake \
      -DREDIS_PLUS_PLUS_BUILD_TEST=OFF \
      -DCMAKE_BUILD_TYPE=Release \
      .. && \
    make -j$(nproc) && \
    make install && \
    ldconfig && \
    rm -rf /tmp/redis-plus-plus

# Клонируем cpp-httplib
RUN git clone https://github.com/yhirose/cpp-httplib.git /tmp/cpp-httplib

# Создаем рабочую директорию
WORKDIR /app

# Копируем исходники
COPY ./server/Makefile .
COPY ./server/src ./src

# Собираем сервер
RUN make

# Открываем порт
EXPOSE 8080

# Запускаем сервер
CMD ["./server"]