package services

import (
	"cleaning-app/auth-service/internal/models"
	"cleaning-app/auth-service/internal/repository"
	"cleaning-app/auth-service/internal/utils"
	"context"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v4"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
	"log"
	"time"
)

type AuthService struct {
	userRepo *repositories.UserRepository
	jwtUtil  *utils.JWTUtil
	google   *GoogleAuthService
	email    EmailService
	redis    *utils.RedisClient
}

func NewAuthService(userRepo *repositories.UserRepository, jwtUtil *utils.JWTUtil, google *GoogleAuthService, email EmailService, redis *utils.RedisClient) *AuthService {
	return &AuthService{userRepo, jwtUtil, google, email, redis}
}

func (s *AuthService) Register(user *models.User) (string, error) {
	existing, _ := s.userRepo.FindUserByEmail(user.Email)
	if existing != nil {
		return "", errors.New("user already exists")
	}

	// Generate temporary password
	tempPass := utils.GenerateCode(10) // You need to add this function to utils
	hashed, err := bcrypt.GenerateFromPassword([]byte(tempPass), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	newUser := &models.User{
		Email:         user.Email,
		Password:      string(hashed),
		Role:          "user",
		ResetRequired: true,
	}

	createdUser, err := s.userRepo.CreateUser(newUser)
	if err != nil {
		return "", err
	}

	// Send email with temporary password
	if err := s.email.SendVerificationCode(user.Email, tempPass); err != nil {
		// Clean up if email sending fails
		_ = s.userRepo.DeleteUser(createdUser.ID)
		return "", errors.New("failed to send email with temporary password")
	}

	return s.jwtUtil.GenerateToken(createdUser.ID.Hex(), createdUser.Role, true)
}

func (s *AuthService) Login(email, password string) (string, error) {
	user, err := s.userRepo.FindUserByEmail(email)
	if err != nil {
		log.Printf("User not found: %s", email)
		return "", errors.New("invalid credentials")
	}

	if user.Banned {
		log.Printf("User is banned: %s", email)
		return "", errors.New("user is banned")
	}

	err = user.ComparePassword(password)
	if err != nil {
		log.Printf("Password comparison failed for user %s: %v", email, err)
		return "", errors.New("invalid credentials")
	}

	return s.jwtUtil.GenerateToken(user.ID.Hex(), user.Role, user.ResetRequired)
}

func (s *AuthService) GetProfile(userID primitive.ObjectID) (*models.User, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("user_profile:%s", userID.Hex())

	var cachedUser models.User
	err := s.redis.Get(ctx, cacheKey, &cachedUser)
	if err == nil {
		// Профиль найден в кэше
		return &cachedUser, nil
	}

	// Профиль не найден в Redis, грузим из MongoDB
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	// Сохраняем в кэш на 5 минут
	err = s.redis.Set(ctx, cacheKey, user, 5*time.Minute)
	if err != nil {
		// Не фейлим процесс, просто логируем ошибку
		fmt.Printf("Failed to cache user profile: %v\n", err)
	}

	return user, nil
}

func (s *AuthService) UpdateProfile(user *models.User) error {
	existingUser, err := s.userRepo.GetUserByID(user.ID)
	if err != nil {
		return errors.New("user not found")
	}

	existingUser.FirstName = user.FirstName
	existingUser.LastName = user.LastName
	existingUser.Address = user.Address
	existingUser.PhoneNumber = user.PhoneNumber
	existingUser.DateOfBirth = user.DateOfBirth
	existingUser.Gender = user.Gender

	if err := s.userRepo.UpdateUser(existingUser); err != nil {
		return err
	}

	// После обновления профиля - удаляем кэш
	cacheKey := fmt.Sprintf("user_profile:%s", user.ID.Hex())
	_ = s.redis.Delete(context.Background(), cacheKey)

	return nil
}

func (s *AuthService) ChangePassword(userID primitive.ObjectID, oldPassword, newPassword string) error {
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return errors.New("user not found")
	}

	if err := user.ComparePassword(oldPassword); err != nil {
		return errors.New("invalid old password")
	}

	user.Password = newPassword

	if err := user.HashPassword(); err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user.ResetRequired = false

	return s.userRepo.UpdateUser(user)
}

func (s *AuthService) Validate(token string) (*jwt.Token, error) {
	return s.jwtUtil.ValidateToken(token)
}

func (s *AuthService) ResendTemporaryPassword(email string) error {
	user, err := s.userRepo.FindUserByEmail(email)
	if err != nil {
		return errors.New("user not found")
	}

	tempPass := utils.GenerateCode(10)
	hashed, err := bcrypt.GenerateFromPassword([]byte(tempPass), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.Password = string(hashed)
	user.ResetRequired = true

	err = s.userRepo.UpdateUser(user)
	if err != nil {
		return err
	}

	return s.email.SendVerificationCode(email, tempPass)
}

func (s *AuthService) SetInitialPassword(userID primitive.ObjectID, newPassword string) error {
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return errors.New("user not found")
	}

	if !user.ResetRequired {
		return errors.New("this action is only allowed for accounts requiring password reset")
	}

	// Хешируем пароль
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Используем специальный метод для обновления пароля
	return s.userRepo.UpdatePassword(userID, string(hashedPassword), false)
}

// Add this method to AuthService
func (s *AuthService) Logout(tokenString string) error {
	token, err := s.jwtUtil.ValidateToken(tokenString)
	if err != nil || !token.Valid {
		return errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return errors.New("invalid token claims")
	}

	jti, ok := claims["jti"].(string)
	if !ok {
		return errors.New("missing jti in token")
	}

	exp, ok := claims["exp"].(float64)
	if !ok {
		return errors.New("invalid token expiration")
	}

	ttl := time.Until(time.Unix(int64(exp), 0))
	ctx := context.Background()

	return s.redis.Set(ctx, fmt.Sprintf("blacklist:%s", jti), true, ttl)
}
