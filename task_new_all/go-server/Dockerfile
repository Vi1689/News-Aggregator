FROM golang:1.21-alpine AS builder

WORKDIR /app

# Устанавливаем зависимости
RUN apk add --no-cache git

# Копируем go.mod и go.sum
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходники
COPY . .

# Собираем бинарник
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o news-server ./cmd/server

# Финальный образ
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Копируем бинарник
COPY --from=builder /app/news-server .

# Копируем директорию с данными
COPY --from=builder /app/data ./data

# Создаём директорию для логов
RUN mkdir -p /root/logs

EXPOSE 8080

CMD ["./news-server"]