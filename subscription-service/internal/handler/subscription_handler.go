package handler

import (
	"cleaning-app/subscription-service/internal/models"
	"cleaning-app/subscription-service/internal/services"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
	"time"
)

type SubscriptionHandler struct {
	service services.SubscriptionService
}

// NewSubscriptionHandler creates a new subscription handler
func NewSubscriptionHandler(service services.SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{service: service}
}

// Create handles the creation of a new subscription
func (h *SubscriptionHandler) Create(c *gin.Context) {
	// Get the client ID from the authenticated user
	clientID := c.GetString("user_id")
	if clientID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var sub models.Subscription
	if err := c.ShouldBindJSON(&sub); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}

	// Set client ID and default fields
	sub.ClientID = clientID
	sub.ID = primitive.NewObjectID()
	sub.CreatedAt = time.Now()
	sub.UpdatedAt = time.Now()

	// Create context with user ID for logging
	ctx := c.Request.Context()

	if err := h.service.Create(ctx, &sub); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, sub)
}

// Update handles updating an existing subscription
func (h *SubscriptionHandler) Update(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
		return
	}

	var update bson.M
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}

	// Create context with user ID for logging
	ctx := c.Request.Context()

	if err := h.service.Update(ctx, id, update); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "subscription updated"})
}

// Cancel handles cancelling a subscription
func (h *SubscriptionHandler) Cancel(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
		return
	}

	// Create context with user ID for logging
	ctx := c.Request.Context()

	if err := h.service.Cancel(ctx, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "subscription cancelled"})
}

// GetMy handles retrieving subscriptions for the current client
func (h *SubscriptionHandler) GetMy(c *gin.Context) {
	clientID := c.GetString("user_id")
	if clientID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	subs, err := h.service.GetByClient(c.Request.Context(), clientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, subs)
}

// Extend handles extending a subscription by a specified duration
func (h *SubscriptionHandler) Extend(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var body struct {
		DurationDays int `json:"duration_days"`
	}

	if err := c.ShouldBindJSON(&body); err != nil || body.DurationDays <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid duration"})
		return
	}

	// Create context with user ID for logging
	ctx := c.Request.Context()

	if err := h.service.Extend(ctx, id, body.DurationDays); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Subscription extended"})
}

// GetAll handles retrieving all subscriptions (admin/manager only)
func (h *SubscriptionHandler) GetAll(c *gin.Context) {
	role := c.GetString("role")
	userID := c.GetString("user_id")

	var subs []models.Subscription
	var err error

	if role == "admin" || role == "manager" {
		subs, err = h.service.GetAll(c.Request.Context())
	} else {
		subs, err = h.service.GetByClient(c.Request.Context(), userID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, subs)
}
