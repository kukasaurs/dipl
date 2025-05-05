package services

import (
	"cleaning-app/order-service/internal/models"
	"context"
	"log"
)

type stubNotifier struct{}

func NewNotificationService() NotificationService {
	return &stubNotifier{}
}

func (s *stubNotifier) SendOrderNotification(ctx context.Context, order models.Order, event string) error {
	log.Printf("Notification event=%s, orderID=%s, status=%s", event, order.ID.Hex(), order.Status)
	return nil
}
