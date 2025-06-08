package services

import (
	"context"
	"log"
	"time"

	"cleaning-app/order-service/internal/config"
	"cleaning-app/order-service/internal/models"
	"cleaning-app/order-service/internal/utils"
	"go.mongodb.org/mongo-driver/bson"
)

// CronJobService отвечает за периодические задачи: напоминания и запросы отзывов
type CronJobService struct {
	OrderRepo OrderRepository
	Cfg       *config.Config
}

func NewCronJobService(repo OrderRepository, cfg *config.Config) *CronJobService {
	return &CronJobService{
		OrderRepo: repo,
		Cfg:       cfg,
	}
}

func (s *CronJobService) Start(ctx context.Context) {
	go s.startReminderJob(ctx)
	go s.startReviewRequestJob(ctx)
}

func (s *CronJobService) startReminderJob(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	for {
		select {
		case <-ticker.C:
			s.sendReminderNotifications(ctx)
		case <-ctx.Done():
			log.Println("[CRON] Stopping reminder job")
			ticker.Stop()
			return
		}
	}
}

func (s *CronJobService) startReviewRequestJob(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	for {
		select {
		case <-ticker.C:
			s.sendReviewRequests(ctx)
		case <-ctx.Done():
			log.Println("[CRON] Stopping review request job")
			ticker.Stop()
			return
		}
	}
}

func (s *CronJobService) sendReminderNotifications(ctx context.Context) {
	// Напоминать за 24 часа до даты заказа
	from := time.Now().Add(24 * time.Hour).Truncate(time.Hour)
	to := from.Add(time.Hour)

	orders, err := s.OrderRepo.Filter(ctx, bson.M{
		"date": bson.M{
			"$gte": from,
			"$lt":  to,
		},
		"status": models.StatusAssigned,
	})
	if err != nil {
		log.Println("[CRON] Failed to fetch upcoming orders:", err)
		return
	}

	for _, order := range orders {
		clientID := order.ClientID
		reminderTime := order.Date.Format(time.RFC3339)
		go func(clientID, orderID, when string) {
			_ = utils.SendNotificationEvent(context.Background(), utils.NotificationEvent{
				UserID:    clientID,
				Role:      "client",
				Type:      "reminder",
				ExtraData: map[string]string{"order_id": orderID, "time": when},
			})
		}(clientID, order.ID.Hex(), reminderTime)
	}
}

func (s *CronJobService) sendReviewRequests(ctx context.Context) {
	// Запрашивать отзыв через час после завершения заказа
	from := time.Now().Add(-1 * time.Hour).Truncate(time.Hour)
	to := from.Add(10 * time.Minute)

	orders, err := s.OrderRepo.Filter(ctx, bson.M{
		"status": models.StatusCompleted,
		"updated_at": bson.M{
			"$gte": from,
			"$lt":  to,
		},
	})
	if err != nil {
		log.Println("[CRON] Failed to fetch completed orders:", err)
		return
	}

	for _, order := range orders {
		clientID := order.ClientID
		go func(clientID, orderID string) {
			_ = utils.SendNotificationEvent(context.Background(), utils.NotificationEvent{
				UserID:    clientID,
				Role:      "client",
				Type:      "review_request",
				ExtraData: map[string]string{"order_id": orderID},
			})
		}(clientID, order.ID.Hex())
	}
}
