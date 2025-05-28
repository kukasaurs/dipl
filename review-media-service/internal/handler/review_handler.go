package handler

import (
	"cleaning-app/review-media-service/internal/models"
	"cleaning-app/review-media-service/internal/services"
	"github.com/gin-gonic/gin"
	"net/http"
)

type ReviewHandler struct {
	service ReviewService
}

type ReviewService interface {
}

func NewReviewHandler(service ReviewService) *ReviewHandler {
	return &ReviewHandler{service: service}
}

func (h *ReviewHandler) CreateReview(c *gin.Context) {
	var review models.Review
	if err := c.ShouldBindJSON(&review); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	// 🧠 Авторизованный пользователь
	userID := c.GetString("userId")
	role := c.GetString("role")

	review.ReviewerID = userID
	if (role == "client" && review.TargetRole != "cleaner") ||
		(role == "cleaner" && review.TargetRole != "client") {
		c.JSON(http.StatusForbidden, gin.H{"error": "Invalid target role for your user role"})
		return
	}

	// Проверка на повторный отзыв
	exists, err := h.service.ReviewExists(c.Request.Context(), review.OrderID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing review"})
		return
	}
	if exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You have already submitted a review for this order"})
		return
	}

	if err := h.service.CreateReview(c.Request.Context(), &review); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Review saved"})
}

func (h *ReviewHandler) GetReviewsByUser(c *gin.Context) {
	id := c.Param("id")
	reviews, err := h.service.GetReviewsByTarget(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, reviews)
}
func (h *ReviewHandler) TriggerReviewReminder(c *gin.Context) {
	role := c.GetString("role")
	if role != "admin" && role != "manager" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admins or managers can send reminders"})
		return
	}
	var req struct {
		OrderID string `json:"order_id"`
		UserID  string `json:"user_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}
	go h.service.ScheduleReviewRequest(req.UserID, req.OrderID)
	c.JSON(http.StatusOK, gin.H{"message": "Reminder scheduled"})
}

func (h *ReviewHandler) GetStatistics(c *gin.Context) {
	stats, err := h.service.GetReviewStatistics(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to compute stats"})
		return
	}
	c.JSON(http.StatusOK, stats)
}
