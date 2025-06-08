package handler

import (
	"cleaning-app/order-service/internal/config"
	"cleaning-app/order-service/internal/models"
	"cleaning-app/order-service/internal/utils"
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"net/http"
	"time"
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

	CountJobsDone(ctx context.Context, cleanerID primitive.ObjectID) (int64, error)
	FinishOrder(ctx context.Context, orderID primitive.ObjectID, cleanerID primitive.ObjectID, photoURL string) error
	GetOrderForCleaner(ctx context.Context, orderID primitive.ObjectID, cleanerID primitive.ObjectID) (*models.Order, error)
	GetOrdersForCleaner(ctx context.Context, cleanerID primitive.ObjectID) ([]models.Order, error)
	AddReview(ctx context.Context, orderID string, rating int, comment string, authHeader string) error
}

// NewOrderHandler создаёт новый хендлер для заказов и получает конфиг
func NewOrderHandler(service OrderService, rdb *redis.Client, cfg *config.Config) *OrderHandler {
	return &OrderHandler{service: service, rdb: rdb, cfg: cfg}
}

// POST /orders/:id/review
func (h *OrderHandler) AddOrderReview(c *gin.Context) {
	orderID := c.Param("id")
	var req struct {
		Rating  int    `json:"rating" binding:"required,min=1,max=5"`
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	authHeader := c.GetHeader("Authorization")

	if err := h.service.AddReview(c.Request.Context(), orderID, req.Rating, req.Comment, authHeader); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusCreated)
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

	// ── Уведомление о статусе платежа ──
	orderIDHex := note.EntityID
	// Преобразуем строку в ObjectID
	orderID, errID := primitive.ObjectIDFromHex(orderIDHex)
	if errID == nil {
		// Достаём order, чтобы узнать clientID и сумму, если нужно
		order, err := h.service.GetOrderByID(c.Request.Context(), orderID)
		if err == nil {
			clientID := order.ClientID
			evtType := "payment_failed"
			if note.Status == "success" {
				evtType = "payment_successful"
			}
			go func() {
				_ = utils.SendNotificationEvent(context.Background(), utils.NotificationEvent{
					UserID:    clientID,
					Role:      "client",
					Type:      evtType,
					ExtraData: map[string]string{"order_id": orderIDHex, "amount": fmt.Sprintf("%.2f", order.TotalPrice)},
				})
			}()
		}
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

	// Предполагаем, что поле Status приходит в JSON как orderUpdate.Status
	oldOrder, _ := h.service.GetOrderByID(c.Request.Context(), id)
	oldStatus := ""
	if oldOrder != nil {
		oldStatus = string(oldOrder.Status)
	}

	if err := h.service.UpdateOrder(c.Request.Context(), id, &orderUpdate); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.clearCache(c.Request.Context())

	// ── Если статус переключился на "confirmed" ──
	if orderUpdate.Status == "confirmed" && oldStatus != "confirmed" {
		// Получим clientID из сохранённого заказа (до обновления) или из orderUpdate.ClientID
		clientID := orderUpdate.ClientID
		if clientID == "" && oldOrder != nil {
			clientID = oldOrder.ClientID
		}
		go func() {
			_ = utils.SendNotificationEvent(context.Background(), utils.NotificationEvent{
				UserID:    clientID,
				Role:      "client",
				Type:      "order_confirmed",
				ExtraData: map[string]string{"order_id": id.Hex()},
			})
		}()
	}

	// ── Если статус переключился на "cancelled" ──
	if orderUpdate.Status == "cancelled" && oldStatus != "cancelled" {
		clientID := orderUpdate.ClientID
		if clientID == "" && oldOrder != nil {
			clientID = oldOrder.ClientID
		}
		go func() {
			_ = utils.SendNotificationEvent(context.Background(), utils.NotificationEvent{
				UserID:    clientID,
				Role:      "client",
				Type:      "order_cancelled",
				ExtraData: map[string]string{"order_id": id.Hex()},
			})
		}()
	}

	// ── Во всех остальных случаях (любое обновление полей) ──
	//    отправляем "order_updated" клиенту и всем назначенным cleaner-ам
	clientID := orderUpdate.ClientID
	if clientID == "" && oldOrder != nil {
		clientID = oldOrder.ClientID
	}
	// Получаем список cleanerIDs из базы (предполагается, что orderUpdate.CleanerID хранит массив или single string)
	var cleanerIDs []string
	if len(orderUpdate.CleanerID) > 0 {
		cleanerIDs = orderUpdate.CleanerID
	} else if oldOrder != nil {
		cleanerIDs = oldOrder.CleanerID
	}

	// Отправляем уведомление клиенту
	go func() {
		_ = utils.SendNotificationEvent(context.Background(), utils.NotificationEvent{
			UserID:    clientID,
			Role:      "client",
			Type:      "order_updated",
			ExtraData: map[string]string{"order_id": id.Hex()},
		})
	}()

	// И каждому назначенному cleaner-у
	for _, cid := range cleanerIDs {
		go func(cleanerID string) {
			_ = utils.SendNotificationEvent(context.Background(), utils.NotificationEvent{
				UserID:    cleanerID,
				Role:      "cleaner",
				Type:      "order_updated",
				ExtraData: map[string]string{"order_id": id.Hex()},
			})
		}(cid)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Order updated"})
}

func (h *OrderHandler) DeleteOrder(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	// Сначала вытащим старый заказ, чтобы знать clientID и cleanerIDs
	oldOrder, err := h.service.GetOrderByID(c.Request.Context(), id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.DeleteOrder(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.clearCache(c.Request.Context())

	// ── Уведомляем клиента ──
	clientID := oldOrder.ClientID
	go func() {
		_ = utils.SendNotificationEvent(context.Background(), utils.NotificationEvent{
			UserID:    clientID,
			Role:      "client",
			Type:      "order_deleted",
			ExtraData: map[string]string{"order_id": id.Hex()},
		})
	}()

	// ── Уведомляем всех назначенных cleaner-ов ──
	for _, cid := range oldOrder.CleanerID {
		go func(cleanerID string) {
			_ = utils.SendNotificationEvent(context.Background(), utils.NotificationEvent{
				UserID:    cleanerID,
				Role:      "cleaner",
				Type:      "order_deleted",
				ExtraData: map[string]string{"order_id": id.Hex()},
			})
		}(cid)
	}

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
		go func(cid string) {
			_ = utils.SendNotificationEvent(context.Background(), utils.NotificationEvent{
				UserID:    cid,
				Role:      "cleaner",
				Type:      "assigned_order",
				ExtraData: map[string]string{"order_id": id.Hex()},
			})
		}(cleanerID)
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
	go func() {
		_ = utils.SendNotificationEvent(context.Background(), utils.NotificationEvent{
			UserID:    body.CleanerID,
			Role:      "cleaner",
			Type:      "assigned_order",
			ExtraData: map[string]string{"order_id": id.Hex()},
		})
	}()
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

	// 1) Меняем статус заказа в БД (service.ConfirmCompletion → устанавливает статус "completed")
	if err := h.service.ConfirmCompletion(c.Request.Context(), id, body.PhotoURL); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.clearCache(c.Request.Context())

	// 2) Достаём заказ, чтобы узнать clientID
	completedOrder, err := h.service.GetOrderByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	clientID := completedOrder.ClientID

	// ── Уведомление: уборка завершена ──
	go func() {
		_ = utils.SendNotificationEvent(context.Background(), utils.NotificationEvent{
			UserID:    clientID,
			Role:      "client",
			Type:      "cleaning_completed",
			ExtraData: map[string]string{"order_id": id.Hex()},
		})
	}()

	// ── Отложенный запрос отзыва через час ──
	go func(orderID, uID string) {
		time.AfterFunc(time.Hour, func() {
			_ = utils.SendNotificationEvent(context.Background(), utils.NotificationEvent{
				UserID:    uID,
				Role:      "client",
				Type:      "review_request",
				ExtraData: map[string]string{"order_id": orderID},
			})
		})
	}(id.Hex(), clientID)

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

func (h *OrderHandler) GetCleanerOrders(c *gin.Context) {
	// 1) Получаем userId из JWT (middleware кладет в context)
	cleanerHex := c.GetString("userId")
	cleanerID, err := primitive.ObjectIDFromHex(cleanerHex)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cleaner ID"})
		return
	}

	// 2) Просим Service дать список
	orders, err := h.service.GetOrdersForCleaner(c.Request.Context(), cleanerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, orders)
}

// GetCleanerOrder возвращает детали заказа по :id, если клинер назначен.
func (h *OrderHandler) GetCleanerOrder(c *gin.Context) {
	// 1) Extract cleanerID из JWT
	cleanerHex := c.GetString("userId")
	cleanerID, err := primitive.ObjectIDFromHex(cleanerHex)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cleaner ID"})
		return
	}
	// 2) Extract orderID из path
	idHex := c.Param("id")
	orderID, err := primitive.ObjectIDFromHex(idHex)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order ID"})
		return
	}
	// 3) Получаем детальный Order из Service
	order, err := h.service.GetOrderForCleaner(c.Request.Context(), orderID, cleanerID)
	if err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, order)
}

// FinishOrder помечает заказ как «completed» и сохраняет фото. Файл приходит в multipart/form-data с ключом "photo".
//func (h *OrderHandler) FinishOrder(c *gin.Context) {
//	// 1) cleanerID из JWT
//	cleanerHex := c.GetString("userId")
//	cleanerID, err := primitive.ObjectIDFromHex(cleanerHex)
//	if err != nil {
//		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cleaner ID"})
//		return
//	}
//	// 2) orderID из path
//	idHex := c.Param("id")
//	orderID, err := primitive.ObjectIDFromHex(idHex)
//	if err != nil {
//		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order ID"})
//		return
//	}
//	// 3) Получаем файл из формы
//	file, err := c.FormFile("photo")
//	if err != nil {
//		c.JSON(http.StatusBadRequest, gin.H{"error": "photo is required"})
//		return
//	}
//	// 4) Загружаем в Review-Media-Service (утилита utils.UploadFileToMediaService делает HTTP-запрос к вашему микросервису и возвращает URL).
//	url, err := utils.UploadFileToMediaService(c.Request.Context(), h.mediaBaseURL, file)
//	if err != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
//		return
//	}
//	// 5) Обновляем заказ в БД через Service
//	if err := h.service.FinishOrder(c.Request.Context(), orderID, cleanerID, url); err != nil {
//		if err == service.ErrForbidden {
//			c.JSON(http.StatusForbidden, gin.H{"error": "not assigned to this order"})
//			return
//		}
//		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
//		return
//	}
//	c.JSON(http.StatusOK, gin.H{"ok": true, "url": url})
//}

// GetJobsDone возвращает число завершённых заказов для клинера с :id (без привязки к JWT).
func (h *OrderHandler) GetJobsDone(c *gin.Context) {
	// 1) Извлекаем cleanerID из path
	idHex := c.Param("id")
	cleanerID, err := primitive.ObjectIDFromHex(idHex)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cleaner ID"})
		return
	}
	// 2) Просим Service вернуть count
	count, err := h.service.CountJobsDone(c.Request.Context(), cleanerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"jobs_done": count})
}
