package utils

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v4"
	//"github.com/redis/go-redis/v9"
)

type JWTUtil struct {
	secret string
}

func NewJWTUtil(secret string) *JWTUtil {
	return &JWTUtil{secret: secret}
}

func (j *JWTUtil) GenerateToken(userID, role string, resetRequired bool) (string, error) {
	expirationTime := time.Now().Add(72 * time.Hour)
	claims := jwt.MapClaims{
		"user_id":        userID,
		"role":           role,
		"reset_required": resetRequired,
		"exp":            expirationTime.Unix(),
		"iat":            time.Now().Unix(),
		"jti":            GenerateCode(10), // Добавляем уникальный идентификатор токена
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.secret))
}

func (j *JWTUtil) ValidateToken(tokenString string) (*jwt.Token, error) {
	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unauthorized")
		}
		return []byte(j.secret), nil
	})
}

var validate = validator.New()

func ValidateStruct(s interface{}) error {
	return validate.Struct(s)
}
func GenerateCode(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func (j *JWTUtil) IsTokenBlacklisted(ctx context.Context, tokenString string, redis *RedisClient) bool {
	var blacklisted bool
	err := redis.Get(ctx, fmt.Sprintf("blacklist:%s", tokenString), &blacklisted)
	return err == nil && blacklisted
}
