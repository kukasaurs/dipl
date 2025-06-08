package services

import (
	"context"
	"fmt"
	"time"

	"cleaning-app/subscription-service/internal/models"
	"cleaning-app/subscription-service/internal/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SubscriptionService отвечает за логику подписок, включая создание заказов по расписанию и уведомления.
type SubscriptionService struct {
	repo          SubscriptionRepository
	orderService  *utils.OrderServiceClient
	paymentClient *utils.PaymentServiceClient
}

// Объявляем интерфейсы для зависимостей (order и payment)
type OrderService interface {
	// CreateOrderFromSubscription создаёт заказ на основании подписки
	CreateOrderFromSubscription(ctx context.Context, sub models.Subscription, authHeader string) error
}

type PaymentServiceClient interface {
	// Charge списывает оплату за подписку
	Charge(ctx context.Context, entityType string, entityID string, userID string, authHeader string, amount float64) error
}

// Репозиторий подписок: CRUD + выборки по расписанию
type SubscriptionRepository interface {
	Create(ctx context.Context, sub *models.Subscription) error
	Update(ctx context.Context, id primitive.ObjectID, update bson.M) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Subscription, error)
	GetAll(ctx context.Context) ([]models.Subscription, error)
	GetByClient(ctx context.Context, clientIDHex string) ([]models.Subscription, error)
	FindExpiringOn(ctx context.Context, targetDate time.Time) ([]models.Subscription, error)
	FindExpired(ctx context.Context, before time.Time) ([]models.Subscription, error)
	UpdateStatus(ctx context.Context, id primitive.ObjectID, status models.SubscriptionStatus) error

	// Возвращает все активные подписки (status=active, end_date >= now)
	GetActiveSubscriptions(ctx context.Context) ([]models.Subscription, error)
	// Возвращает все активные подписки, у которых NextPlannedDate <= before
	FindDue(ctx context.Context, before time.Time) ([]models.Subscription, error)
	// После создания заказа обновляет NextPlannedDate
	UpdateAfterOrder(ctx context.Context, id primitive.ObjectID, nextDate time.Time) error
	SetExpired(ctx context.Context, id primitive.ObjectID) error
}

// Конструктор сервиса
func NewSubscriptionService(repo SubscriptionRepository, paymentClient *utils.PaymentServiceClient, orderService *utils.OrderServiceClient) *SubscriptionService {
	return &SubscriptionService{
		repo:          repo,
		orderService:  orderService,
		paymentClient: paymentClient,
	}
}

// NextDates вычисляет даты (без времени) в диапазоне [from..until] по расписанию spec.
func (s *SubscriptionService) NextDates(spec models.ScheduleSpec, from time.Time, until time.Time) []time.Time {
	var out []time.Time
	current := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	limit := time.Date(until.Year(), until.Month(), until.Day(), 0, 0, 0, 0, time.UTC)

	for !current.After(limit) {
		weekOfMonth := ((current.Day() - 1) / 7) + 1
		weekdayAbbr := current.Weekday().String()[:3]

		switch spec.Frequency {
		case models.Weekly:
			if contains(spec.DaysOfWeek, weekdayAbbr) {
				out = append(out, current)
			}
		case models.BiWeekly, models.TriWeekly, models.Monthly:
			if contains(spec.DaysOfWeek, weekdayAbbr) && containsInt(spec.WeekNumbers, weekOfMonth) {
				out = append(out, current)
			}
		}
		current = current.AddDate(0, 0, 1)
	}
	return out
}

// contains проверяет, есть ли строка s в срезе slice.
func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// containsInt проверяет, есть ли число n в срезе slice.
func containsInt(slice []int, n int) bool {
	for _, v := range slice {
		if v == n {
			return true
		}
	}
	return false
}

// Create сохраняет новую подписку с расчётом NextPlannedDate на первый возможный заказ.
func (s *SubscriptionService) Create(ctx context.Context, sub *models.Subscription) error {
	now := time.Now().UTC()
	sub.Status = models.StatusActive
	sub.CreatedAt = now
	sub.UpdatedAt = now

	// первый заказ планируем на StartDate
	next := sub.StartDate
	sub.NextPlannedDate = &next
	return s.repo.Create(ctx, sub)
}

// Extend продлевает EndDate у подписки (без оплаты).
func (s *SubscriptionService) Extend(ctx context.Context, id primitive.ObjectID, extraCleanings int) error {
	sub, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	newEnd := sub.EndDate.AddDate(0, 0, extraCleanings)
	return s.repo.Update(ctx, id, bson.M{"end_date": newEnd})
}

// PayForExtension списывает оплату за дополнительные дни по подписке.
func (s *SubscriptionService) PayForExtension(ctx context.Context, id primitive.ObjectID, extraDays int, authHeader string) error {
	sub, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	for i := 0; i < extraDays; i++ {
		if err := s.paymentClient.Charge(
			ctx,
			"subscription",
			sub.ID.Hex(),
			sub.UserID.Hex(),
			authHeader,
			sub.Price,
		); err != nil {
			return fmt.Errorf("payment for extension #%d failed: %w", i+1, err)
		}
	}
	return nil
}

