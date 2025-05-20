package services

import (
	"cleaning-app/subscription-service/internal/models"
	"cleaning-app/subscription-service/internal/repository"
	"cleaning-app/subscription-service/internal/utils"
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SubscriptionService struct {
	repo         *repository.SubscriptionRepository
	orderService struct {
		CreateOrderFromSubscription func(ctx context.Context, sub models.Subscription) error
		Payment                     *utils.PaymentServiceClient
	}
}

func NewSubscriptionService(
	repo *repository.SubscriptionRepository,
	orderClient *utils.OrderServiceClient,
	paymentClient *utils.PaymentServiceClient,
) *SubscriptionService {
	s := &SubscriptionService{repo: repo}
	s.orderService.CreateOrderFromSubscription = orderClient.CreateOrderFromSubscription
	s.orderService.Payment = paymentClient
	return s
}

func (s *SubscriptionService) Create(ctx context.Context, sub *models.Subscription) error {
	sub.Status = models.StatusActive
	sub.RemainingCleanings = sub.TotalCleanings
	nextDate := utils.NextValidDate(sub.DaysOfWeek, time.Now())
	sub.NextPlannedDate = &nextDate
	return s.repo.Create(ctx, sub)
}

func (s *SubscriptionService) Extend(ctx context.Context, id primitive.ObjectID, days int) error {
	return s.repo.Update(ctx, id, map[string]interface{}{
		"remaining_cleanings": bson.M{"$inc": days},
		"status":              models.StatusActive,
	})
}

func (s *SubscriptionService) GetByClient(ctx context.Context, clientID string) ([]models.Subscription, error) {
	return s.repo.GetByClient(ctx, clientID)
}

func (s *SubscriptionService) ProcessDailyOrders(ctx context.Context) {
	subs, err := s.repo.GetActiveSubscriptions(ctx)
	if err != nil {
		return
	}
	today := time.Now().Weekday().String()

	for _, sub := range subs {
		if !utils.DayMatch(sub.DaysOfWeek, today) {
			continue
		}
		if sub.NextPlannedDate != nil && !utils.IsToday(*sub.NextPlannedDate) {
			continue
		}
		if sub.RemainingCleanings == 0 {
			// За 2 дня до уборки — пытаемся выставить оплату
			twoDaysBefore := time.Now().Add(48 * time.Hour)
			if utils.DayMatch(sub.DaysOfWeek, twoDaysBefore.Weekday().String()) {
				_ = s.orderService.Payment.PayForCleanings(ctx, sub.ClientID, sub.ID.Hex(), 1)
			}
			continue
		}

		err := s.orderService.CreateOrderFromSubscription(ctx, sub)
		if err == nil {
			next := utils.NextValidDate(sub.DaysOfWeek, time.Now().Add(24*time.Hour))
			_ = s.repo.UpdateAfterOrder(ctx, sub.ID, next)

			if sub.RemainingCleanings <= 1 {
				_ = s.repo.SetExpired(ctx, sub.ID)
			}
		}
	}

}
func (s *SubscriptionService) InitPayment(ctx context.Context, sub models.Subscription) error {
	return s.orderService.Payment.PayForCleanings(ctx, sub.ClientID, sub.ID.Hex(), sub.TotalCleanings)
}

func (s *SubscriptionService) PayForExtension(ctx context.Context, id primitive.ObjectID, cleanings int) error {
	sub, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	return s.orderService.Payment.PayForCleanings(ctx, sub.ClientID, sub.ID.Hex(), cleanings)
}
func (s *SubscriptionService) GetAll(ctx context.Context) ([]models.Subscription, error) {
	return s.repo.GetAll(ctx)
}

func (s *SubscriptionService) Update(ctx context.Context, id primitive.ObjectID, update map[string]interface{}) error {
	return s.repo.Update(ctx, id, update)
}

func (s *SubscriptionService) Cancel(ctx context.Context, id primitive.ObjectID) error {
	return s.repo.Update(ctx, id, map[string]interface{}{"status": models.StatusCancelled})
}
func (s *SubscriptionService) FindExpiringOn(ctx context.Context, targetDate time.Time) ([]models.Subscription, error) {
	return s.repo.FindExpiringOn(ctx, targetDate)
}

func (s *SubscriptionService) FindExpired(ctx context.Context, before time.Time) ([]models.Subscription, error) {
	return s.repo.FindExpired(ctx, before)
}

func (s *SubscriptionService) UpdateStatus(ctx context.Context, id primitive.ObjectID, status models.SubscriptionStatus) error {
	return s.repo.Update(ctx, id, map[string]interface{}{"status": status})
}
