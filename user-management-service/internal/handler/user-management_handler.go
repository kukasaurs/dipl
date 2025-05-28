package handler

import (
	"cleaning-app/user-management-service/internal/models"
	"cleaning-app/user-management-service/internal/services"
	"cleaning-app/user-management-service/internal/utils"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
)

type UserHandler struct {
	service *services.UserService
}

func NewUserHandler(s *services.UserService) *UserHandler {
	return &UserHandler{service: s}
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
	users, err := h.service.GetAllUsers(c.Request.Context())
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

	if err := h.service.ChangeUserRole(c.Request.Context(), id, input.Role); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update role"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "user role updated"})
}

// PUT /api/users/:id/block
func (h *UserHandler) BlockUser(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	if err := h.service.BlockUser(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to block user"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "user blocked"})
}

// PUT /api/users/:id/unblock
func (h *UserHandler) UnblockUser(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	if err := h.service.UnblockUser(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to unblock user"})
		return
	}
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
