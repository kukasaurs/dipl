package handlers

import (
	"cleaning-app/auth-service/internal/models"
	"cleaning-app/auth-service/internal/services"
	"cleaning-app/auth-service/internal/utils"
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
	"strings"
	"time"
)

type AuthHandler struct {
	authService AuthService
}

type AuthService interface {
	Register(user *models.User) (string, error)
	Login(email, password string) (string, error)
	GetProfile(userID primitive.ObjectID) (*models.User, error)
	UpdateProfile(userID primitive.ObjectID, req interface{}) error
	ChangePassword(userID primitive.ObjectID, oldPassword, newPassword string) error
	Validate(token string) (*jwt.Token, error)
	SetInitialPassword(userID primitive.ObjectID, tempPassword, newPassword string) error
	Logout(tokenString string) error
	GetByRole(role string) ([]*models.User, error)
	GetTotalUsers(ctx context.Context) (int64, error)
	AddRating(userID primitive.ObjectID, rating int) error
	GetRating(userID primitive.ObjectID) (float64, error)
	ResendTemporaryPassword(email string) error
	GoogleLogin(idToken string) (string, error)
}

func NewAuthHandler(authService *services.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req struct {
		FirstName string `json:"first_name" binding:"required"`
		LastName  string `json:"last_name" binding:"required"`
		Email     string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email"})
		return
	}
	user := &models.User{FirstName: req.FirstName, LastName: req.LastName, Email: req.Email}
	token, err := h.authService.Register(user)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Уведомление «Добро пожаловать»
	go func() {
		payload := utils.NotificationPayload{
			RecipientID:   user.ID.Hex(),
			RecipientRole: "user",
			Title:         "Добро пожаловать!",
			Body:          "Благодарим за регистрацию. Приятного пользования нашим сервисом!",
			Type:          "welcome",
			Channel:       "email",
			Data: map[string]interface{}{
				"user_email": user.Email,
			},
		}
		_ = utils.SendNotification(context.Background(), payload)
	}()

	c.JSON(http.StatusOK, gin.H{"token": token})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var credentials struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&credentials); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid credentials"})
		return
	}
	token, err := h.authService.Login(credentials.Email, credentials.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token})
}

