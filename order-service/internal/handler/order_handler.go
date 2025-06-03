package handler

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"net/http"
	"time"

	"cleaning-app/order-service/internal/config"
	"cleaning-app/order-service/internal/models"
	"cleaning-app/order-service/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type OrderHandler struct {
	service OrderService
	rdb     *redis.Client
	cfg     *config.Config
}

type OrderService interface {
	CreateOrder(ctx context.Context, order *models.Order) error
	UpdateOrder(ctx context.Context, id primitive.ObjectID, updated *models.Order) error
	DeleteOrder(ctx context.Context, id primitive.ObjectID) error

	AssignCleaner(ctx context.Context, id primitive.ObjectID, cleanerID string) error
	AssignCleaners(ctx context.Context, id primitive.ObjectID, cleanerIDs []string) error
	UnassignCleaner(ctx context.Context, id primitive.ObjectID, cleanerID string) error

	ConfirmCompletion(ctx context.Context, id primitive.ObjectID, photoURL string) error

	GetOrderByID(ctx context.Context, id primitive.ObjectID) (*models.Order, error)
	GetAllOrders(ctx context.Context) ([]models.Order, error)
	GetOrdersByClient(ctx context.Context, clientID string) ([]models.Order, error)
	FilterOrders(ctx context.Context, filter map[string]interface{}) ([]models.Order, error)
	GetActiveOrdersCount(ctx context.Context) (int64, error)
	GetTotalRevenue(ctx context.Context) (float64, error)
	UpdatePaymentStatus(ctx context.Context, orderID string, status string) error
}

// NewOrderHandler создаёт новый хендлер для заказов и получает конфиг
func NewOrderHandler(service OrderService, rdb *redis.Client, cfg *config.Config) *OrderHandler {
	return &OrderHandler{service: service, rdb: rdb, cfg: cfg}
}

func (h *OrderHandler) GetOrderByIDHTTP(c *gin.Context) {
	idHex := c.Param("id")
	id, err := primitive.ObjectIDFromHex(idHex)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order ID"})
		return
	}
	order, err := h.service.GetOrderByID(c.Request.Context(), id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch order"})
		return
	}
	// просто вернём всё, что модель Order держит, включая total_price
	c.JSON(http.StatusOK, order)
}

