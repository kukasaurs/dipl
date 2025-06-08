package services

import (
	"cleaning-app/auth-service/internal/config"
	"cleaning-app/auth-service/internal/models"
	"cleaning-app/auth-service/internal/utils"
	"context"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
	"log"
	"reflect"
	"time"
)

type AuthService struct {
	userRepo UserRepository
	jwtUtil  *utils.JWTUtil
	email    EmailService
	redis    *utils.RedisClient
	cfg      *config.Config
}

type UserRepository interface {
	CreateUser(user *models.User) (*models.User, error)
	FindUserByEmail(email string) (*models.User, error)
	GetUserByID(userID primitive.ObjectID) (*models.User, error)
	UpdateUser(user *models.User) error
	UpdateUserFields(userID primitive.ObjectID, fields bson.M) error
	UpdatePassword(userID primitive.ObjectID, hashedPassword string, resetRequired bool) error
	DeleteUser(userID primitive.ObjectID) error
	GetByRole(role string) ([]*models.User, error)
	CountUsers(ctx context.Context) (int64, error)
	GetRating(userID primitive.ObjectID) (float64, error)
	AddRating(ctx context.Context, cleanerID string, rating int) error
}

func NewAuthService(userRepo UserRepository, jwtUtil *utils.JWTUtil, email EmailService, redis *utils.RedisClient, config *config.Config) *AuthService {
	return &AuthService{userRepo, jwtUtil, email, redis, config}
}

func (s *AuthService) Register(user *models.User) (string, error) {
	existing, _ := s.userRepo.FindUserByEmail(user.Email)
	if existing != nil {
		return "", errors.New("user already exists")
	}

	tempPass := utils.GenerateCode(10)
	hashed, err := bcrypt.GenerateFromPassword([]byte(tempPass), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	newUser := &models.User{
		FirstName:     user.FirstName,
		LastName:      user.LastName,
		Email:         user.Email,
		Password:      string(hashed),
		Role:          "user",
		ResetRequired: true,
	}

	createdUser, err := s.userRepo.CreateUser(newUser)
	if err != nil {
		return "", err
	}

	if err := s.email.SendVerificationCode(user.Email, tempPass); err != nil {
		_ = s.userRepo.DeleteUser(createdUser.ID)
		return "", errors.New("failed to send email with temporary password")
	}

	return s.jwtUtil.GenerateToken(createdUser.ID.Hex(), createdUser.Role, false, createdUser.ResetRequired, 0)
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

	return s.jwtUtil.GenerateToken(user.ID.Hex(), user.Role, false, user.ResetRequired, user.AverageRating)
}

func (s *AuthService) GetProfile(userID primitive.ObjectID) (*models.User, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("user_profile:%s", userID.Hex())

	var cachedUser models.User
	err := s.redis.Get(ctx, cacheKey, &cachedUser)
	if err == nil {
		return &cachedUser, nil
	}

	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	err = s.redis.Set(ctx, cacheKey, user, 5*time.Minute)
	if err != nil {
		fmt.Printf("Failed to cache user profile: %v\n", err)
	}

	return user, nil
}

func (s *AuthService) UpdateProfile(userID primitive.ObjectID, req interface{}) error {
	updateFields := bson.M{}

	if r, ok := req.(map[string]interface{}); ok {
		for k, v := range r {
			if v != nil {
				updateFields[k] = v
			}
		}
	} else {
		val := reflect.ValueOf(req)
		typ := reflect.TypeOf(req)
		for i := 0; i < val.NumField(); i++ {
			field := val.Field(i)
			if !field.IsNil() {
				jsonTag := typ.Field(i).Tag.Get("json")
				updateFields[jsonTag] = field.Elem().Interface()
			}
		}
	}

	if len(updateFields) == 0 {
		return nil
	}

	if err := s.userRepo.UpdateUserFields(userID, updateFields); err != nil {
		return err
	}

	cacheKey := fmt.Sprintf("user_profile:%s", userID.Hex())
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

func (s *AuthService) SetInitialPassword(userID primitive.ObjectID, tempPassword, newPassword string) error {

	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return errors.New("user not found")
	}

	if err := user.ComparePassword(tempPassword); err != nil {
		return errors.New("invalid temporary password")
	}

	if !user.ResetRequired {
		return errors.New("this action is only allowed for accounts requiring password reset")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	return s.userRepo.UpdatePassword(userID, string(hashedPassword), false)
}

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

func (s *AuthService) GetByRole(role string) ([]*models.User, error) {
	return s.userRepo.GetByRole(role)
}

func (s *AuthService) GetTotalUsers(ctx context.Context) (int64, error) {
	return s.userRepo.CountUsers(ctx)
}

func (s *AuthService) AddBulkRatings(ctx context.Context, clientID string, cleanerIDs []string, rating int, comment string) error {
	log.Printf("[DEBUG auth] AddBulkRatings called for client %s, cleanerIDs=%v, rating=%d", clientID, cleanerIDs, rating)
	for _, cid := range cleanerIDs {
		log.Printf("[DEBUG auth] about to AddRating for cleaner %s", cid)
		if err := s.userRepo.AddRating(ctx, cid, rating); err != nil {
			return fmt.Errorf("save rating for cleaner %s: %w", cid, err)
		}
		log.Printf("[DEBUG auth] successfully added rating for cleaner %s", cid)
	}
	return nil
}

func (s *AuthService) GetRating(userID primitive.ObjectID) (float64, error) {
	return s.userRepo.GetRating(userID)

}
