package utils

import (
	"context"
	"encoding/json"
	"os"

	"github.com/redis/go-redis/v9"
)

// NotificationEvent — структура для публикации уведомления через Redis
type NotificationEvent struct {
	UserID      string            `json:"user_id"`
	Role        string            `json:"role"`
	Type        string            `json:"type"`                   // Например: "welcome", "security", "order_confirmed"
	Title       string            `json:"title,omitempty"`        // Можно оставить пустым — будет дефолтный
	Message     string            `json:"message,omitempty"`      // Можно оставить пустым — будет дефолтный
	ExtraData   map[string]string `json:"extra_data,omitempty"`   // Любые доп. поля (например, email)
	DeviceToken string            `json:"device_token,omitempty"` // Для пуша
}

const NotificationEventsChannel = "notification_events"

// Получение Redis-клиента (инициализируй один раз на старте приложения!)
var redisClient *redis.Client

func InitRedisClient() {
	url := os.Getenv("REDIS_URL")
	opt, err := redis.ParseURL(url)
	if err != nil {
		panic(err)
	}
	redisClient = redis.NewClient(opt)
}

// SendNotificationEvent — публикация события в notification_events
func SendNotificationEvent(ctx context.Context, event NotificationEvent) error {
	if redisClient == nil {
		InitRedisClient() // на случай, если забыли инициализировать явно
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return redisClient.Publish(ctx, NotificationEventsChannel, payload).Err()
}
