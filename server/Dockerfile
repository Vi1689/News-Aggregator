FROM gcc:14

RUN apt-get update && apt-get install -y \
    libpqxx-dev \
    libpq-dev \
    nlohmann-json3-dev \
    curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY ./cpp-httplib ./cpp-httplib
COPY ./server/Makefile .
COPY ./server/src ./src

RUN make

EXPOSE 8080

CMD ["./server"]
