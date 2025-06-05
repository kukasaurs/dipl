package services

import (
	"cleaning-app/review-media-service/internal/config"
	"cleaning-app/review-media-service/internal/models"
	"cleaning-app/review-media-service/internal/repository"
	"context"
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
