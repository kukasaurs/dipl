package handler

import (
	"net/http"

	"cleaning-app/support-service/internal/models"
	"cleaning-app/support-service/internal/services"
	"github.com/gin-gonic/gin"
)

type ChatHandler struct {
	Service *services.ChatService
}

func NewChatHandler(service *services.ChatService) *ChatHandler {
	return &ChatHandler{Service: service}
}

func (h *ChatHandler) SendMessage(c *gin.Context) {
	var msg models.Message
	if err := c.ShouldBindJSON(&msg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	if err := h.Service.SendMessage(c, &msg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not send message"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "message sent"})
}

func (h *ChatHandler) GetMessages(c *gin.Context) {
	user1 := c.Query("user1")
	user2 := c.Query("user2")
	messages, err := h.Service.GetMessages(c, user1, user2)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch messages"})
		return
	}
	c.JSON(http.StatusOK, messages)
}
