package services

import (
	"context"
	"fmt"
	"time"

	"cleaning-app/subscription-service/internal/models"
	"cleaning-app/subscription-service/internal/repository"
	"cleaning-app/subscription-service/internal/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SubscriptionService struct {
	repo         *repository.SubscriptionRepository
	orderService struct {
		// Создаёт новый заказ на основании существующей подписки
		CreateOrderFromSubscription func(ctx context.Context, sub models.Subscription) error
		// Отправляет уведомление пользователю (push/email/SMS)
		Notify func(ctx context.Context, userID, message string) error
	}
	paymentClient *utils.PaymentServiceClient
}

func NewSubscriptionService(
	repo *repository.SubscriptionRepository,
	orderClient *utils.OrderServiceClient,
	notifier *utils.NotificationServiceClient,
	paymentClient *utils.PaymentServiceClient,
) *SubscriptionService {
	s := &SubscriptionService{repo: repo}
	s.orderService.CreateOrderFromSubscription = orderClient.CreateOrderFromSubscription
	s.orderService.Notify = notifier.SendNotification
	s.paymentClient = paymentClient
	return s
}

func (s *SubscriptionService) Extend(ctx context.Context, id primitive.ObjectID, extraCleanings int) error {
	// здесь вы решаете, что для вас «продление»:
	// либо $inc на какой-то счётчик, либо сдвиг EndDate
	// пример: увеличить EndDate на extraCleanings дней
	sub, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	newEnd := sub.EndDate.AddDate(0, 0, extraCleanings)
	return s.repo.Update(ctx, id, bson.M{"end_date": newEnd})
}

func (s *SubscriptionService) PayForExtension(ctx context.Context, id primitive.ObjectID, extraDays int) error {
	sub, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	// amount – целочисленная сумма, как требует ваш мок
	amount := int64(extraDays)
	return s.paymentClient.ChargeSubscription(ctx, sub.OrderID.Hex(), sub.UserID.Hex(), amount)
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
func (s *SubscriptionService) ProcessDailyOrders(ctx context.Context) {
	subs, err := s.repo.GetActiveSubscriptions(ctx)
	if err != nil {
		return
	}
	now := time.Now().UTC()

	for _, sub := range subs {
		if sub.NextPlannedDate == nil {
			continue
		}
		delta := sub.NextPlannedDate.Sub(now)
		// если до следующей уборки осталось от 72 до 96 часов
		if delta.Hours() >= 72 && delta.Hours() <= 96 {
			// 1) создаём заказ
			if err := s.orderService.CreateOrderFromSubscription(ctx, sub); err == nil {
				// 2) уведомляем пользователя
				msg := fmt.Sprintf("Через 3 дня будет уборка по подписке %s", sub.ID.Hex())
				s.orderService.Notify(ctx, sub.UserID.Hex(), msg)
				// 3) рассчитываем новый next_planned_date
				last := *sub.NextPlannedDate
				next := utils.NextValidDate(sub.DaysOfWeek, last.Add(24*time.Hour))
				s.repo.UpdateAfterOrder(ctx, sub.ID, next)
			}
		}
	}

	// Проверить подписки, срок которых истёк
	expired, _ := s.repo.FindExpired(ctx, now)
	for _, sub := range expired {
		// пометить expired
		s.repo.SetExpired(ctx, sub.ID)
		// и уведомить пользователя
		msg := fmt.Sprintf("Подписка %s завершена. Хотите продлить или отменить?", sub.ID.Hex())
		s.orderService.Notify(ctx, sub.UserID.Hex(), msg)
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
