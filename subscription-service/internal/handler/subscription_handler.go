package handler

import (
	"cleaning-app/subscription-service/internal/models"
	"cleaning-app/subscription-service/internal/utils"
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
	"time"
)

type SubscriptionHandler struct {
	service SubscriptionService
}

type SubscriptionService interface {
	Extend(ctx context.Context, id primitive.ObjectID, extraCleanings int) error
	PayForExtension(ctx context.Context, id primitive.ObjectID, extraDays int) error
	Create(ctx context.Context, sub *models.Subscription) error
	ProcessDailyOrders(ctx context.Context)
	Update(ctx context.Context, id primitive.ObjectID, update bson.M) error
	Cancel(ctx context.Context, id primitive.ObjectID) error
	GetAll(ctx context.Context) ([]models.Subscription, error)
	GetByClient(ctx context.Context, clientIDHex string) ([]models.Subscription, error)
	FindExpiringOn(ctx context.Context, targetDate time.Time) ([]models.Subscription, error)
	FindExpired(ctx context.Context, before time.Time) ([]models.Subscription, error)
	UpdateStatus(ctx context.Context, id primitive.ObjectID, status models.SubscriptionStatus) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Subscription, error)
}

func NewSubscriptionHandler(s SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{service: s}
}

// POST /api/subscriptions
func (h *SubscriptionHandler) Create(c *gin.Context) {
	// 1. Привязка JSON-параметров
	var in struct {
		OrderID    primitive.ObjectID `json:"order_id"     binding:"required"`
		StartDate  time.Time          `json:"start_date"   binding:"required"`
		EndDate    time.Time          `json:"end_date"     binding:"required"`
		DaysOfWeek []string           `json:"days_of_week" binding:"required,dive,oneof=Mon Tue Wed Thu Fri Sat Sun"`
		Price      float64            `json:"price"        binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 2. Получаем userId из JWT (строка), конвертим в ObjectID
	userIDHex := c.GetString("userId")
	userID, err := primitive.ObjectIDFromHex(userIDHex)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID in token"})
		return
	}

	// 3. Формируем модель подписки
	sub := models.Subscription{
		OrderID:         in.OrderID,
		UserID:          userID,
		StartDate:       in.StartDate,
		EndDate:         in.EndDate,
		DaysOfWeek:      in.DaysOfWeek,
		Price:           in.Price,
		NextPlannedDate: &in.StartDate,
	}

	// 4. Вызываем сервис для сохранения
	if err := h.service.Create(c.Request.Context(), &sub); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create subscription"})
		return
	}

	// 5. Уведомление клиенту о создании подписки
	go func() {
		payload := utils.NotificationPayload{
			RecipientID:   userIDHex,
			RecipientRole: "client",
			Title:         "Подписка создана",
			Body:          "Ваша подписка успешно создана и будет действовать до " + in.EndDate.Format("2006-01-02") + ".",
			Type:          "subscription_created",
			Channel:       "email",
			Data: map[string]interface{}{
				"subscription_id": sub.ID.Hex(),
				"start_date":      in.StartDate.Format(time.RFC3339),
				"end_date":        in.EndDate.Format(time.RFC3339),
			},
		}
		_ = utils.SendNotification(context.Background(), payload)
	}()

	// 6. Возвращаем ID созданной подписки
	c.JSON(http.StatusCreated, gin.H{"id": sub.ID.Hex()})
}

func (h *SubscriptionHandler) Extend(c *gin.Context) {
	id, _ := primitive.ObjectIDFromHex(c.Param("id"))
	var req struct {
		EndDate time.Time `json:"end_date" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 1) получить существующую подписку
	sub, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "subscription not found"})
		return
	}

	// 2) посчитать, сколько дополнительных уборок появится
	newCount := countScheduledDays(sub.DaysOfWeek, sub.EndDate.Add(24*time.Hour), req.EndDate)
	if newCount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "new end_date must be after current end_date"})
		return
	}

	// 3) обновить дату окончания
	if err := h.service.Update(c.Request.Context(), id, bson.M{"end_date": req.EndDate}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to extend subscription"})
		return
	}

	// 4) спровоцировать оплату за эти newCount уборок
	if err := h.service.PayForExtension(c.Request.Context(), id, newCount); err != nil {
		c.JSON(http.StatusPaymentRequired, gin.H{"error": "payment failed"})
		return
	}

	// 5. Уведомление клиенту о продлении подписки
	userIDHex := sub.UserID.Hex()
	go func() {
		payload := utils.NotificationPayload{
			RecipientID:   userIDHex,
			RecipientRole: "client",
			Title:         "Подписка продлена",
			Body:          "Ваша подписка продлена до " + req.EndDate.Format("2006-01-02") + ". Дополнительных уборок: " + fmt.Sprintf("%d", newCount) + ".",
			Type:          "subscription_extended",
			Channel:       "email",
			Data: map[string]interface{}{
				"subscription_id": sub.ID.Hex(),
				"new_end_date":    req.EndDate.Format(time.RFC3339),
				"extra_cleanings": newCount,
			},
		}
		_ = utils.SendNotification(context.Background(), payload)
	}()

	c.JSON(http.StatusOK, gin.H{"message": "subscription extended", "new_cleanings": newCount})
}

