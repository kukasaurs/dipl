package services

import (
	"cleaning-app/notification-service/internal/models"
	"cleaning-app/notification-service/internal/repository"
	"context"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type NotificationService struct {
	repo *repository.NotificationRepository
}

func NewNotificationService(repo *repository.NotificationRepository) *NotificationService {
	return &NotificationService{repo: repo}
}

func (s *NotificationService) SendNotification(ctx context.Context, n *models.Notification) error {
	// Здесь можно добавить логику отправки через push, email или SMS
	// В зависимости от n.Type
	return s.repo.Create(ctx, n)
}

func (s *NotificationService) GetNotifications(ctx context.Context, userID string, limit, offset int64) ([]models.Notification, error) {
	return s.repo.GetByUserID(ctx, userID, limit, offset)
}

func (s *NotificationService) MarkAsRead(ctx context.Context, id primitive.ObjectID) error {
	return s.repo.MarkAsRead(ctx, id)
}
