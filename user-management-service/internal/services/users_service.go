package services

import (
	"context"
	"fmt"

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
	AddXP(ctx context.Context, id primitive.ObjectID, xp int) error
	UpdateLevel(ctx context.Context, id primitive.ObjectID, newLevel int) error
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

// ─── AddXPToUser ───
func (s *UserService) AddXPToUser(ctx context.Context, id primitive.ObjectID, xp int) (*models.GamificationStatus, error) {
	// 1. Вычитываем текущее состояние пользователя
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("could not fetch user: %w", err)
	}

	// 2. Новое суммарное XP
	newXPTotal := user.XPTotal + xp

	// 3. Вычисляем новый уровень
	newLevel, xpToNext := models.CalculateLevel(newXPTotal)

	// 4. Обновляем в БД: xp_total и, при необходимости, current_level
	// Сначала обновим xp_total
	if err := s.repo.AddXP(ctx, id, xp); err != nil {
		return nil, fmt.Errorf("could not add xp: %w", err)
	}
	// Затем, если уровень изменился, обновляем current_level
	if newLevel != user.CurrentLevel {
		if err := s.repo.UpdateLevel(ctx, id, newLevel); err != nil {
			return nil, fmt.Errorf("could not update level: %w", err)
		}
	}

	// 5. Вернём свежую информацию
	return &models.GamificationStatus{
		UserID:        id,
		XPTotal:       newXPTotal,
		CurrentLevel:  newLevel,
		XPToNextLevel: xpToNext,
	}, nil
}

// ─── GetGamificationStatus ───
func (s *UserService) GetGamificationStatus(ctx context.Context, id primitive.ObjectID) (*models.GamificationStatus, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("could not fetch user: %w", err)
	}
	currentLevel, xpToNext := models.CalculateLevel(user.XPTotal)
	return &models.GamificationStatus{
		UserID:        id,
		XPTotal:       user.XPTotal,
		CurrentLevel:  currentLevel,
		XPToNextLevel: xpToNext,
	}, nil
}