func (h *AuthHandler) GetProfile(c *gin.Context) {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID, err := primitive.ObjectIDFromHex(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	user, err := h.authService.GetProfile(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID, err := primitive.ObjectIDFromHex(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req struct {
		FirstName   *string    `json:"first_name"`
		LastName    *string    `json:"last_name"`
		Address     *string    `json:"address"`
		PhoneNumber *string    `json:"phone_number"`
		DateOfBirth *time.Time `json:"date_of_birth"`
		Gender      *string    `json:"gender"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := h.authService.UpdateProfile(userID, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Уведомление об обновлении профиля
	go func() {
		payload := utils.NotificationPayload{
			RecipientID:   userID.Hex(),
			RecipientRole: "user",
			Title:         "Профиль обновлён",
			Body:          "Ваш профиль был успешно обновлён.",
			Type:          "profile_updated",
			Channel:       "email",
			Data: map[string]interface{}{
				"user_id": userID.Hex(),
			},
		}
		_ = utils.SendNotification(context.Background(), payload)
	}()

	c.JSON(http.StatusOK, gin.H{"message": "Profile updated successfully"})
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userIDStr, _ := c.Get("user_id")
	userID, _ := primitive.ObjectIDFromHex(userIDStr.(string))
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password" validate:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	if err := h.authService.ChangePassword(userID, req.OldPassword, req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Уведомление о смене пароля
	go func() {
		payload := utils.NotificationPayload{
			RecipientID:   userID.Hex(),
			RecipientRole: "user",
			Title:         "Пароль изменён",
			Body:          "Ваш пароль был успешно обновлён.",
			Type:          "password_changed",
			Channel:       "email",
			Data: map[string]interface{}{
				"user_id": userID.Hex(),
			},
		}
		_ = utils.SendNotification(context.Background(), payload)
	}()

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

func (h *AuthHandler) Validate(c *gin.Context) {
	tokenStr := c.GetHeader("Authorization")
	if tokenStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing token"})
		return
	}
	tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")
	token, err := h.authService.Validate(tokenStr)
	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}
	claims := token.Claims.(jwt.MapClaims)
	c.JSON(http.StatusOK, gin.H{
		"user_id":        claims["user_id"],
		"role":           claims["role"],
		"reset_required": claims["reset_required"],
	})
}

func (h *AuthHandler) GoogleLogin(c *gin.Context) {
	var req struct {
		IDToken string `json:"id_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}
	token, err := h.authService.GoogleLogin(req.IDToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token})
}

func (h *AuthHandler) ResendPassword(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	err := h.authService.ResendTemporaryPassword(req.Email)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Уведомление о повторной отправке временного пароля
	go func() {
		// Предполагаем, что в сервисе ResendTemporaryPassword уже отправляется письмо,
		// но дублирующее пуш-уведомление может быть полезно.
		payload := utils.NotificationPayload{
			RecipientID:   "", // если нужно отправить всем менеджерам, иначе оставить пустым
			RecipientRole: "user",
			Title:         "Временный пароль отправлен",
			Body:          "На вашу почту был отправлен новый временный пароль.",
			Type:          "temporary_password_resent",
			Channel:       "email",
			Data: map[string]interface{}{
				"user_email": req.Email,
			},
		}
		_ = utils.SendNotification(context.Background(), payload)
	}()

	c.JSON(http.StatusOK, gin.H{"message": "Temporary password sent to email"})
}

func (h *AuthHandler) SetInitialPassword(c *gin.Context) {
	userIDStr, _ := c.Get("user_id")
	userID, _ := primitive.ObjectIDFromHex(userIDStr.(string))

	var req struct {
		TemporaryPassword string `json:"temporary_password" binding:"required"`
		NewPassword       string `json:"new_password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := h.authService.SetInitialPassword(userID, req.TemporaryPassword, req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Уведомление о первом назначении пароля
	go func() {
		payload := utils.NotificationPayload{
			RecipientID:   userID.Hex(),
			RecipientRole: "user",
			Title:         "Пароль установлен",
			Body:          "Ваш временный пароль был успешно изменён на постоянный.",
			Type:          "initial_password_set",
			Channel:       "email",
			Data: map[string]interface{}{
				"user_id": userID.Hex(),
			},
		}
		_ = utils.SendNotification(context.Background(), payload)
	}()

	c.JSON(http.StatusOK, gin.H{"message": "Password set successfully"})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	tokenStr := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	if tokenStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No token provided"})
		return
	}

	if err := h.authService.Logout(tokenStr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Successfully logged out"})
}

func (h *AuthHandler) GetManagers(c *gin.Context) {
	managers, err := h.authService.GetByRole("manager")
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to get managers"})
		return
	}
	c.JSON(200, managers)
}

func (h *AuthHandler) GetTotalUsers(c *gin.Context) {
	count, err := h.authService.GetTotalUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user count"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"totalUsers": count})
}

func (h *AuthHandler) AddRating(c *gin.Context) {
	userIDStr, _ := c.Get("user_id")
	userID, _ := primitive.ObjectIDFromHex(userIDStr.(string))

	var req struct {
		Rating int `json:"rating" binding:"required,min=1,max=5"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid rating"})
		return
	}

	if err := h.authService.AddRating(userID, req.Rating); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Уведомление о добавлении рейтинга (например, менеджерам)
	go func() {
		payload := utils.NotificationPayload{
			RecipientID:   "",
			RecipientRole: "manager",
			Title:         "Новый рейтинг пользователя",
			Body:          fmt.Sprintf("Пользователь %s поставил рейтинг %d", userID.Hex(), req.Rating),
			Type:          "new_user_rating",
			Channel:       "email",
			Data: map[string]interface{}{
				"user_id": userID.Hex(),
				"rating":  req.Rating,
			},
		}
		_ = utils.SendNotification(context.Background(), payload)
	}()

	c.JSON(http.StatusOK, gin.H{"message": "Rating added successfully"})
}

func (h *AuthHandler) GetRating(c *gin.Context) {
	userIDStr, _ := c.Get("user_id")
	userID, _ := primitive.ObjectIDFromHex(userIDStr.(string))

	rating, err := h.authService.GetRating(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"rating": rating})
}
