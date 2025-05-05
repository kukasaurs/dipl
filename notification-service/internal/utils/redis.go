package utils

import (
	"cleaning-app/notification-service/internal/models"
	"cleaning-app/notification-service/internal/services"
	"context"
	"encoding/json"
	"github.com/redis/go-redis/v9"
	"log"
)

const RedisChannel = "notifications"

type NotificationPayload struct {
	UserID  string `json:"user_id"`
	Role    string `json:"role"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

func SubscribeToRedis(ctx context.Context, rdb *redis.Client, notifService service.NotificationService) {
	pubsub := rdb.Subscribe(ctx, RedisChannel)
	ch := pubsub.Channel()

	log.Println("Subscribed to Redis channel:", RedisChannel)

	for msg := range ch {
		var payload NotificationPayload
		err := json.Unmarshal([]byte(msg.Payload), &payload)
		if err != nil {
			log.Printf("Invalid notification payload: %v\n", err)
			continue
		}

		notif := &models.Notification{
			UserID:  payload.UserID,
			Role:    payload.Role,
			Title:   payload.Title,
			Message: payload.Message,
		}

		if err := notifService.Send(ctx, notif); err != nil {
			log.Printf("Failed to save notification: %v\n", err)
		} else {
			log.Printf("Notification saved: %+v\n", notif)
		}
	}
}
