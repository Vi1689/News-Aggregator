FROM golang:1.25-alpine AS builder

WORKDIR /app

# Копируем go.mod и go.sum
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходники
COPY . .

# Собираем бинарник
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o kafka-streams .

# Финальный образ
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/kafka-streams .

EXPOSE 8080

CMD ["./kafka-streams"]