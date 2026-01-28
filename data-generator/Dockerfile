FROM golang:1.18-alpine AS builder

WORKDIR /app

# Установка зависимостей
RUN apk add --no-cache git

# Копируем файлы
COPY data_generator.go go.mod go.sum ./

# Инициализируем модуль и загружаем зависимости
RUN go mod init data-generator || true
RUN go mod tidy

# Компилируем
RUN go build -o data-generator .

# Финальный образ
FROM alpine:latest

WORKDIR /app

# Копируем бинарник
COPY --from=builder /app/data-generator .

# Устанавливаем зависимости
RUN apk --no-cache add ca-certificates

# Переменные окружения
ENV API_URL="http://server:8080"
ENV BATCH_SIZE=5
ENV DELAY_BETWEEN_RUNS=30
ENV MAX_CYCLES=0
ENV LOG_LEVEL="info"

CMD ["./data-generator"]
