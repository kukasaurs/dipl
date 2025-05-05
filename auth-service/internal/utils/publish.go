package utils

import (
	"context"
	"encoding/json"
	"github.com/redis/go-redis/v9"
	"log"
)

type NotificationPayload struct {
	UserID  string `json:"user_id"`
	Role    string `json:"role"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

func PublishNotification(ctx context.Context, rdb *redis.Client, payload NotificationPayload) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("❌ Failed to marshal notification: %v\n", err)
		return
	}
	if err := rdb.Publish(ctx, "notifications", data).Err(); err != nil {
		log.Printf("❌ Failed to publish notification: %v\n", err)
	}
}
