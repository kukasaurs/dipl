package services

import (
	"bytes"
	"cleaning-app/subscription-service/internal/config"
	"cleaning-app/subscription-service/internal/models"
	_ "cleaning-app/subscription-service/internal/models"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type NotificationService interface {
	// SendNotification sends a notification to a user
	SendNotification(ctx context.Context, userID, role, title, message, notificationType, deliveryType string, metadata map[string]string) error

	// SendSubscriptionNotification sends a subscription-specific notification
	SendSubscriptionNotification(ctx context.Context, sub models.Subscription, action string, details map[string]string) error
}

type notificationService struct {
	cfg *config.Config
}

// NewNotificationService creates a new notification service
func NewNotificationService(cfg *config.Config) NotificationService {
	return &notificationService{
		cfg: cfg,
	}
}

// NotificationRequest represents a request to send a notification
type NotificationRequest struct {
	UserID       string            `json:"user_id"`
	Role         string            `json:"role"`
	Title        string            `json:"title"`
	Message      string            `json:"message"`
	Type         string            `json:"type"`
	DeliveryType string            `json:"delivery_type"` // push | email | both
	Metadata     map[string]string `json:"metadata,omitempty"`
}

func (s *notificationService) SendNotification(ctx context.Context, userID, role, title, message, notificationType, deliveryType string, metadata map[string]string) error {
	if s.cfg == nil {
		return fmt.Errorf("config is nil")
	}

	notification := NotificationRequest{
		UserID:       userID,
		Role:         role,
		Title:        title,
		Message:      message,
		Type:         notificationType,
		DeliveryType: deliveryType,
		Metadata:     metadata,
	}

	jsonData, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.NotifiServiceURL+"/api/notifications/send", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create notification request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("notification service returned status %d", resp.StatusCode)
	}
	return nil
}

func (s *notificationService) SendSubscriptionNotification(ctx context.Context, sub models.Subscription, action string, details map[string]string) error {
	var title, message, notificationType string

	switch action {
	case "created":
		title = "Подписка создана"
		message = "Ваша подписка успешно создана."
		notificationType = "subscription_created"
	case "extended":
		title = "Подписка продлена"
		message = "Ваша подписка успешно продлена."
		notificationType = "subscription_extended"
	case "cancelled":
		title = "Подписка отменена"
		message = "Ваша подписка была отменена."
		notificationType = "subscription_cancelled"
	case "expiring_soon":
		title = "Срок подписки подходит к концу"
		message = "Через 3 дня закончится срок вашей подписки. Продлите, чтобы не потерять доступ."
		notificationType = "subscription_expiring"
	case "expired":
		title = "Подписка истекла"
		message = "Срок действия вашей подписки истек."
		notificationType = "subscription_expired"
	default:
		title = "Обновление подписки"
		message = "Ваша подписка была обновлена."
		notificationType = "subscription_updated"
	}

	return s.SendNotification(ctx, sub.ClientID, "user", title, message, notificationType, "push", details)
}
