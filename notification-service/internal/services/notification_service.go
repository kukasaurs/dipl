package service

import (
	"cleaning-app/notification-service/internal/models"
	"cleaning-app/notification-service/internal/repository"
	"context"
)

type NotificationService interface {
	Send(ctx context.Context, notif *models.Notification) error
	GetAll(ctx context.Context, userID string) ([]models.Notification, error)
	MarkAsRead(ctx context.Context, id string) error
}

type notifService struct {
	repo repository.NotificationRepository
}

func NewNotificationService(repo repository.NotificationRepository) NotificationService {
	return &notifService{repo: repo}
}

func (s *notifService) Send(ctx context.Context, notif *models.Notification) error {
	return s.repo.Create(ctx, notif)
}

func (s *notifService) GetAll(ctx context.Context, userID string) ([]models.Notification, error) {
	return s.repo.List(ctx, userID)
}

func (s *notifService) MarkAsRead(ctx context.Context, id string) error {
	return s.repo.MarkAsRead(ctx, id)
}
