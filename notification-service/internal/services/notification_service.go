package services

import (
	"cleaning-app/notification-service/internal/models"
	"cleaning-app/notification-service/internal/repository"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Каналы Redis для получения событий
const (
	OrderEventsChannel   = "order_events"
	SupportEventsChannel = "support_events"
	AdminEventsChannel   = "admin_events"
)

// NotificationService обрабатывает все операции, связанные с уведомлениями
type NotificationService struct {
	repo  *repository.NotificationRepository
	redis *redis.Client
}

// NewNotificationService создает новый экземпляр сервиса уведомлений
func NewNotificationService(repo *repository.NotificationRepository, rdb *redis.Client) *NotificationService {
	return &NotificationService{
		repo:  repo,
		redis: rdb,
	}
}

// EventPayload общая структура для событий из Redis
type EventPayload struct {
	UserID    string            `json:"user_id"`
	Role      string            `json:"role"`
	EventType string            `json:"event_type,omitempty"`
	Title     string            `json:"title,omitempty"`
	Message   string            `json:"message"`
	ExtraData map[string]string `json:"extra_data,omitempty"`
}

// SendNotification создает и отправляет уведомление
func (s *NotificationService) SendNotification(ctx context.Context, notification *models.Notification) error {
	// Сохраняем уведомление в БД
	if err := s.repo.Create(ctx, notification); err != nil {
		return fmt.Errorf("failed to save notification: %w", err)
	}

	log.Printf("Notification sent - Type: %s, User: %s, Title: %s",
		notification.Type, notification.UserID, notification.Title)

	return nil
}

// ProcessEvent обрабатывает событие из Redis и создает соответствующее уведомление
func (s *NotificationService) ProcessEvent(ctx context.Context, channel string, payload []byte) error {
	var event EventPayload
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	var notifType models.NotificationType
	var deliveryType models.DeliveryMethod
	var title string

	switch channel {
	case OrderEventsChannel:
		notifType = models.TypeOrderEvent
		title = formatOrderTitle(event.EventType, event.ExtraData)
		deliveryType = determineDeliveryType(event.Role, "order")
	case SupportEventsChannel:
		notifType = models.TypeSupportEvent
		title = "Сообщение от службы поддержки"
		deliveryType = determineDeliveryType(event.Role, "support")
	case AdminEventsChannel:
		notifType = models.TypeAdminAlert
		title = "Административное уведомление"
		deliveryType = models.DeliveryPush
	default:
		notifType = models.TypeSystemMessage
		title = event.Title
		if title == "" {
			title = "Системное уведомление"
		}
		deliveryType = models.DeliveryPush
	}

	notification := &models.Notification{
		UserID:       event.UserID,
		Title:        title,
		Message:      event.Message,
		Type:         notifType,
		DeliveryType: deliveryType,
		IsRead:       false,
		CreatedAt:    time.Now(),
		Metadata:     event.ExtraData,
	}

	return s.SendNotification(ctx, notification)
}

// Вспомогательные функции для форматирования уведомлений
func formatOrderTitle(eventType string, data map[string]string) string {
	switch eventType {
	case "created":
		return "Новый заказ создан"
	case "assigned":
		return "Заказ назначен"
	case "completed":
		return "Заказ выполнен"
	case "cancelled":
		return "Заказ отменен"
	case "reminder":
		return "Напоминание о заказе"
	default:
		return "Обновление заказа"
	}
}

// determineDeliveryType определяет способ доставки на основе роли пользователя и типа уведомления
func determineDeliveryType(role, eventSource string) models.DeliveryMethod {
	if role == "client" {
		if eventSource == "order" {
			return models.DeliveryEmail
		}
		return models.DeliveryPush
	}

	if role == "cleaner" || role == "manager" {
		return models.DeliverySMS
	}

	return models.DeliveryPush
}

// GetNotifications возвращает список уведомлений пользователя
func (s *NotificationService) GetNotifications(ctx context.Context, userID string, limit, offset int64) ([]models.Notification, error) {
	return s.repo.GetByUserID(ctx, userID, limit, offset)
}

// MarkAsRead отмечает уведомление как прочитанное
func (s *NotificationService) MarkAsRead(ctx context.Context, id primitive.ObjectID) error {
	return s.repo.MarkAsRead(ctx, id)
}

// StartRedisSubscribers запускает подписки на все каналы уведомлений Redis
func (s *NotificationService) StartRedisSubscribers(ctx context.Context) {
	channels := []string{OrderEventsChannel, SupportEventsChannel, AdminEventsChannel}

	pubsub := s.redis.Subscribe(ctx, channels...)
	defer pubsub.Close()

	log.Printf("Subscribed to Redis channels: %v", channels)

	ch := pubsub.Channel()
	for {
		select {
		case msg := <-ch:
			log.Printf("Received message from channel %s", msg.Channel)
			if err := s.ProcessEvent(ctx, msg.Channel, []byte(msg.Payload)); err != nil {
				log.Printf("Error processing event: %v", err)
			}
		case <-ctx.Done():
			log.Println("Stopping Redis subscribers...")
			return
		}
	}
}
