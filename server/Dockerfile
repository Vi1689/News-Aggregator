FROM gcc:14

# Устанавливаем зависимости
RUN apt-get update && apt-get install -y \
    cmake \
    make \
    git \
    libpqxx-dev \
    libpq-dev \
    nlohmann-json3-dev \
    curl \
    libhiredis-dev \
    libssl-dev \
    libmongocxx-dev \
    libbsoncxx-dev \
    && rm -rf /var/lib/apt/lists/*

# Сборка и установка Redis++
RUN git clone https://github.com/sewenew/redis-plus-plus.git /tmp/redis-plus-plus && \
    cd /tmp/redis-plus-plus && mkdir build && cd build && \
    cmake -DREDIS_PLUS_PLUS_BUILD_TEST=OFF .. && \
    make -j$(nproc) && make install && \
    ldconfig && \              
    rm -rf /tmp/redis-plus-plus

# Рабочий каталог
WORKDIR /app

# Копируем исходники
RUN git clone https://github.com/yhirose/cpp-httplib.git
COPY ./server/Makefile .
COPY ./server/src ./src

# Сборка проекта
RUN make

EXPOSE 8080

CMD ["./server"]