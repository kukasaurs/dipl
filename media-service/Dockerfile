FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o media-service ./cmd/main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/media-service .
COPY --from=builder /app/.env .

EXPOSE 8007
CMD ["./media-service"]
