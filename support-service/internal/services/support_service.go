package services

import (
	"context"

	"cleaning-app/support-service/internal/models"
	"cleaning-app/support-service/internal/repository"
	"cleaning-app/support-service/internal/utils"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SupportService отвечает за бизнес-логику support-сервиса.
type SupportService struct {
	repo     repository.SupportRepository
	notifier *utils.NotificationClient
}

// NewSupportService конструирует SupportService.
func NewSupportService(repo repository.SupportRepository, notifier *utils.NotificationClient) *SupportService {
	return &SupportService{repo: repo, notifier: notifier}
}

// CreateTicket создаёт новый тикет.
func (s *SupportService) CreateTicket(ctx context.Context, ticket *models.Ticket) error {
	return s.repo.CreateTicket(ctx, ticket)
}

// GetTicketByID возвращает тикет по его ObjectID.
func (s *SupportService) GetTicketByID(ctx context.Context, id primitive.ObjectID) (*models.Ticket, error) {
	return s.repo.GetTicketByID(ctx, id)
}

// GetTicketsForClient возвращает все тикеты данного клиента (без фильтра по статусу).
func (s *SupportService) GetTicketsForClient(ctx context.Context, clientID string) ([]models.Ticket, error) {
	return s.repo.GetTicketsByClient(ctx, clientID)
}

// GetAllTickets возвращает абсолютно все тикеты.
func (s *SupportService) GetAllTickets(ctx context.Context) ([]models.Ticket, error) {
	return s.repo.GetAllTickets(ctx)
}

// UpdateTicketStatus обновляет статус заданного тикета.
func (s *SupportService) UpdateTicketStatus(ctx context.Context, id primitive.ObjectID, status models.TicketStatus) error {
	return s.repo.UpdateTicketStatus(ctx, id, status)
}

// AddMessage сохраняет новое сообщение в тикете и рассылает нотификацию.
func (s *SupportService) AddMessage(ctx context.Context, msg *models.Message) error {
	if err := s.repo.AddMessage(ctx, msg); err != nil {
		return err
	}

	// Определяем, кому уведомление:
	var targetUserID string
	if msg.SenderRole == "client" {
		// клиент пишет → уведомляем менеджера
		targetUserID = "manager"
	} else {
		// менеджер/админ пишет → уведомляем клиента
		ticket, _ := s.repo.GetTicketByID(ctx, msg.TicketID)
		targetUserID = ticket.ClientID
	}

	_ = s.notifier.SendMessageNotification(ctx, targetUserID, msg.Text)
	return nil
}

// GetMessagesByTicket возвращает все сообщения тикета в порядке времени.
func (s *SupportService) GetMessagesByTicket(ctx context.Context, ticketID primitive.ObjectID) ([]models.Message, error) {
	return s.repo.GetMessagesByTicket(ctx, ticketID)
}

// GetTicketsForUserByStatus возвращает тикеты данного клиента с конкретным статусом.
func (s *SupportService) GetTicketsForUserByStatus(ctx context.Context, userID string, status models.TicketStatus) ([]models.Ticket, error) {
	return s.repo.GetTicketsByClientAndStatus(ctx, userID, status)
}

// GetAllTicketsByStatus возвращает все тикеты с указанным статусом.
func (s *SupportService) GetAllTicketsByStatus(ctx context.Context, status models.TicketStatus) ([]models.Ticket, error) {
	return s.repo.GetTicketsByStatus(ctx, status)
}
