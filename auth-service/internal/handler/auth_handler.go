package handlers

import (
	"cleaning-app/auth-service/internal/models"
	"cleaning-app/auth-service/internal/utils"
	"context"
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
	GetRating(userID primitive.ObjectID) (float64, error)
	ResendTemporaryPassword(email string) error
	AddBulkRatings(ctx context.Context, customerID string, cleanerIDs []string, rating int, comment string) error
}

func NewAuthHandler(authService AuthService) *AuthHandler {
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

	go func() {
		_ = utils.SendNotificationEvent(context.Background(), utils.NotificationEvent{
			UserID:    user.ID.Hex(),
			Role:      "client",
			Type:      "welcome",
			ExtraData: map[string]string{"user_email": user.Email},
		})
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

	go func() {
		_ = utils.SendNotificationEvent(context.Background(), utils.NotificationEvent{
			UserID:    userID.Hex(),
			Role:      "client",
			Type:      "profile_updated", // если добавишь такой тип в notification-service
			Title:     "Profile updated",
			Message:   "Your profile has been updated",
			ExtraData: map[string]string{"user_id": userID.Hex()},
		})
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
		_ = utils.SendNotificationEvent(context.Background(), utils.NotificationEvent{
			UserID: userID.Hex(),
			Role:   "client",
			Type:   "security",
		})
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

func (h *AuthHandler) AddBulkRatings(c *gin.Context) {
	var req struct {
		CleanerIDs []string `json:"cleaner_ids" binding:"required,min=1,dive,required"`
		Rating     int      `json:"rating" binding:"required,min=1,max=5"`
		Comment    string   `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userIDIface, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id not found"})
		return
	}
	clientID := userIDIface.(string)
	ctx := c.Request.Context()

	if err := h.authService.AddBulkRatings(ctx, clientID, req.CleanerIDs, req.Rating, req.Comment); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ratings added to all cleaners"})
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
