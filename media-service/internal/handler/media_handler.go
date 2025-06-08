package handler

import (
	"cleaning-app/media-service/internal/models"
	"context"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"net/http"
)

type MediaHandler struct {
	svc         MediaService
	orderClient OrderServiceClient
}

type MediaService interface {
	Upload(ctx context.Context, reader io.Reader, size int64, contentType, filename string, mType models.MediaType, orderID, userID string) (string, error)
	GetReports(ctx context.Context, orderID string) ([]models.Media, error)
	GetAvatars(ctx context.Context, userID string) ([]models.Media, error)
	GeneratePresignedURL(ctx context.Context, objectName string) (string, error)
	GetMediaByID(ctx context.Context, id string) (*models.Media, error)
}

type OrderServiceClient interface {
	IsCleaner(ctx context.Context, orderID, authHeader string) (bool, error)
}

func NewMediaHandler(svc MediaService, orderClient OrderServiceClient) *MediaHandler {
	return &MediaHandler{svc: svc, orderClient: orderClient}
}
func (h *MediaHandler) GetPresignedURLByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	media, err := h.svc.GetMediaByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "media not found"})
		return
	}

	url, err := h.svc.GeneratePresignedURL(c.Request.Context(), media.ObjectKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"file_name": media.FileName,
		"url":       url,
	})
}
func (h *MediaHandler) UploadReport(c *gin.Context) {
	orderID := c.Param("orderId")
	authHdr := c.GetHeader("Authorization")

	ok, err := h.orderClient.IsCleaner(
		c.Request.Context(),
		orderID,
		authHdr,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "validation error"})
		return
	}
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "only assigned cleaner can upload report"})
		return
	}

	// дальше — multipart/FormFile + svc.Upload
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()

	url, err := h.svc.Upload(
		c.Request.Context(), file, header.Size,
		header.Header.Get("Content-Type"),
		header.Filename,
		models.ReportMedia,
		orderID, "",
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": url})
}

func (h *MediaHandler) GetReports(c *gin.Context) {
	orderID := c.Param("orderId")
	medias, err := h.svc.GetReports(c.Request.Context(), orderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, medias)
}

func (h *MediaHandler) UploadAvatar(c *gin.Context) {
	log.Println("[UploadAvatar] Headers:", c.Request.Header)

	userID := c.GetString("userId")

	file, header, err := c.Request.FormFile("file")
	log.Println("[UploadAvatar] FormFile error:", err)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()

	url, err := h.svc.Upload(
		c.Request.Context(),
		file,
		header.Size,
		header.Header.Get("Content-Type"),
		header.Filename,
		models.AvatarMedia,
		"",
		userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": url})
}

func (h *MediaHandler) GetAvatars(c *gin.Context) {
	// userId берём из токена
	userID := c.GetString("userId")

	medias, err := h.svc.GetAvatars(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Printf("[GetAvatars] found %d docs for user %s\n", len(medias), userID)
	c.JSON(http.StatusOK, medias)
}
