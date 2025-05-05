package http

import (
	service "cleaning-app/notification-service/internal/services"
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service service.NotificationService
}

func NewHandler(service service.NotificationService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetNotifications(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	notifs, err := h.service.GetAll(context.Background(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch notifications"})
		return
	}
	c.JSON(http.StatusOK, notifs)
}

func (h *Handler) MarkAsRead(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "notification ID is required"})
		return
	}

	err := h.service.MarkAsRead(context.Background(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not mark as read"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "marked as read"})
}
