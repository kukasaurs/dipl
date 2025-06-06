package services

import (
	"context"
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"time"

	"cleaning-app/notification-service/internal/models"
	"cleaning-app/notification-service/internal/utils/push"

	"github.com/redis/go-redis/v9"
)

const (
	// Один универсальный канал для всех событий
	NotificationEventsChannel = "notification_events"
)

// NotificationEvent — структура события, публикуемого в Redis
type NotificationEvent struct {
	UserID      string            `json:"user_id"`
	Role        string            `json:"role"`
	Type        string            `json:"type"`                   // ключ из eventMeta
	Title       string            `json:"title,omitempty"`        // можно переопределить дефолтный
	Message     string            `json:"message,omitempty"`      // можно переопределить дефолтный
	ExtraData   map[string]string `json:"extra_data,omitempty"`   // любые дополнительные поля
	DeviceToken string            `json:"device_token,omitempty"` // для push-уведомлений
}

// Metadata о каждом типе события: дефолтный заголовок, текст, тип уведомления и DeliveryMethod
var eventMeta = map[string]struct {
	Title      string
	DefaultMsg string
	NotifType  models.NotificationType
	Delivery   models.DeliveryMethod
}{
	// 1. User registration
	"welcome": {
		Title:      "Welcome!",
		DefaultMsg: "Thank you for registering. Enjoy using our service!",
		NotifType:  models.TypeSystemMessage,
		Delivery:   models.DeliveryPush,
	},
	// 2. Password change
	"security": {
		Title:      "Password changed",
		DefaultMsg: "Your password has been successfully updated.",
		NotifType:  models.TypeSystemMessage,
		Delivery:   models.DeliveryPush,
	},
	// 3. Order confirmation
	"order_confirmed": {
		Title:      "Order confirmed",
		DefaultMsg: "Your order has been successfully confirmed and will be fulfilled at the appointed time.",
		NotifType:  models.TypeOrderEvent,
		Delivery:   models.DeliveryPush,
	},
	// 4. Reminder 24 hours before cleaning
	"reminder": {
		Title:      "Cleaning tomorrow",
		DefaultMsg: "Reminder: your cleaning will take place tomorrow at {{time}}.",
		NotifType:  models.TypeOrderEvent,
		Delivery:   models.DeliveryPush,
	},
	// 5. Arrival of the cleaner
	"cleaning_started": {
		Title:      "The cleaner has started cleaning",
		DefaultMsg: "Your cleaner has started the order.",
		NotifType:  models.TypeOrderEvent,
		Delivery:   models.DeliveryPush,
	},
	// 6. Cleaning completed
	"cleaning_completed": {
		Title:      "Cleaning completed",
		DefaultMsg: "Cleaning has been successfully completed. Please rate the quality!",
		NotifType:  models.TypeOrderEvent,
		Delivery:   models.DeliveryPush,
	},
	// 7. Request for feedback
	"review_request": {
		Title:      "How did you like the cleaning?",
		DefaultMsg: "Rate the cleaner's work. Your opinion is important to us!",
		NotifType:  models.TypeSystemMessage,
		Delivery:   models.DeliveryPush,
	},
	// 8. Order cancellation
	"order_cancelled": {
		Title:      "Order canceled",
		DefaultMsg: "Your order has been canceled. We apologize for any inconvenience.",
		NotifType:  models.TypeOrderEvent,
		Delivery:   models.DeliveryPush,
	},
	// 9. Successful payment
	"payment_successful": {
		Title:      "Payment completed",
		DefaultMsg: "Your payment has been successfully processed.",
		NotifType:  models.TypeSystemMessage,
		Delivery:   models.DeliveryPush,
	},
	// 10. Failed payment
	"payment_failed": {
		Title:      "Payment error",
		DefaultMsg: "An error occurred while attempting to make a payment. Please try again.",
		NotifType:  models.TypeSystemMessage,
		Delivery:   models.DeliveryPush,
	},
	// 11. Subscription update
	"subscription_updated": {
		Title:      "Subscription updated",
		DefaultMsg: "Your subscription has been updated.",
		NotifType:  models.TypeSystemMessage,
		Delivery:   models.DeliveryPush,
	},
	// 12. Subscription expiring
	"subscription_expiring": {
		Title:      "Subscription expiring",
		DefaultMsg: "Your subscription will expire in 3 days. Renew it to keep your access.",
		NotifType:  models.TypeSystemMessage,
		Delivery:   models.DeliveryPush,
	},
	// 13. New message from support
	"support_message": {
		Title:      "New message",
		DefaultMsg: "You have received a new message from support.",
		NotifType:  models.TypeSupportEvent,
		Delivery:   models.DeliveryPush,
	},
	// 14. Assigning a cleaner to an order
	"assigned_order": {
		Title:      "New order",
		DefaultMsg: "You have been assigned a new order. Check the details in your profile.",
		NotifType:  models.TypeOrderEvent,
		Delivery:   models.DeliveryPush,
	},
	// 15. Order change
	"order_updated": {
		Title:      "Order update",
		DefaultMsg: "Your order details have been changed. Please check the updated information.",
		NotifType:  models.TypeOrderEvent,
		Delivery:   models.DeliveryPush,
	},
	// 16. Order deleted
	"order_deleted": {
		Title:      "Order deleted",
		DefaultMsg: "One of your orders has been deleted.",
		NotifType:  models.TypeOrderEvent,
		Delivery:   models.DeliveryPush,
	},
	// 17. Warning from the administrator
	"admin_alert": {
		Title:      "Important notification",
		DefaultMsg: "", // текст приходит в payload.Message
		NotifType:  models.TypeAdminAlert,
		Delivery:   models.DeliveryPush,
	},
	// 18. Report uploaded
	"report_uploaded": {
		Title:      "Photo report uploaded",
		DefaultMsg: "The cleaner has finished cleaning and uploaded the photo report.",
		NotifType:  models.TypeSystemMessage,
		Delivery:   models.DeliveryPush,
	},
	// default на случай неизвестного типа
	"default": {
		Title:      "System notification",
		DefaultMsg: "You have a new notification.",
		NotifType:  models.TypeSystemMessage,
		Delivery:   models.DeliveryPush,
	},
}

