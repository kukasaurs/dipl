package services

import (
	"cleaning-app/review-media-service/internal/config"
	"cleaning-app/review-media-service/internal/models"
	"cleaning-app/review-media-service/internal/repository"
	"cleaning-app/review-media-service/internal/utils"
	"context"
	"time"
)

type ReviewService struct {
	repo *repository.ReviewRepository
	cfg  *config.Config
}

func NewReviewService(r *repository.ReviewRepository, cfg *config.Config) *ReviewService {
	return &ReviewService{repo: r, cfg: cfg}
}

func (s *ReviewService) CreateReview(ctx context.Context, review *models.Review) error {
	return s.repo.Create(ctx, review)
}

func (s *ReviewService) GetReviewsByTarget(ctx context.Context, targetID string) ([]models.Review, error) {
	return s.repo.GetByTargetID(ctx, targetID)
}

func (s *ReviewService) ScheduleReviewRequest(userID string, orderID string) {
	go func() {
		time.Sleep(1 * time.Hour)
		req := utils.NotificationRequest{
			UserID:       userID,
			Role:         "client",
			Title:        "Как вам уборка?",
			Message:      "Оцените работу клинера. Нам важно ваше мнение!",
			Type:         "review_request",
			DeliveryType: "push",
			Metadata: map[string]string{
				"order_id": orderID,
			},
		}
		_ = utils.SendNotification(context.Background(), s.cfg, req)
	}()
}
func (s *ReviewService) ReviewExists(ctx context.Context, orderID, reviewerID string) (bool, error) {
	return s.repo.ExistsByOrderAndReviewer(ctx, orderID, reviewerID)
}

type ReviewStat struct {
	TargetID string  `json:"target_id"`
	Count    int     `json:"count"`
	Average  float64 `json:"average_rating"`
}

func (s *ReviewService) GetReviewStatistics(ctx context.Context) ([]repository.ReviewStat, error) {
	return s.repo.AggregateStatistics(ctx)
}
