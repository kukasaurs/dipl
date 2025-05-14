FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY . .

# Загружаем зависимости
RUN go mod download

# Собираем бинарник
RUN CGO_ENABLED=0 GOOS=linux go build -o payment-service ./cmd/main.go

# Финальный минимальный образ
FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/payment-service .
COPY --from=builder /app/.env .

EXPOSE 8005
CMD ["./payment-service"]
