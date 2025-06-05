package services

import (
	"cleaning-app/subscription-service/internal/utils"
	"context"
	"fmt"
	"time"

	"cleaning-app/subscription-service/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SubscriptionService struct {
	repo          SubscriptionRepository
	orderService  *utils.OrderServiceClient
	paymentClient *utils.PaymentServiceClient
}

type OrderService interface {
	// GetOrderByID возвращает OrderResponse и ошибку.
	GetOrderByID(ctx context.Context, orderID, authHeader string) (*utils.OrderResponse, error)

	// CreateOrderFromSubscription принимает и sub, и authHeader, чтобы проксировать JWT дальше.
	CreateOrderFromSubscription(ctx context.Context, sub models.Subscription, authHeader string) error
}

type PaymentServiceClient interface {
	Charge(ctx context.Context, entityType string, entityID string, userID string, authHeader string, amount float64) error
}

type SubscriptionRepository interface {
	Create(ctx context.Context, sub *models.Subscription) error
	Update(ctx context.Context, id primitive.ObjectID, update primitive.M) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Subscription, error)
	GetAll(ctx context.Context) ([]models.Subscription, error)
	GetByClient(ctx context.Context, clientIDHex string) ([]models.Subscription, error)
	FindExpiringOn(ctx context.Context, targetDate time.Time) ([]models.Subscription, error)
	FindExpired(ctx context.Context, before time.Time) ([]models.Subscription, error)
	UpdateStatus(ctx context.Context, id primitive.ObjectID, status models.SubscriptionStatus) error

	// Возвращает все подписки, у которых status = active и EndDate >= now.
	GetActiveSubscriptions(ctx context.Context) ([]models.Subscription, error)
	// Возвращает подписки, у которых NextPlannedDate <= before и status = active.
	FindDue(ctx context.Context, before time.Time) ([]models.Subscription, error)
	UpdateAfterOrder(ctx context.Context, id primitive.ObjectID, nextDate time.Time) error
	SetExpired(ctx context.Context, id primitive.ObjectID) error
}

func NewSubscriptionService(repo SubscriptionRepository, paymentClient *utils.PaymentServiceClient, orderService *utils.OrderServiceClient) *SubscriptionService {
	s := &SubscriptionService{repo: repo}
	s.paymentClient = paymentClient
	s.orderService = orderService
	return s
}

// contains возвращает true, если среди []string есть s.
func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// containsInt возвращает true, если среди []int есть n.
func containsInt(slice []int, n int) bool {
	for _, v := range slice {
		if v == n {
			return true
		}
	}
	return false
}

// NextDates вычисляет все даты (с нулевой частью времени), подходящие под ScheduleSpec,
// в диапазоне [from .. until], сравнивая только «дату» без времени.
// Возвращает список time.Time (каждый — 00:00:00 UTC), на которые нужно создавать заказы.
func (s *SubscriptionService) NextDates(spec models.ScheduleSpec, from time.Time, until time.Time) []time.Time {
	var out []time.Time

	// Установим current в полночь UTC для «from»
	current := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	limit := time.Date(until.Year(), until.Month(), until.Day(), 0, 0, 0, 0, time.UTC)

	for !current.After(limit) {
		// Вычисляем номер недели в месяце: 1..5
		weekOfMonth := ((current.Day() - 1) / 7) + 1
		// Сокращённое название дня недели, например "Mon", "Tue", "Wed"
		weekdayAbbr := current.Weekday().String()[:3]

		switch spec.Frequency {
		case models.Weekly:
			if contains(spec.DaysOfWeek, weekdayAbbr) {
				out = append(out, current)
			}

		case models.BiWeekly, models.TriWeekly, models.Monthly:
			// Для bi/tri/monthly надо совпадение и дня недели, и номера недели в Month
			if contains(spec.DaysOfWeek, weekdayAbbr) && containsInt(spec.WeekNumbers, weekOfMonth) {
				out = append(out, current)
			}
		}

		current = current.AddDate(0, 0, 1) // следующий день
	}

	return out
}

func (s *SubscriptionService) Extend(ctx context.Context, id primitive.ObjectID, extraCleanings int) error {
	sub, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	newEnd := sub.EndDate.AddDate(0, 0, extraCleanings)
	return s.repo.Update(ctx, id, bson.M{"end_date": newEnd})
}

func (s *SubscriptionService) PayForExtension(ctx context.Context, id primitive.ObjectID, extraDays int, authHeader string) error {
	sub, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	for i := 0; i < extraDays; i++ {
		if err := s.paymentClient.Charge(
			ctx,
			"subscription",
			sub.ID.Hex(),     // EntityID — ID подписки
			sub.UserID.Hex(), // UserID
			authHeader,       // JWT
			sub.Price,        // платим ровно sub.Price (например, 80.0)
		); err != nil {
			return fmt.Errorf("payment for extension #%d failed: %w", i+1, err)
		}
	}
	return nil
}

