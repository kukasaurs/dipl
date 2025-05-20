package services

import (
	"cleaning-app/support-service/internal/models"
	"cleaning-app/support-service/internal/repository"
	"cleaning-app/support-service/internal/utils"
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SupportService struct {
	repo     *repository.SupportRepository
	notifier *utils.NotificationClient
}

func NewSupportService(repo *repository.SupportRepository, notifier *utils.NotificationClient) *SupportService {
	return &SupportService{repo: repo, notifier: notifier}
}

// --- Tickets ---

func (s *SupportService) CreateTicket(ctx context.Context, ticket *models.Ticket) error {
	return s.repo.CreateTicket(ctx, ticket)
}

func (s *SupportService) GetTicketByID(ctx context.Context, id primitive.ObjectID) (*models.Ticket, error) {
	return s.repo.GetTicketByID(ctx, id)
}

func (s *SupportService) GetTicketsForClient(ctx context.Context, clientID string) ([]models.Ticket, error) {
	return s.repo.GetTicketsByClient(ctx, clientID)
}

func (s *SupportService) GetAllTickets(ctx context.Context) ([]models.Ticket, error) {
	return s.repo.GetAllTickets(ctx)
}

func (s *SupportService) UpdateTicketStatus(ctx context.Context, id primitive.ObjectID, status models.TicketStatus) error {
	return s.repo.UpdateTicketStatus(ctx, id, status)
}

// --- Chat messages ---

func (s *SupportService) AddMessage(ctx context.Context, msg *models.Message) error {
	err := s.repo.AddMessage(ctx, msg)
	if err != nil {
		return err
	}

	// Определить получателя (если пишет клиент → уведомить менеджера и наоборот)
	var targetUserID string
	if msg.SenderRole == "client" {
		targetUserID = "manager" // здесь можно сделать по-другому, если ID менеджера известен
	} else {
		ticket, _ := s.repo.GetTicketByID(ctx, msg.TicketID)
		targetUserID = ticket.ClientID
	}

	_ = s.notifier.SendMessageNotification(ctx, targetUserID, msg.Text)
	return nil
}

func (s *SupportService) GetMessagesByTicket(ctx context.Context, ticketID primitive.ObjectID) ([]models.Message, error) {
	return s.repo.GetMessagesByTicket(ctx, ticketID)
}
