package services

import (
	"cleaning-app/order-service/internal/config"
	"cleaning-app/order-service/internal/utils"
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"log"
	"time"
)

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
	from := time.Now().Add(24 * time.Hour).Truncate(time.Hour)
	to := from.Add(time.Hour)

	orders, err := s.OrderRepo.Filter(ctx, bson.M{
		"date": bson.M{
			"$gte": from,
			"$lt":  to,
		},
		"status": "assigned",
	})
	if err != nil {
		log.Println("Failed to fetch upcoming orders:", err)
		return
	}

	for _, order := range orders {
		err := utils.SendNotification(ctx, s.Cfg, utils.NotificationRequest{
			UserID:       order.ClientID,
			Role:         "client",
			Title:        "Завтра уборка",
			Message:      "Напоминаем: завтра в " + order.Date.Format("15:04") + " состоится ваша уборка.",
			Type:         "reminder",
			DeliveryType: "push",
		})
		if err != nil {
			log.Println("Failed to send reminder notification:", err)
		}
	}
}
func (s *CronJobService) sendReviewRequests(ctx context.Context) {
	from := time.Now().Add(-1 * time.Hour).Truncate(time.Hour)
	to := from.Add(10 * time.Minute)

	orders, err := s.OrderRepo.Filter(ctx, bson.M{
		"status": "completed",
		"updated_at": bson.M{
			"$gte": from,
			"$lt":  to,
		},
	})
	if err != nil {
		log.Println("Failed to fetch completed orders:", err)
		return
	}

	for _, order := range orders {
		err := utils.SendNotification(ctx, s.Cfg, utils.NotificationRequest{
			UserID:       order.ClientID,
			Role:         "client",
			Title:        "Как вам уборка?",
			Message:      "Оцените работу клинера. Нам важно ваше мнение!",
			Type:         "review_request",
			DeliveryType: "push",
		})
		if err != nil {
			log.Println("Failed to send review notification:", err)
		}
	}
}
