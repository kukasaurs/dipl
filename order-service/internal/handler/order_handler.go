package handler

import (
	"cleaning-app/order-service/internal/models"
	"cleaning-app/order-service/internal/services"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
)

type OrderHandler struct {
	service services.OrderService
}

func NewOrderHandler(service services.OrderService) *OrderHandler {
	return &OrderHandler{service: service}
}

func (h *OrderHandler) CreateOrder(c *gin.Context) {
	userID := c.GetString("userId")
	var order models.Order
	if err := c.ShouldBindJSON(&order); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}
	order.ClientID = userID
	if err := h.service.CreateOrder(c.Request.Context(), &order); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, order)
}

func (h *OrderHandler) GetMyOrders(c *gin.Context) {
	userID := c.GetString("userId")
	orders, err := h.service.GetOrdersByClient(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, orders)
}

func (h *OrderHandler) UpdateOrder(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	var updated models.Order
	if err := c.ShouldBindJSON(&updated); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}
	if err := h.service.UpdateOrder(c.Request.Context(), id, &updated); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Order updated"})
}

func (h *OrderHandler) DeleteOrder(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	if err := h.service.DeleteOrder(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Order deleted"})
}

func (h *OrderHandler) AssignCleaner(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	var body struct {
		CleanerID string `json:"cleaner_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.CleanerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}
	if err := h.service.AssignCleaner(c.Request.Context(), id, body.CleanerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Cleaner assigned"})
}

func (h *OrderHandler) UnassignCleaner(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	if err := h.service.UnassignCleaner(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Cleaner unassigned"})
}

func (h *OrderHandler) ConfirmCompletion(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	var body struct {
		PhotoURL string `json:"photo_url"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.PhotoURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing photo URL"})
		return
	}
	if err := h.service.ConfirmCompletion(c.Request.Context(), id, body.PhotoURL); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Order marked as completed"})
}

func (h *OrderHandler) GetAllOrders(c *gin.Context) {
	orders, err := h.service.GetAllOrders(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, orders)
}

func (h *OrderHandler) FilterOrders(c *gin.Context) {
	filters := make(map[string]interface{})
	if status := c.Query("status"); status != "" {
		filters["status"] = status
	}
	if cleaner := c.Query("cleaner_id"); cleaner != "" {
		filters["cleaner_id"] = cleaner
	}
	if client := c.Query("client_id"); client != "" {
		filters["client_id"] = client
	}
	orders, err := h.service.FilterOrders(c.Request.Context(), filters)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, orders)
}

func (h *OrderHandler) GetActiveOrdersCount(c *gin.Context) {
	count, err := h.service.GetActiveOrdersCount(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": count})
}

func (h *OrderHandler) GetTotalRevenue(c *gin.Context) {
	revenue, err := h.service.GetTotalRevenue(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"revenue": revenue})
}
