package utils

import (
	"cleaning-app/notification-service/internal/models"
	"cleaning-app/notification-service/internal/services"
	"context"
	"encoding/json"
	redis2 "github.com/redis/go-redis/v9"
	"log"
)

const RedisChannel = "notifications"

type NotificationPayload struct {
	UserID  string `json:"user_id"`
	Role    string `json:"role"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

func SubscribeToRedis(ctx context.Context, rdb *redis2.Client, notifService services.NotificationService) {
	pubsub := rdb.Subscribe(ctx, RedisChannel)
	ch := pubsub.Channel()

	log.Println("✅ Subscribed to Redis channel:", RedisChannel)

	for msg := range ch {
		var payload NotificationPayload
		if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil {
			log.Printf("❌ Invalid notification payload: %v\n", err)
			continue
		}

		notif := &models.Notification{
			UserID:  payload.UserID,
			Title:   payload.Title,
			Message: payload.Message,
		}

		if err := notifService.SendNotification(ctx, notif); err != nil {
			log.Printf("❌ Failed to save notification: %v\n", err)
		} else {
			log.Printf("📨 Notification saved: %+v\n", notif)
		}
	}
}
