package services

import (
	"cleaning-app/cleaning-details-service/internal/models"
	"cleaning-app/cleaning-details-service/utils"
	"context"
	"encoding/json"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CleaningServiceRepository interface {
	GetAllServices(context.Context) ([]models.CleaningService, error)
	GetActiveServices(ctx context.Context) ([]models.CleaningService, error)
	CreateService(context.Context, *models.CleaningService) error
	UpdateService(context.Context, *models.CleaningService) error
	DeleteService(context.Context, primitive.ObjectID) error
	UpdateServiceStatus(context.Context, primitive.ObjectID, bool) error
	GetServicesByIDs(ctx context.Context, ids []primitive.ObjectID) ([]models.CleaningService, error)
}

type CleaningService struct {
	repo        CleaningServiceRepository
	redisClient *redis.Client
}

func NewCleaningService(repo CleaningServiceRepository, redisClient *redis.Client) *CleaningService {
	return &CleaningService{
		repo:        repo,
		redisClient: redisClient,
	}
}

func (s *CleaningService) GetAllServices(ctx context.Context) ([]models.CleaningService, error) {
	services, err := s.repo.GetAllServices(ctx)
	if err != nil {
		return nil, err
	}

	return services, nil
}

func (s *CleaningService) GetActiveServices(ctx context.Context) ([]models.CleaningService, error) {
	cacheKey := "active_services"

	// Try to get from Redis
	cached, err := utils.GetFromCache(ctx, s.redisClient, cacheKey)
	if err == nil && cached != "" {
		var services []models.CleaningService
		if err := json.Unmarshal([]byte(cached), &services); err == nil {
			return services, nil
		}
	}

	// Fetch from DB
	services, err := s.repo.GetActiveServices(ctx)
	if err != nil {
		return nil, err
	}

	// Cache result
	data, _ := json.Marshal(services)
	utils.SetToCache(ctx, s.redisClient, cacheKey, string(data), utils.RedisCacheDuration)

	return services, nil
}

func (s *CleaningService) CreateService(ctx context.Context, service *models.CleaningService) error {
	if err := service.Validate(); err != nil {
		return err
	}
	err := s.repo.CreateService(ctx, service)
	if err == nil {
		_ = utils.DeleteFromCache(ctx, s.redisClient, "active_services")
	}
	return err
}

func (s *CleaningService) UpdateService(ctx context.Context, service *models.CleaningService) error {
	if service.ID.IsZero() {
		return models.ErrInvalidID
	}
	if err := service.Validate(); err != nil {
		return err
	}
	err := s.repo.UpdateService(ctx, service)
	if err == nil {
		_ = utils.DeleteFromCache(ctx, s.redisClient, "active_services")
	}
	return err
}

func (s *CleaningService) DeleteService(ctx context.Context, id string) error {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return models.ErrInvalidID
	}
	err = s.repo.DeleteService(ctx, objID)
	if err == nil {
		_ = utils.DeleteFromCache(ctx, s.redisClient, "active_services")
	}
	return err
}

func (s *CleaningService) UpdateServiceStatus(ctx context.Context, id string, isActive bool) error {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return models.ErrInvalidID
	}
	err = s.repo.UpdateServiceStatus(ctx, objID, isActive)
	if err == nil {
		_ = utils.DeleteFromCache(ctx, s.redisClient, "active_services")
	}
	return err
}
func (s *CleaningService) GetServicesByIDs(ctx context.Context, ids []primitive.ObjectID) ([]models.CleaningService, error) {
	return s.repo.GetServicesByIDs(ctx, ids)
}
