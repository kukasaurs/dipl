FROM golang:1.24-alpine AS builder

# Установка зависимостей для сборки
RUN apk add --no-cache git build-base

WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o notification-service ./cmd/main.go

FROM alpine:latest
WORKDIR /app

# Установка зависимостей для отправки почты
RUN apk add --no-cache ca-certificates

COPY --from=builder /app/notification-service .
COPY --from=builder /app/.env .

EXPOSE 8002
CMD ["./notification-service"]