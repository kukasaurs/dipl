FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o user-management-service ./cmd/main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/user-management-service .
COPY --from=builder /app/.env .

EXPOSE 8006
CMD ["./user-management-service"]

