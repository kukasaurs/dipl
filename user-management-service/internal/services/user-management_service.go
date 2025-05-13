package services

import (
	"cleaning-app/user-management-service/internal/models"
	"cleaning-app/user-management-service/internal/repository"
	"context"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UserService struct {
	repo *repository.UserRepository
}

func NewUserService(repo *repository.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) CreateUser(ctx context.Context, user *models.User) error {
	return s.repo.Create(ctx, user)
}

func (s *UserService) GetUserByID(ctx context.Context, id primitive.ObjectID) (*models.User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *UserService) GetAllUsers(ctx context.Context) ([]models.User, error) {
	return s.repo.GetAll(ctx)
}

func (s *UserService) ChangeUserRole(ctx context.Context, id primitive.ObjectID, newRole models.Role) error {
	return s.repo.UpdateRole(ctx, id, newRole)
}

func (s *UserService) BlockUser(ctx context.Context, id primitive.ObjectID) error {
	return s.repo.SetBanStatus(ctx, id, true)
}

func (s *UserService) UnblockUser(ctx context.Context, id primitive.ObjectID) error {
	return s.repo.SetBanStatus(ctx, id, false)
}
