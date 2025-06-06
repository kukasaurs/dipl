package handler

import (
	"cleaning-app/user-management-service/internal/models"
	"cleaning-app/user-management-service/internal/utils"
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UserHandler struct {
	service UserService
}

type UserService interface {
	CreateUser(ctx context.Context, user *models.User) error
	GetUserByID(ctx context.Context, id primitive.ObjectID) (*models.User, error)
	GetAllUsers(ctx context.Context, role models.Role) ([]models.User, error)
	ChangeUserRole(ctx context.Context, id primitive.ObjectID, newRole models.Role) error
	BlockUser(ctx context.Context, id primitive.ObjectID) error
	UnblockUser(ctx context.Context, id primitive.ObjectID) error
	AddXPToUser(ctx context.Context, id primitive.ObjectID, xp int) (*models.GamificationStatus, error)
	GetGamificationStatus(ctx context.Context, id primitive.ObjectID) (*models.GamificationStatus, error)
}

func NewUserHandler(s UserService) *UserHandler {
	return &UserHandler{service: s}
}

// POST /api/users/gamification/add-xp
func (h *UserHandler) AddXP(c *gin.Context) {
	// 1) Считываем JSON из тела { "user_id": "<hex-id>", "xp": <int> }
	var payload struct {
		UserID string `json:"user_id" binding:"required"`
		XP     int    `json:"xp"      binding:"required"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	// 2) Конвертируем user_id в ObjectID
	userID, err := primitive.ObjectIDFromHex(payload.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	// 3) Добавляем XP
	status, err := h.service.AddXPToUser(c.Request.Context(), userID, payload.XP)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

// GET /api/users/gamification/status
func (h *UserHandler) GetStatus(c *gin.Context) {
	// 1. Получаем userID из JWT-middleware
	idStr, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID not found in token"})
		return
	}
	idHex, ok := idStr.(string)
	if !ok || idHex == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID in token"})
		return
	}

	// 2. Конвертируем hex-строку в ObjectID
	userID, err := primitive.ObjectIDFromHex(idHex)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID format"})
		return
	}

	// 3. Получаем статус из сервиса
	status, err := h.service.GetGamificationStatus(context.TODO(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

// GET /api/users/me
func (h *UserHandler) GetMe(c *gin.Context) {
	userID := c.GetString("userId")
	objID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	user, err := h.service.GetUserByID(c.Request.Context(), objID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// GET /api/users (admin/manager only)
func (h *UserHandler) GetAllUsers(c *gin.Context) {
	role := c.Query("filter_role")
	users, err := h.service.GetAllUsers(c.Request.Context(), models.ToRole(role))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch users"})
		return
	}

	c.JSON(http.StatusOK, users)
}

// POST /api/users (manager+admin)
func (h *UserHandler) CreateUser(c *gin.Context) {
	var input struct {
		Email    string      `json:"email"    binding:"required,email"`
		Name     string      `json:"name"     binding:"required"`
		Phone    string      `json:"phone"    binding:"required"`
		Password string      `json:"password" binding:"required,min=6"`
		Role     models.Role `json:"role"     binding:"required,oneof=manager admin"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Хешируем пароль
	hash, err := utils.HashPassword(input.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось захешировать пароль"})
		return
	}

	// Определяем, кто создаёт
	creatorRole := c.GetString("role")
	role := input.Role
	if creatorRole == string(models.RoleManager) {
		// Менеджер не может создавать никого кроме обычных юзеров
		role = models.RoleUser
	}

	user := models.User{
		ID:            primitive.NewObjectID(),
		Email:         input.Email,
		FirstName:     input.Name,
		LastName:      "",
		PhoneNumber:   input.Phone,
		Role:          role,
		Banned:        false,
		ResetRequired: false,
		Password:      hash,
	}

	if err := h.service.CreateUser(c.Request.Context(), &user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	// Уведомление созданному пользователю
	go func() {
		payload := utils.NotificationPayload{
			RecipientID:   user.ID.Hex(),
			RecipientRole: "user",
			Title:         "Аккаунт создан",
			Body:          fmt.Sprintf("Ваш аккаунт (%s) успешно создан.", user.Email),
			Type:          "user_created",
			Channel:       "email",
			Data: map[string]interface{}{
				"user_id": user.ID.Hex(),
			},
		}
		_ = utils.SendNotification(context.Background(), payload)
	}()

	// Отдаём только безопасные поля
	c.JSON(http.StatusCreated, gin.H{
		"id":         user.ID.Hex(),
		"email":      user.Email,
		"first_name": user.FirstName,
		"role":       user.Role,
	})
}

// PUT /api/users/:id/role (admin only) - Изменение роли существующего пользователя
func (h *UserHandler) ChangeUserRole(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	var input struct {
		Role models.Role `json:"role" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Получаем старую информацию для уведомления
	user, err := h.service.GetUserByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	if err := h.service.ChangeUserRole(c.Request.Context(), id, input.Role); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update role"})
		return
	}

	// Уведомление пользователю о смене роли
	go func() {
		payload := utils.NotificationPayload{
			RecipientID:   id.Hex(),
			RecipientRole: "user",
			Title:         "Роль изменена",
			Body:          fmt.Sprintf("Ваша роль изменена с '%s' на '%s'.", user.Role, input.Role),
			Type:          "role_changed",
			Channel:       "email",
			Data: map[string]interface{}{
				"user_id":  id.Hex(),
				"new_role": input.Role,
				"old_role": user.Role,
			},
		}
		_ = utils.SendNotification(context.Background(), payload)
	}()

	c.JSON(http.StatusOK, gin.H{"message": "user role updated"})
}

// PUT /api/users/:id/block
func (h *UserHandler) BlockUser(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	// Получаем информацию о пользователе для уведомления
	_, err = h.service.GetUserByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	if err := h.service.BlockUser(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to block user"})
		return
	}

	// Уведомление пользователю о блокировке
	go func() {
		payload := utils.NotificationPayload{
			RecipientID:   id.Hex(),
			RecipientRole: "user",
			Title:         "Аккаунт заблокирован",
			Body:          "Ваш аккаунт временно заблокирован. Пожалуйста, свяжитесь с поддержкой.",
			Type:          "user_blocked",
			Channel:       "email",
			Data: map[string]interface{}{
				"user_id": id.Hex(),
			},
		}
		_ = utils.SendNotification(context.Background(), payload)
	}()

	c.JSON(http.StatusOK, gin.H{"message": "user blocked"})
}

// PUT /api/users/:id/unblock
func (h *UserHandler) UnblockUser(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	// Получаем информацию о пользователе для уведомления
	_, err = h.service.GetUserByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	if err := h.service.UnblockUser(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to unblock user"})
		return
	}

	// Уведомление пользователю о разблокировке
	go func() {
		payload := utils.NotificationPayload{
			RecipientID:   id.Hex(),
			RecipientRole: "user",
			Title:         "Аккаунт разблокирован",
			Body:          "Ваш аккаунт был разблокирован. Добро пожаловать обратно!",
			Type:          "user_unblocked",
			Channel:       "email",
			Data: map[string]interface{}{
				"user_id": id.Hex(),
			},
		}
		_ = utils.SendNotification(context.Background(), payload)
	}()

	c.JSON(http.StatusOK, gin.H{"message": "user unblocked"})
}

func (h *UserHandler) GetUserByID(c *gin.Context) {
	// 1) Парсим hex → ObjectID
	hexID := c.Param("id")
	objID, err := primitive.ObjectIDFromHex(hexID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	// 2) Вызываем сервис
	user, err := h.service.GetUserByID(c.Request.Context(), objID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// 3) Отдаём нужные поля
	c.JSON(http.StatusOK, gin.H{
		"id":    user.ID.Hex(),
		"email": user.Email,
		"role":  user.Role,
	})
}
