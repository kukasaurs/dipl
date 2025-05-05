package services

import (
	"cleaning-app/auth-service/internal/models"
	"context"
	"google.golang.org/api/idtoken"
)

type GoogleAuthService struct {
	ClientID string
}

func NewGoogleAuthService(clientID string) *GoogleAuthService {
	return &GoogleAuthService{ClientID: clientID}
}

func (g *GoogleAuthService) VerifyGoogleToken(idToken string) (*idtoken.Payload, error) {
	ctx := context.Background()
	payload, err := idtoken.Validate(ctx, idToken, g.ClientID)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func (s *AuthService) GoogleLogin(idToken string) (string, error) {
	payload, err := s.google.VerifyGoogleToken(idToken)
	if err != nil {
		return "", err
	}
	email := payload.Claims["email"].(string)
	user, err := s.userRepo.FindUserByEmail(email)
	if err != nil {
		user := &models.User{
			Email:         email,
			Role:          "user",
			Banned:        false,
			ResetRequired: false,
		}

		user, err = s.userRepo.CreateUser(user)
		if err != nil {
			return "", err
		}
	}
	return s.jwtUtil.GenerateToken(user.ID.Hex(), user.Role, user.ResetRequired)
}