// NotificationService отвечает за приём из Redis, сохранение и push
type NotificationService struct {
	repo  NotificationRepository
	redis *redis.Client
	FCM   *push.FCMClient
}

type NotificationRepository interface {
	Create(ctx context.Context, notification *models.Notification) error
	GetByUserID(ctx context.Context, userID string, limit, offset int64) ([]models.Notification, error)
	MarkAsRead(ctx context.Context, id primitive.ObjectID) error
}

func NewNotificationService(repo NotificationRepository, rdb *redis.Client, fcm *push.FCMClient) *NotificationService {
	return &NotificationService{
		repo:  repo,
		redis: rdb,
		FCM:   fcm,
	}
}

// SendPush отправляет push-уведомление через FCM
func (s *NotificationService) SendPush(req models.PushNotificationRequest) error {
	return s.FCM.SendPushNotification(req.Token, req.Title, req.Message)
}

// SendNotification сохраняет уведомление в MongoDB
func (s *NotificationService) SendNotification(ctx context.Context, notification *models.Notification) error {
	if err := s.repo.Create(ctx, notification); err != nil {
		return fmt.Errorf("failed to save notification: %w", err)
	}
	log.Printf("Notification saved - Type: %s, User: %s, Title: %s",
		notification.Type, notification.UserID, notification.Title)
	return nil
}

// ProcessEvent обрабатывает один payload из Redis
func (s *NotificationService) ProcessEvent(ctx context.Context, payload []byte) error {
	var event NotificationEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	meta, ok := eventMeta[event.Type]
	if !ok {
		meta = eventMeta["default"]
	}

	// Если в payload пришёл собственный Title/Message, используем его, иначе дефолт
	title := event.Title
	if title == "" {
		title = meta.Title
	}
	msg := event.Message
	if msg == "" {
		msg = meta.DefaultMsg
	}

	notification := &models.Notification{
		UserID:       event.UserID,
		Title:        title,
		Message:      msg,
		Type:         meta.NotifType,
		DeliveryType: meta.Delivery,
		IsRead:       false,
		CreatedAt:    time.Now(),
		Metadata:     event.ExtraData,
	}

	// Сохраняем уведомление в БД
	if err := s.SendNotification(ctx, notification); err != nil {
		return err
	}

	// Если пришёл device token, отправляем push
	if s.FCM != nil && event.DeviceToken != "" {
		if err := s.FCM.SendPushNotification(event.DeviceToken, title, msg); err != nil {
			log.Printf("Failed to send FCM push: %v", err)
		}
	}

	return nil
}

// StartRedisSubscriber подписывается на единый канал и обрабатывает входящие события
func (s *NotificationService) StartRedisSubscriber(ctx context.Context) {
	pubsub := s.redis.Subscribe(ctx, NotificationEventsChannel)
	defer pubsub.Close()

	log.Printf("Subscribed to Redis channel: %s", NotificationEventsChannel)
	ch := pubsub.Channel()

	for {
		select {
		case msg := <-ch:
			log.Printf("Received notification event")
			if err := s.ProcessEvent(ctx, []byte(msg.Payload)); err != nil {
				log.Printf("Error processing event: %v", err)
			}
		case <-ctx.Done():
			log.Println("Stopping Redis subscriber...")
			return
		}
	}
}
func (s *NotificationService) GetNotifications(ctx context.Context, userID string, limit, offset int64) ([]models.Notification, error) {
	return s.repo.GetByUserID(ctx, userID, limit, offset)
}

func (s *NotificationService) MarkAsRead(ctx context.Context, id primitive.ObjectID) error {
	return s.repo.MarkAsRead(ctx, id)
}