// Create сохраняет новую подписку.
// sub.OrderID, sub.StartDate, sub.EndDate, sub.DaysOfWeek, sub.Price приходят из handler-а.
func (s *SubscriptionService) Create(ctx context.Context, sub *models.Subscription) error {
	now := time.Now().UTC()

	sub.Status = models.StatusActive
	sub.CreatedAt = now
	sub.UpdatedAt = now
	// первый повторный заказ произойдёт в StartDate
	next := sub.StartDate
	sub.NextPlannedDate = &next

	return s.repo.Create(ctx, sub)
}

// ProcessDailyOrders — cron-задача, которой ежедневно проверяем подписки.
// За 3 дня до NextPlannedDate создаём заказ и уведомление, затем смещаем NextPlannedDate.
// Если текущая дата > EndDate — помечаем подписку как expired и спрашиваем о продлении.
// ProcessDailyOrders перебирает «срочные» подписки и создаёт заказы там, где это нужно.
// вызывается из utils/scheduler.go.
func (s *SubscriptionService) ProcessDailyOrders(ctx context.Context) {
	// 1) Вытащить все активные подписки (status=active, end_date >= now)
	subs, err := s.repo.GetActiveSubscriptions(ctx)
	if err != nil {
		fmt.Println("Error fetching active subscriptions:", err)
		return
	}

	now := time.Now().UTC()
	horizon := now.Add(96 * time.Hour)

	for _, sub := range subs {
		// 2) Если NextPlannedDate == nil или уже дальше горизонта — пропустить
		if sub.NextPlannedDate == nil || sub.NextPlannedDate.After(horizon) {
			continue
		}

		// 3) Вычислить все подходящие даты в окне [now..horizon]
		matches := s.NextDates(sub.Schedule, now, horizon)

		// 4) Если sub.NextPlannedDate совпадает с одной из дат — создаём заказ
		shouldCreate := false
		for _, m := range matches {
			if sub.NextPlannedDate.Truncate(24 * time.Hour).Equal(m) {
				if err := s.orderService.CreateOrderFromSubscription(ctx, sub, ""); err != nil {
					fmt.Println("Error creating order:", err)
					// не выходим, a пытаемся лишь один раз для текущей даты
				}
				shouldCreate = true
				break
			}
		}

		// 5) Пересчитать NextPlannedDate (берём первый элемент из futureDates)
		nextWindowStart := horizon.AddDate(0, 0, 1)
		var futureDates []time.Time
		if nextWindowStart.Before(sub.EndDate) || nextWindowStart.Equal(sub.EndDate) {
			futureDates = s.NextDates(sub.Schedule, nextWindowStart, sub.EndDate)
		}
		update := bson.M{}
		if len(futureDates) > 0 {
			next := futureDates[0]
			update["next_planned_date"] = next
		} else {
			// Больше нет будущих дат — подписку нужно пометить expired
			update["next_planned_date"] = nil
			update["status"] = models.StatusExpired
		}

		// Сохраняем новое NextPlannedDate и, возможно, новый статус
		if err := s.repo.Update(ctx, sub.ID, update); err != nil {
			fmt.Println("Error updating subscription:", err)
		}
		_ = shouldCreate // только чтобы не ругался линтер; флаг можно использовать для логирования
	}
}

// Update даёт возможность администратору менять параметры подписки:
// новые end_date, days_of_week, price и автоматически обновляет updated_at.
func (s *SubscriptionService) Update(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	update["updated_at"] = time.Now().UTC()
	return s.repo.Update(ctx, id, update)
}

// Cancel снимает подписку с активности.
func (s *SubscriptionService) Cancel(ctx context.Context, id primitive.ObjectID) error {
	return s.Update(ctx, id, bson.M{"status": models.StatusCanceled})
}

// GetAll используется, например, для админки, чтобы вывести все подписки.
func (s *SubscriptionService) GetAll(ctx context.Context) ([]models.Subscription, error) {
	return s.repo.GetAll(ctx)
}

// GetByClient возвращает подписки конкретного клиента.
func (s *SubscriptionService) GetByClient(ctx context.Context, clientIDHex string) ([]models.Subscription, error) {
	return s.repo.GetByClient(ctx, clientIDHex)
}

// FindExpiringOn возвращает подписки, у которых EndDate == targetDate.
func (s *SubscriptionService) FindExpiringOn(ctx context.Context, targetDate time.Time) ([]models.Subscription, error) {
	return s.repo.FindExpiringOn(ctx, targetDate)
}

// FindExpired возвращает подписки, у которых EndDate < before.
func (s *SubscriptionService) FindExpired(ctx context.Context, before time.Time) ([]models.Subscription, error) {
	return s.repo.FindExpired(ctx, before)
}

// UpdateStatus обновляет статус подписки (например, ставит StatusExpired).
func (s *SubscriptionService) UpdateStatus(ctx context.Context, id primitive.ObjectID, status models.SubscriptionStatus) error {
	return s.repo.Update(ctx, id, bson.M{"status": status})
}

func (s *SubscriptionService) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Subscription, error) {
	return s.repo.GetByID(ctx, id)
}
