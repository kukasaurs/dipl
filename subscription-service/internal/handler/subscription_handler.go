package handler

import (
	"cleaning-app/subscription-service/internal/models"
	"cleaning-app/subscription-service/internal/utils"
	"context"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"net/http"
	"time"
)

type SubscriptionHandler struct {
	service       SubscriptionService
	orderClient   *utils.OrderServiceClient
	paymentClient *utils.PaymentServiceClient
}

type SubscriptionService interface {
	Create(ctx context.Context, sub *models.Subscription) error
	Extend(ctx context.Context, id primitive.ObjectID, extraCleanings int) error
	PayForExtension(ctx context.Context, id primitive.ObjectID, extraDays int, authHeader string) error
	ProcessDailyOrders(ctx context.Context)
	Update(ctx context.Context, id primitive.ObjectID, update primitive.M) error
	Cancel(ctx context.Context, id primitive.ObjectID) error
	GetAll(ctx context.Context) ([]models.Subscription, error)
	GetByClient(ctx context.Context, clientIDHex string) ([]models.Subscription, error)
	FindExpiringOn(ctx context.Context, targetDate time.Time) ([]models.Subscription, error)
	FindExpired(ctx context.Context, before time.Time) ([]models.Subscription, error)
	UpdateStatus(ctx context.Context, id primitive.ObjectID, status models.SubscriptionStatus) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Subscription, error)
	NextDates(spec models.ScheduleSpec, from time.Time, until time.Time) []time.Time
}

func NewSubscriptionHandler(svc SubscriptionService, orderClient *utils.OrderServiceClient, paymentClient *utils.PaymentServiceClient) *SubscriptionHandler {
	return &SubscriptionHandler{
		service:       svc,
		orderClient:   orderClient,
		paymentClient: paymentClient,
	}
}

func (h *SubscriptionHandler) GetSubscriptionByIDHTTP(c *gin.Context) {
	idHex := c.Param("id")
	id, err := primitive.ObjectIDFromHex(idHex)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subscription ID"})
		return
	}
	sub, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "subscription not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch subscription"})
		return
	}
	c.JSON(http.StatusOK, sub)
}