func (h *OrderHandler) HandlePaymentNotification(c *gin.Context) {
	var note struct {
		EntityID string `json:"entity_id"`
		Status   string `json:"status"`
	}
	if err := c.ShouldBindJSON(&note); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if err := h.service.UpdatePaymentStatus(c.Request.Context(), note.EntityID, note.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

func (h *OrderHandler) clearCache(ctx context.Context) {
	h.rdb.Del(ctx, "orders:activeCount", "orders:totalRevenue", "orders:all")
}

// CreateOrder остаётся без изменений по логике, но вызываем SendNotification с новым контекстом
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
	h.clearCache(c.Request.Context())

	notification := utils.NotificationRequest{
		Role:         "manager",
		Title:        "Новый заказ",
		Message:      fmt.Sprintf("Поступил новый заказ #%s от клиента %s.", order.ID.Hex(), userID),
		Type:         "new_order",
		DeliveryType: "email",
		Metadata: map[string]string{
			"order_id":  order.ID.Hex(),
			"client_id": userID,
		},
	}

	go func(n utils.NotificationRequest) {
		ctxNotify, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := utils.SendNotification(ctxNotify, h.cfg, n); err != nil {
			log.Printf("[OrderHandler] failed to send notification: %v\n", err)
		}
	}(notification)

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
	var orderUpdate models.Order
	if err := c.ShouldBindJSON(&orderUpdate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}
	if err := h.service.UpdateOrder(c.Request.Context(), id, &orderUpdate); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.clearCache(c.Request.Context())

	userID := c.GetString("userId")
	notification := utils.NotificationRequest{
		Role:         "manager",
		Title:        "Заказ обновлён",
		Message:      fmt.Sprintf("Пользователь %s внёс изменения в заказ #%s.", userID, id.Hex()),
		Type:         "order_updated",
		DeliveryType: "email",
		Metadata: map[string]string{
			"order_id": id.Hex(),
		},
	}

	go func(n utils.NotificationRequest) {
		ctxNotify, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := utils.SendNotification(ctxNotify, h.cfg, n); err != nil {
			log.Printf("[OrderHandler] failed to send notification: %v\n", err)
		}
	}(notification)

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
	h.clearCache(c.Request.Context())

	userID := c.GetString("userId")
	notification := utils.NotificationRequest{
		UserID:       "",
		Role:         "manager",
		Title:        "Заказ удалён",
		Message:      fmt.Sprintf("Пользователь %s удалил заказ #%s.", userID, id.Hex()),
		Type:         "order_deleted",
		DeliveryType: "email",
		Metadata: map[string]string{
			"order_id": id.Hex(),
		},
	}

	go func(n utils.NotificationRequest) {
		ctxNotify, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := utils.SendNotification(ctxNotify, h.cfg, n); err != nil {
			log.Printf("[OrderHandler] failed to send notification: %v\n", err)
		}
	}(notification)

	c.JSON(http.StatusOK, gin.H{"message": "Order deleted"})
}

// AssignCleaners — теперь правильно формируем NotificationRequest
func (h *OrderHandler) AssignCleaners(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	var body struct {
		CleanerIDs []string `json:"cleaner_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.CleanerIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provide non-empty array cleaner_ids"})
		return
	}

	if err := h.service.AssignCleaners(c.Request.Context(), id, body.CleanerIDs); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	h.clearCache(c.Request.Context())

	for _, cleanerID := range body.CleanerIDs {
		notification := utils.NotificationRequest{
			UserID:       cleanerID,
			Role:         "cleaner",
			Title:        "Вам назначен новый заказ",
			Message:      fmt.Sprintf("Вы назначены на заказ #%s. Проверьте детали в приложении.", id.Hex()),
			Type:         "assigned_order",
			DeliveryType: "push",
			Metadata: map[string]string{
				"order_id": id.Hex(),
			},
		}
		go func(cln string, n utils.NotificationRequest) {
			ctxNotify, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := utils.SendNotification(ctxNotify, h.cfg, n); err != nil {
				log.Printf("[OrderHandler] failed to send notification to %s: %v\n", cln, err)
			}
		}(cleanerID, notification)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Cleaners assigned"})
}

// AssignCleaner вызывает AssignCleaners{…}, но тоже надо правильно формировать NotificationRequest
func (h *OrderHandler) AssignCleaner(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	var body struct {
		CleanerID string `json:"cleaner_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.CleanerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provide cleaner_id"})
		return
	}
	if err := h.service.AssignCleaners(c.Request.Context(), id, []string{body.CleanerID}); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	h.clearCache(c.Request.Context())

	notification := utils.NotificationRequest{
		UserID:       body.CleanerID,
		Role:         "cleaner",
		Title:        "Вам назначен заказ",
		Message:      fmt.Sprintf("Вы назначены на заказ #%s. Проверьте детали в приложении.", id.Hex()),
		Type:         "assigned_order",
		DeliveryType: "push",
		Metadata: map[string]string{
			"order_id": id.Hex(),
		},
	}
	go func(n utils.NotificationRequest) {
		ctxNotify, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := utils.SendNotification(ctxNotify, h.cfg, n); err != nil {
			log.Printf("[OrderHandler] failed to send notification: %v\n", err)
		}
	}(notification)

	c.JSON(http.StatusOK, gin.H{"message": "Cleaner assigned"})
}

// UnassignCleaner теперь тоже передаём новый контекст и правильный Request
func (h *OrderHandler) UnassignCleaner(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	var body struct {
		CleanerID string `json:"cleaner_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.CleanerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provide cleaner_id"})
		return
	}
	if err := h.service.UnassignCleaner(c.Request.Context(), id, body.CleanerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.clearCache(c.Request.Context())

	notification := utils.NotificationRequest{
		UserID:       body.CleanerID,
		Role:         "cleaner",
		Title:        "Вас сняли с заказа",
		Message:      fmt.Sprintf("Вас сняли с заказа #%s.", id.Hex()),
		Type:         "unassigned_order",
		DeliveryType: "push",
		Metadata: map[string]string{
			"order_id": id.Hex(),
		},
	}
	go func(n utils.NotificationRequest) {
		ctxNotify, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := utils.SendNotification(ctxNotify, h.cfg, n); err != nil {
			log.Printf("[OrderHandler] failed to send notification: %v\n", err)
		}
	}(notification)

	c.JSON(http.StatusOK, gin.H{"message": "Cleaner unassigned"})
}

// ConfirmCompletion — получаем clientID из заказа и отправляем уведомление
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
	h.clearCache(c.Request.Context())

	order, err := h.service.GetOrderByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	clientID := order.ClientID

	notification := utils.NotificationRequest{
		UserID:       clientID,
		Role:         "user",
		Title:        "Уборка завершена",
		Message:      fmt.Sprintf("Уборка по заказу #%s успешно завершена. Оцените работу клинера!", id.Hex()),
		Type:         "cleaning_completed",
		DeliveryType: "push",
		Metadata: map[string]string{
			"order_id": id.Hex(),
		},
	}
	go func(n utils.NotificationRequest) {
		ctxNotify, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := utils.SendNotification(ctxNotify, h.cfg, n); err != nil {
			log.Printf("[OrderHandler] failed to send notification: %v\n", err)
		}
	}(notification)

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
