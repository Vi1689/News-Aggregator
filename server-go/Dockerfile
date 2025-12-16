# Этап builder - используем Alpine Linux
FROM golang:alpine AS builder

WORKDIR /app

# Устанавливаем необходимые пакеты в Alpine
RUN apk add --no-cache git gcc musl-dev wget

# Копируем файлы модулей
COPY go.mod go.sum ./
RUN go mod download

# Копируем остальной код
COPY . .

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server

# Этап runtime
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/
COPY --from=builder /app/server .
EXPOSE 8080
CMD ["./server"]