// вспомогательная функция
func countScheduledDays(daysOfWeek []string, from, to time.Time) int {
	set := map[string]struct{}{}
	for _, d := range daysOfWeek {
		set[d] = struct{}{}
	}
	cnt := 0
	for d := from; !d.After(to); d = d.Add(24 * time.Hour) {
		if _, ok := set[d.Weekday().String()[:3]]; ok {
			cnt++
		}
	}
	return cnt
}

func (h *SubscriptionHandler) GetAll(c *gin.Context) {
	role := c.GetString("role")
	if role != "admin" && role != "manager" {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}
	subs, err := h.service.GetAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch subscriptions"})
		return
	}
	c.JSON(http.StatusOK, subs)
}

// PUT /api/subscriptions/:id
func (h *SubscriptionHandler) Update(c *gin.Context) {
	idHex := c.Param("id")
	id, err := primitive.ObjectIDFromHex(idHex)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subscription ID"})
		return
	}

	// 2) Биндим только days_of_week
	var req struct {
		DaysOfWeek []string `json:"days_of_week" binding:"required,dive,oneof=Mon Tue Wed Thu Fri Sat Sun"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 3) Получаем подписку, чтобы знать userId для уведомления
	sub, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "subscription not found"})
		return
	}

	// 4) Сохраняем только это поле
	if err := h.service.Update(c.Request.Context(), id, bson.M{
		"days_of_week": req.DaysOfWeek,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update schedule"})
		return
	}

	// 5. Уведомление клиенту об изменении расписания
	userIDHex := sub.UserID.Hex()
	go func() {
		payload := utils.NotificationPayload{
			RecipientID:   userIDHex,
			RecipientRole: "client",
			Title:         "Расписание подписки обновлено",
			Body:          "Дни недели вашей подписки были изменены.",
			Type:          "subscription_updated",
			Channel:       "email",
			Data: map[string]interface{}{
				"subscription_id": sub.ID.Hex(),
				"new_schedule":    req.DaysOfWeek,
			},
		}
		_ = utils.SendNotification(context.Background(), payload)
	}()

	c.JSON(http.StatusOK, gin.H{"message": "schedule updated"})
}

// DELETE /api/subscriptions/:id
func (h *SubscriptionHandler) Cancel(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	// Получаем подписку, чтобы знать userId
	sub, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "subscription not found"})
		return
	}

	if err := h.service.Cancel(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cancel failed"})
		return
	}

	// Уведомление клиенту об отмене подписки
	userIDHex := sub.UserID.Hex()
	go func() {
		payload := utils.NotificationPayload{
			RecipientID:   userIDHex,
			RecipientRole: "client",
			Title:         "Подписка отменена",
			Body:          "Ваша подписка была успешно отменена.",
			Type:          "subscription_cancelled",
			Channel:       "email",
			Data: map[string]interface{}{
				"subscription_id": sub.ID.Hex(),
			},
		}
		_ = utils.SendNotification(context.Background(), payload)
	}()

	c.JSON(http.StatusOK, gin.H{"message": "subscription cancelled"})
}

// GetMy возвращает подписки текущего пользователя ("/subscriptions/my").
func (h *SubscriptionHandler) GetMy(c *gin.Context) {
	// 1) Извлекаем userId из контекста
	userIDHex := c.GetString("userId")
	if userIDHex == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// 2) Получаем список подписок
	subs, err := h.service.GetByClient(c.Request.Context(), userIDHex)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch subscriptions"})
		return
	}

	// 3) Гарантируем, что мы отдадим не nil, а пустой массив
	if subs == nil {
		subs = make([]models.Subscription, 0)
	}

	// 4) Подключаем уведомление при просмотре (необязательно, но можно уведомить менеджера)
	go func() {
		payload := utils.NotificationPayload{
			RecipientID:   "",
			RecipientRole: "manager",
			Title:         "Пользователь просматривает подписки",
			Body:          "Пользователь " + userIDHex + " запросил свои подписки.",
			Type:          "view_subscriptions",
			Channel:       "email",
			Data: map[string]interface{}{
				"user_id": userIDHex,
			},
		}
		_ = utils.SendNotification(context.Background(), payload)
	}()

	// 5) Отдаём JSON-массив подписок
	c.JSON(http.StatusOK, subs)
}
