package services

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"cleaning-app/user-management-service/internal/models"
)

type UserService struct {
	repo UserRepository
}

type UserRepository interface {
	Create(ctx context.Context, u *models.User) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.User, error)
	GetAll(ctx context.Context, role models.Role) ([]models.User, error)
	SetBanStatus(ctx context.Context, id primitive.ObjectID, banned bool) error
	UpdateRole(ctx context.Context, id primitive.ObjectID, role models.Role) error
}

func NewUserService(repo UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) CreateUser(ctx context.Context, user *models.User) error {
	return s.repo.Create(ctx, user)
}

func (s *UserService) GetUserByID(ctx context.Context, id primitive.ObjectID) (*models.User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *UserService) GetAllUsers(ctx context.Context, role models.Role) ([]models.User, error) {
	return s.repo.GetAll(ctx, role)
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
