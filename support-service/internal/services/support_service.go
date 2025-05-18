package services

import (
	"context"

	"cleaning-app/support-service/internal/models"
	"cleaning-app/support-service/internal/repository"
)

type ChatService struct {
	Repo     *repository.ChatRepository
	Notifier *NotifierService
}

func NewChatService(repo *repository.ChatRepository, notifier *NotifierService) *ChatService {
	return &ChatService{Repo: repo, Notifier: notifier}
}

func (s *ChatService) SendMessage(ctx context.Context, msg *models.Message) error {
	if err := s.Repo.SaveMessage(ctx, msg); err != nil {
		return err
	}
	return s.Notifier.SendNotification(msg.ReceiverID, msg.Text)
}

func (s *ChatService) GetMessages(ctx context.Context, u1, u2 string) ([]*models.Message, error) {
	return s.Repo.GetMessages(ctx, u1, u2)
}
