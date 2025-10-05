FROM gcc:13

WORKDIR /app

RUN apt-get update && apt-get install -y \
    cmake \
    build-essential \
    libpq-dev \
    libpqxx-dev \
    wget \
    && rm -rf /var/lib/apt/lists/*

# header-only libs
RUN mkdir -p include
RUN wget -q -O include/httplib.h https://raw.githubusercontent.com/yhirose/cpp-httplib/master/httplib.h
RUN wget -q -O include/json.hpp https://raw.githubusercontent.com/nlohmann/json/develop/single_include/nlohmann/json.hpp
RUN apt-get update && apt-get install -y libhiredis-dev
RUN git clone https://github.com/sewenew/redis-plus-plus.git && cd redis-plus-plus && mkdir build && cd build && cmake .. && make && make install


COPY . .

RUN mkdir -p build && cd build && cmake .. && make -j$(nproc)

EXPOSE 8080

CMD ["./build/server"]
