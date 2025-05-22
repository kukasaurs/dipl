FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY . .

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o api-gateway ./cmd/main.go

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/api-gateway .
COPY --from=builder /app/.env .

EXPOSE 8080
CMD ["./api-gateway"]
