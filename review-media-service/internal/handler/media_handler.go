package handler

import (
	"cleaning-app/review-media-service/internal/services"
	"github.com/gin-gonic/gin"
	"net/http"
)

type MediaHandler struct {
	service *services.MediaService
}

func NewMediaHandler(service *services.MediaService) *MediaHandler {
	return &MediaHandler{service: service}
}

func (h *MediaHandler) Upload(c *gin.Context) {
	orderID := c.PostForm("order_id")
	uploaderID := c.GetString("userId")

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File missing"})
		return
	}

	url, err := h.service.UploadMedia(c.Request.Context(), orderID, uploaderID, file, header)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": url})
}
func (h *MediaHandler) GetMediaByOrder(c *gin.Context) {
	orderID := c.Param("order_id")
	media, err := h.service.GetMediaByOrder(c.Request.Context(), orderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, media)
}
