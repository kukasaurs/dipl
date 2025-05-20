package handler

import (
	"cleaning-app/support-service/internal/models"
	"cleaning-app/support-service/internal/services"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
)

type SupportHandler struct {
	service *services.SupportService
}

func NewSupportHandler(s *services.SupportService) *SupportHandler {
	return &SupportHandler{service: s}
}

// POST /api/support/tickets
func (h *SupportHandler) CreateTicket(c *gin.Context) {
	var ticket models.Ticket
	if err := c.ShouldBindJSON(&ticket); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	ticket.ClientID = c.GetString("userId")

	if err := h.service.CreateTicket(c.Request.Context(), &ticket); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create ticket"})
		return
	}
	c.JSON(http.StatusCreated, ticket)
}

// GET /api/support/tickets/my
func (h *SupportHandler) GetMyTickets(c *gin.Context) {
	clientID := c.GetString("userId")
	tickets, err := h.service.GetTicketsForClient(c.Request.Context(), clientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get tickets"})
		return
	}
	c.JSON(http.StatusOK, tickets)
}

// GET /api/support/tickets (admin/manager only)
func (h *SupportHandler) GetAllTickets(c *gin.Context) {
	role := c.GetString("role")
	if role != "admin" && role != "manager" {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}
	tickets, err := h.service.GetAllTickets(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get tickets"})
		return
	}
	c.JSON(http.StatusOK, tickets)
}

// PUT /api/support/tickets/:id/status
func (h *SupportHandler) UpdateTicketStatus(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
		return
	}
	var req struct {
		Status models.TicketStatus `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}

	if err := h.service.UpdateTicketStatus(c.Request.Context(), id, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update status"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "status updated"})
}

// POST /api/support/tickets/:id/messages
func (h *SupportHandler) SendMessage(c *gin.Context) {
	ticketID, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
		return
	}
	var msg models.Message
	if err := c.ShouldBindJSON(&msg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message body"})
		return
	}
	msg.TicketID = ticketID
	msg.SenderID = c.GetString("userId")
	msg.SenderRole = c.GetString("role")

	if err := h.service.AddMessage(c.Request.Context(), &msg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send message"})
		return
	}
	c.JSON(http.StatusCreated, msg)
}

// GET /api/support/tickets/:id/messages
func (h *SupportHandler) GetMessages(c *gin.Context) {
	ticketID, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
		return
	}
	msgs, err := h.service.GetMessagesByTicket(c.Request.Context(), ticketID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get messages"})
		return
	}
	c.JSON(http.StatusOK, msgs)
}
