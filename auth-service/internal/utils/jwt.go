package utils

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

type JWTUtil struct {
	secret string
}

func NewJWTUtil(secret string) *JWTUtil {
	return &JWTUtil{secret: secret}
}

func (j *JWTUtil) GenerateToken(userID, role string, banned bool, resetRequired bool, averageRating float64) (string, error) {
	expirationTime := time.Now().Add(200 * time.Hour)
	claims := jwt.MapClaims{
		"user_id":        userID,
		"role":           role,
		"banned":         banned,
		"reset_required": resetRequired,
		"exp":            expirationTime.Unix(),
		"iat":            time.Now().Unix(),
		"average_rating": averageRating,
		"jti":            GenerateCode(10),
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
	token, err := j.ValidateToken(tokenString)
	if err != nil || !token.Valid {
		return true
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return true
	}

	jti, _ := claims["jti"].(string)
	if jti == "" {
		return true
	}

	return redis.Exists(ctx, fmt.Sprintf("blacklist:%s", jti))
}
