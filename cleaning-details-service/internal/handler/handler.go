package handlers

import (
	"cleaning-app/cleaning-details-service/internal/models"
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
)

type CleaningServiceService interface {
	GetAllServices(context.Context) ([]models.CleaningService, error)
	GetActiveServices(context.Context) ([]models.CleaningService, error)
	CreateService(context.Context, *models.CleaningService) error
	UpdateService(context.Context, *models.CleaningService) error
	DeleteService(context.Context, string) error
	UpdateServiceStatus(context.Context, string, bool) error
	GetServicesByIDs(context.Context, []primitive.ObjectID) ([]models.CleaningService, error)
}

type CleaningServiceHandler struct {
	service CleaningServiceService
}

func NewCleaningServiceHandler(service CleaningServiceService) *CleaningServiceHandler {
	return &CleaningServiceHandler{service: service}
}

func (h *CleaningServiceHandler) GetActiveServices(c *gin.Context) {
	services, err := h.service.GetActiveServices(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, services)
}

func (h *CleaningServiceHandler) GetServicesByIDs(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: missing or malformed IDs"})
		return
	}

	objIDs := make([]primitive.ObjectID, 0, len(req.IDs))
	for _, idStr := range req.IDs {
		id, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid ObjectID: %s", idStr)})
			return
		}
		objIDs = append(objIDs, id)
	}

	services, err := h.service.GetServicesByIDs(c.Request.Context(), objIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, services)
}

func (h *CleaningServiceHandler) GetAllServices(c *gin.Context) {
	services, err := h.service.GetAllServices(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, services)
}

func (h *CleaningServiceHandler) CreateService(c *gin.Context) {
	var service models.CleaningService
	if err := c.ShouldBindJSON(&service); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.CreateService(c.Request.Context(), &service); err != nil {
		if errors.Is(err, models.ErrDuplicate) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, service)
}

func (h *CleaningServiceHandler) UpdateService(c *gin.Context) {
	var service models.CleaningService
	if err := c.ShouldBindJSON(&service); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdateService(c.Request.Context(), &service); err != nil {
		handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, service)
}

func (h *CleaningServiceHandler) DeleteService(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.service.DeleteService(c.Request.Context(), id); err != nil {
		handleServiceError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *CleaningServiceHandler) ToggleServiceStatus(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var statusUpdate models.ServiceStatusUpdate
	if err := c.ShouldBindJSON(&statusUpdate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdateServiceStatus(c.Request.Context(), id, statusUpdate.IsActive); err != nil {
		handleServiceError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func handleServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, models.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, models.ErrInvalidID):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, models.ErrDuplicate):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
