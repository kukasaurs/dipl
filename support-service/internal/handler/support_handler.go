package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"cleaning-app/support-service/internal/models"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// DTO для отдачи клиенту
type MessageResponse struct {
	ID          string    `json:"id"`
	TicketID    string    `json:"ticket_id"`
	SenderID    string    `json:"sender_id"`
	SenderRole  string    `json:"sender_role"`
	SenderEmail string    `json:"sender_email"`
	Text        string    `json:"text"`
	Timestamp   time.Time `json:"timestamp"`
}

type SupportHandler struct {
	service        SupportService
	userServiceURL string
}

type SupportService interface {
	CreateTicket(ctx context.Context, ticket *models.Ticket) error
	GetTicketByID(ctx context.Context, id primitive.ObjectID) (*models.Ticket, error)
	GetTicketsForClient(ctx context.Context, clientID string) ([]models.Ticket, error)
	GetAllTickets(ctx context.Context) ([]models.Ticket, error)
	UpdateTicketStatus(ctx context.Context, id primitive.ObjectID, status models.TicketStatus) error
	AddMessage(ctx context.Context, msg *models.Message) error
	GetMessagesByTicket(ctx context.Context, ticketID primitive.ObjectID) ([]models.Message, error)
	GetTicketsForUserByStatus(ctx context.Context, userID string, status models.TicketStatus) ([]models.Ticket, error)
	GetAllTicketsByStatus(ctx context.Context, status models.TicketStatus) ([]models.Ticket, error)
}

func NewSupportHandler(srv SupportService, userServiceURL string) *SupportHandler {
	return &SupportHandler{
		service:        srv,
		userServiceURL: userServiceURL,
	}
}

// POST /support/tickets
func (h *SupportHandler) CreateTicket(c *gin.Context) {
	var req struct {
		Subject string `json:"subject"`
		Text    string `json:"text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	userID := c.GetString("userId")
	now := time.Now()
	t := &models.Ticket{
		ClientID:  userID,
		Subject:   req.Subject,
		Status:    models.StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := h.service.CreateTicket(c.Request.Context(), t); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, t)
}

// GET /support/tickets/my
func (h *SupportHandler) GetMyTickets(c *gin.Context) {
	userID := c.GetString("userId")
	tickets, err := h.service.GetTicketsForClient(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tickets)
}

// GET /support/tickets
// для админа — возвращает все тикеты
func (h *SupportHandler) GetAllTickets(c *gin.Context) {
	tickets, err := h.service.GetAllTickets(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tickets)
}

// PUT /support/tickets/:id/status
func (h *SupportHandler) UpdateTicketStatus(c *gin.Context) {
	hexID := c.Param("id")
	objID, err := primitive.ObjectIDFromHex(hexID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket ID"})
		return
	}
	var req struct {
		Status models.TicketStatus `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if err := h.service.UpdateTicketStatus(c.Request.Context(), objID, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// POST /support/tickets/:id/messages
func (h *SupportHandler) SendMessage(c *gin.Context) {

	hexID := c.Param("id")
	ticketID, err := primitive.ObjectIDFromHex(hexID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket ID"})
		return
	}
	var req struct {
		Text string `json:"text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	msg := &models.Message{
		TicketID:   ticketID,
		SenderID:   c.GetString("userId"),
		SenderRole: c.GetString("role"),
		Text:       req.Text,
		Timestamp:  time.Now(),
	}
	if err := h.service.AddMessage(c.Request.Context(), msg); err != nil {
		if errors.Is(err, errors.New("ticket is closed")) {
			c.JSON(http.StatusConflict, gin.H{"error": "ticket closed"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, msg)
}

// GET /support/tickets/:id/messages
func (h *SupportHandler) GetMessages(c *gin.Context) {
	hexID := c.Param("id")
	ticketID, err := primitive.ObjectIDFromHex(hexID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket ID"})
		return
	}
	rawMsgs, err := h.service.GetMessagesByTicket(c.Request.Context(), ticketID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]MessageResponse, 0, len(rawMsgs))
	for _, m := range rawMsgs {
		email := ""
		userURL := fmt.Sprintf("%s/users/%s", h.userServiceURL, m.SenderID)
		req, err := http.NewRequestWithContext(c.Request.Context(), "GET", userURL, nil)
		if err == nil {
			// Форвардим исходный токен
			if auth := c.GetHeader("Authorization"); auth != "" {
				req.Header.Set("Authorization", auth)
			}
			if res, err2 := http.DefaultClient.Do(req); err2 == nil && res.StatusCode == http.StatusOK {
				var u struct {
					Email string `json:"email"`
				}
				_ = json.NewDecoder(res.Body).Decode(&u)
				email = u.Email
				res.Body.Close()
			}
		}
		out = append(out, MessageResponse{
			ID:          m.ID.Hex(),
			TicketID:    m.TicketID.Hex(),
			SenderID:    m.SenderID,
			SenderRole:  m.SenderRole,
			SenderEmail: email,
			Text:        m.Text,
			Timestamp:   m.Timestamp,
		})
	}
	c.JSON(http.StatusOK, out)
}

// support-service/internal/handler/support_handler.go

func (h *SupportHandler) GetTickets(c *gin.Context) {
	userID := c.GetString("userId")
	role := c.GetString("role")

	status := c.Query("status") // "", "open", "in_progress", "closed"

	var tickets []models.Ticket
	var err error

	switch role {
	case "manager":
		if status == "" {
			status = string(models.StatusOpen)
		}
		tickets, err = h.service.GetAllTicketsByStatus(
			c.Request.Context(), models.TicketStatus(status))
	case "admin":
		// админ видит все тикеты, по умолчанию in_progress (эскалированные)
		if status == "" {
			status = string(models.StatusInProgress)
		}
		tickets, err = h.service.GetAllTicketsByStatus(c.Request.Context(), models.TicketStatus(status))
	case "user":
		// Клиент видит только свои тикеты, с возможной фильтрацией по статусу
		if status == "" {
			tickets, err = h.service.GetTicketsForClient(c.Request.Context(), userID)
		} else {
			tickets, err = h.service.GetTicketsForUserByStatus(c.Request.Context(), userID, models.TicketStatus(status))
		}
	case "cleaner":
		if status == "" {
			status = string(models.StatusOpen)
		}
		tickets, err = h.service.GetAllTicketsByStatus(
			c.Request.Context(), models.TicketStatus(status))
	default:
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tickets)
}
