package handler

import (
	"cleaning-app/notification-service/internal/models"
	"cleaning-app/notification-service/internal/services"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
	"strconv"
)

type NotificationHandler struct {
	service *services.NotificationService
}

type SendNotificationRequest struct {
	UserID       string            `json:"user_id" binding:"required"`
	Role         string            `json:"role" binding:"required"`
	Title        string            `json:"title" binding:"required"`
	Message      string            `json:"message" binding:"required"`
	Type         string            `json:"type" binding:"required"`
	DeliveryType string            `json:"delivery_type" binding:"required"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

func NewNotificationHandler(service *services.NotificationService) *NotificationHandler {
	return &NotificationHandler{service: service}
}

// GetNotifications возвращает список уведомлений пользователя
func (h *NotificationHandler) GetNotifications(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User ID not found in context"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User ID is not a string"})
		return
	}

	limitStr := c.DefaultQuery("limit", "10")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit"})
		return
	}

	offset, err := strconv.ParseInt(offsetStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid offset"})
		return
	}

	notifications, err := h.service.GetNotifications(c.Request.Context(), userIDStr, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get notifications"})
		return
	}

	c.JSON(http.StatusOK, notifications)
}

// MarkAsRead отмечает уведомление как прочитанное
func (h *NotificationHandler) MarkAsRead(c *gin.Context) {
	idStr := c.Param("id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	err = h.service.MarkAsRead(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark as read"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "marked as read"})
}

// SendManualNotification создает уведомление вручную через API
func (h *NotificationHandler) SendManualNotification(c *gin.Context) {
	var req SendNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Преобразование строковых типов в константы
	var notifType models.NotificationType
	var deliveryType models.DeliveryMethod

	switch req.Type {
	case "order_event":
		notifType = models.TypeOrderEvent
	case "support_event":
		notifType = models.TypeSupportEvent
	case "admin_alert":
		notifType = models.TypeAdminAlert
	default:
		notifType = models.TypeSystemMessage
	}

	switch req.DeliveryType {
	case "email":
		deliveryType = models.DeliveryEmail
	case "sms":
		deliveryType = models.DeliverySMS
	default:
		deliveryType = models.DeliveryPush
	}

	notification := &models.Notification{
		UserID:       req.UserID,
		Title:        req.Title,
		Message:      req.Message,
		Type:         notifType,
		DeliveryType: deliveryType,
		IsRead:       false,
		Metadata:     req.Metadata,
	}

	if err := h.service.SendNotification(c.Request.Context(), notification); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send notification"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "notification sent"})
}