func (h *SubscriptionHandler) Create(c *gin.Context) {
	var in struct {
		OrderID     primitive.ObjectID `json:"order_id"    binding:"required"`
		StartDate   time.Time          `json:"start_date"  binding:"required"`
		EndDate     time.Time          `json:"end_date"    binding:"required"`
		Frequency   models.Frequency   `json:"frequency"    binding:"required"`
		DaysOfWeek  []string           `json:"days_of_week" binding:"required"`
		WeekNumbers []int              `json:"week_numbers"`
	}

	// 1) считать JSON
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userIDHex := c.GetString("userId")
	userID, err := primitive.ObjectIDFromHex(userIDHex)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	// 3) проверяем валидность schedule
	if len(in.DaysOfWeek) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "days_of_week cannot be empty"})
		return
	}
	switch in.Frequency {
	case models.Weekly:
		// week_numbers игнорируем
	case models.BiWeekly:
		if len(in.WeekNumbers) != 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "week_numbers must have exactly 2 items for biweekly"})
			return
		}
	case models.TriWeekly:
		if len(in.WeekNumbers) != 3 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "week_numbers must have exactly 3 items for triweekly"})
			return
		}
	case models.Monthly:
		if len(in.WeekNumbers) != 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "week_numbers must have exactly 1 item for monthly"})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid frequency"})
		return
	}

	// 4) Получаем цену из order-service (через orderClient)
	orderIDHex := in.OrderID.Hex()
	authHeader := c.GetHeader("Authorization")
	orderResp, err := h.orderClient.GetOrderByID(c.Request.Context(), orderIDHex, authHeader)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to fetch order: " + err.Error()})
		return
	}
	calculatedPrice := orderResp.TotalPrice

	now := time.Now().UTC()
	// 5) собираем модель новой подписки
	sub := &models.Subscription{
		OrderID:   in.OrderID,
		UserID:    userID,
		StartDate: in.StartDate.UTC(),
		EndDate:   in.EndDate.UTC(),
		Schedule: models.ScheduleSpec{
			Frequency:   in.Frequency,
			DaysOfWeek:  in.DaysOfWeek,
			WeekNumbers: in.WeekNumbers,
		},
		Price:           calculatedPrice,
		Status:          models.StatusActive,
		CreatedAt:       now,
		UpdatedAt:       now,
		LastOrderDate:   nil,
		NextPlannedDate: nil, // посчитаем ниже
	}

	// 6) рассчитываем первую NextPlannedDate
	windowStart := time.Now().UTC()
	if sub.StartDate.After(windowStart) {
		windowStart = sub.StartDate
	}
	candidates := h.service.NextDates(sub.Schedule, windowStart, sub.EndDate)
	if len(candidates) > 0 {
		first := candidates[0]
		sub.NextPlannedDate = &first
	}

	// 7) сохраняем подписку
	if err := h.service.Create(c.Request.Context(), sub); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 8) отправляем уведомление пользователю о создании подписки
	go func() {
		_ = utils.SendNotificationEvent(context.Background(), utils.NotificationEvent{
			UserID:    userID.Hex(),
			Role:      "client",
			Type:      "subscription_updated", // или создай тип "subscription_created", если нужно различать
			ExtraData: map[string]string{"subscription_id": sub.ID.Hex()},
		})
	}()

	// 8) отвечаем новым объектом
	c.JSON(http.StatusCreated, sub)
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

	sub, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "subscription not found"})
		return
	}

	newCount := countScheduledDays(sub.Schedule.DaysOfWeek, sub.EndDate.Add(24*time.Hour), req.EndDate)
	if newCount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "new end_date must be after current end_date"})
		return
	}

	if err := h.service.Update(c.Request.Context(), id, bson.M{"end_date": req.EndDate}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to extend subscription"})
		return
	}

	authHeader := c.GetHeader("Authorization")
	if err := h.service.PayForExtension(c.Request.Context(), id, newCount, authHeader); err != nil {
		c.JSON(http.StatusPaymentRequired, gin.H{"error": "payment failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "subscription extended", "new_cleanings": newCount})

	// отправляем уведомление пользователю о продлении
	go func() {
		_ = utils.SendNotificationEvent(context.Background(), utils.NotificationEvent{
			UserID:    sub.UserID.Hex(),
			Role:      "client",
			Type:      "subscription_updated", // или "subscription_extended"
			ExtraData: map[string]string{"subscription_id": sub.ID.Hex()},
		})
	}()
}

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

	var req struct {
		DaysOfWeek []string `json:"days_of_week" binding:"required,dive,oneof=Mon Tue Wed Thu Fri Sat Sun"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 3) Получаем подписку, чтобы знать userId для уведомления
	_, err = h.service.GetByID(c.Request.Context(), id)
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

	c.JSON(http.StatusOK, gin.H{"message": "schedule updated"})
	go func() {
		_ = utils.SendNotificationEvent(context.Background(), utils.NotificationEvent{
			UserID:    id.Hex(), // если sub.UserID есть, лучше sub.UserID.Hex()
			Role:      "client",
			Type:      "subscription_updated",
			ExtraData: map[string]string{"subscription_id": id.Hex()},
		})
	}()
}

// DELETE /api/subscriptions/:id
func (h *SubscriptionHandler) Cancel(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	// Получаем подписку, чтобы знать userId
	_, err = h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "subscription not found"})
		return
	}

	if err := h.service.Cancel(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cancel failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "subscription cancelled"})
	go func() {
		_ = utils.SendNotificationEvent(context.Background(), utils.NotificationEvent{
			UserID:    id.Hex(), // лучше sub.UserID.Hex()
			Role:      "client",
			Type:      "subscription_updated", // или "subscription_cancelled" если заведёшь такой тип
			ExtraData: map[string]string{"subscription_id": id.Hex()},
		})
	}()
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

	// 5) Отдаём JSON-массив подписок
	c.JSON(http.StatusOK, subs)
}