// Update меняет поля подписки (админ может менять end_date, days_of_week и т. д.).
func (s *SubscriptionService) Update(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	update["updated_at"] = time.Now().UTC()
	return s.repo.Update(ctx, id, update)
}

// Cancel переводит подписку в статус Canceled.
func (s *SubscriptionService) Cancel(ctx context.Context, id primitive.ObjectID) error {
	return s.Update(ctx, id, bson.M{"status": models.StatusCanceled})
}

// GetAll возвращает все подписки (например, для админки).
func (s *SubscriptionService) GetAll(ctx context.Context) ([]models.Subscription, error) {
	return s.repo.GetAll(ctx)
}

// GetByClient возвращает подписки конкретного клиента.
func (s *SubscriptionService) GetByClient(ctx context.Context, clientIDHex string) ([]models.Subscription, error) {
	return s.repo.GetByClient(ctx, clientIDHex)
}

// FindExpiringOn находит подписки, у которых EndDate == targetDate (12:00:00 UTC).
func (s *SubscriptionService) FindExpiringOn(ctx context.Context, targetDate time.Time) ([]models.Subscription, error) {
	return s.repo.FindExpiringOn(ctx, targetDate)
}

// FindExpired возвращает подписки, у которых EndDate < before.
func (s *SubscriptionService) FindExpired(ctx context.Context, before time.Time) ([]models.Subscription, error) {
	return s.repo.FindExpired(ctx, before)
}

// UpdateStatus меняет статус подписки (например, переводит в Expired).
func (s *SubscriptionService) UpdateStatus(ctx context.Context, id primitive.ObjectID, status models.SubscriptionStatus) error {
	return s.repo.Update(ctx, id, bson.M{"status": status, "updated_at": time.Now().UTC()})
}

// GetByID возвращает подписку по ID.
func (s *SubscriptionService) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Subscription, error) {
	return s.repo.GetByID(ctx, id)
}

// ProcessDailyOrders — cron-задача, выполняемая ежедневно.
// 1) Уведомляет о том, что подписка истекает через 3 дня.
// 2) Создаёт заказы по расписанию (NextPlannedDate ≤ horizon).
// 3) Перекладывает NextPlannedDate на следующий «цикл» или переводит подписку в Expired.
func (s *SubscriptionService) ProcessDailyOrders(ctx context.Context) {
	now := time.Now().UTC()

	// ----- 1. Уведомляем за 3 дня до истечения -----
	target := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 3)
	expiringSubs, err := s.repo.FindExpiringOn(ctx, target)
	if err != nil {
		fmt.Println("Error fetching expiring subscriptions:", err)
	} else {
		for _, sub := range expiringSubs {
			go func(su models.Subscription) {
				_ = utils.SendNotificationEvent(context.Background(), utils.NotificationEvent{
					UserID:    su.UserID.Hex(),
					Role:      "client",
					Type:      "subscription_expiring",
					ExtraData: map[string]string{"subscription_id": su.ID.Hex(), "end_date": su.EndDate.Format("2006-01-02")},
				})
			}(sub)
		}
	}

	// ----- 2. Создаём заказы по расписанию -----
	activeSubs, err := s.repo.GetActiveSubscriptions(ctx)
	if err != nil {
		fmt.Println("Error fetching active subscriptions:", err)
		return
	}

	horizon := now.Add(96 * time.Hour) // 96h = 4 дня вперёд

	for _, sub := range activeSubs {
		// Пропускаем, если NextPlannedDate nil или дальше горизонта
		if sub.NextPlannedDate == nil || sub.NextPlannedDate.After(horizon) {
			continue
		}

		// Находим все даты в окне [now..horizon]
		matches := s.NextDates(sub.Schedule, now, horizon)
		created := false

		for _, m := range matches {
			if sub.NextPlannedDate.Truncate(24 * time.Hour).Equal(m) {
				// Создаём заказ из подписки
				if err := s.orderService.CreateOrderFromSubscription(ctx, sub, ""); err != nil {
					fmt.Println("Error creating order:", err)
				}
				created = true
				break
			}
		}

		// Пересчитываем NextPlannedDate на следующий цикл
		nextWindowStart := horizon.AddDate(0, 0, 1)
		var futureDates []time.Time
		if !nextWindowStart.After(sub.EndDate) {
			futureDates = s.NextDates(sub.Schedule, nextWindowStart, sub.EndDate)
		}
		update := bson.M{}
		if len(futureDates) > 0 {
			update["next_planned_date"] = futureDates[0]
		} else {
			// Больше нет дат → подписку помечаем Expired
			update["next_planned_date"] = nil
			update["status"] = models.StatusExpired
		}

		if err := s.repo.Update(ctx, sub.ID, update); err != nil {
			fmt.Println("Error updating subscription:", err)
		}

		// (не обязательно) можно логировать создание
		_ = created
	}
}
