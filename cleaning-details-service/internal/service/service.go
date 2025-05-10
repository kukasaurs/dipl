package services

import (
	"cleaning-app/cleaning-details-service/internal/models"
	"context"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CleaningServiceRepository interface {
	GetAllServices(context.Context) ([]models.CleaningService, error)
	GetActiveServices(context.Context) ([]models.CleaningService, error)
	CreateService(context.Context, *models.CleaningService) error
	UpdateService(context.Context, *models.CleaningService) error
	DeleteService(context.Context, primitive.ObjectID) error
	UpdateServiceStatus(context.Context, primitive.ObjectID, bool) error
	GetServicesByIDs(ctx context.Context, ids []primitive.ObjectID) ([]models.CleaningService, error)
}

type CleaningService struct {
	repo CleaningServiceRepository
}

func NewCleaningService(repo CleaningServiceRepository) *CleaningService {
	return &CleaningService{
		repo: repo,
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
	services, err := s.repo.GetActiveServices(ctx)
	if err != nil {
		return nil, err
	}

	return services, nil
}

func (s *CleaningService) CreateService(ctx context.Context, service *models.CleaningService) error {
	if err := service.Validate(); err != nil {
		return err
	}

	return s.repo.CreateService(ctx, service)
}

func (s *CleaningService) UpdateService(ctx context.Context, service *models.CleaningService) error {
	if service.ID.IsZero() {
		return models.ErrInvalidID
	}

	if err := service.Validate(); err != nil {
		return err
	}

	return s.repo.UpdateService(ctx, service)
}

func (s *CleaningService) DeleteService(ctx context.Context, id string) error {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return models.ErrInvalidID
	}

	return s.repo.DeleteService(ctx, objID)
}

func (s *CleaningService) UpdateServiceStatus(ctx context.Context, id string, isActive bool) error {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return models.ErrInvalidID
	}

	return s.repo.UpdateServiceStatus(ctx, objID, isActive)
}
func (s *CleaningService) GetServicesByIDs(ctx context.Context, ids []primitive.ObjectID) ([]models.CleaningService, error) {
	return s.repo.GetServicesByIDs(ctx, ids)
}
