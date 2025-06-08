package handler

import (
	"cleaning-app/media-service/internal/models"
	"cleaning-app/media-service/internal/services"
	"cleaning-app/media-service/internal/utils"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

type MediaHandler struct {
	svc         *service.MediaService
	orderClient *utils.OrderServiceClient
}

func NewMediaHandler(svc *service.MediaService, orderClient *utils.OrderServiceClient) *MediaHandler {
	return &MediaHandler{svc: svc, orderClient: orderClient}
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

	// userId берём сразу из JWT-AuthMiddleware
	userID := c.GetString("user_id")
	// (нет больше проверки userID != param)

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
		models.AvatarMedia, // тип аватарки
		"",                 // нет orderID
		userID,             // userID из токена
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": url})
}

func (h *MediaHandler) GetAvatars(c *gin.Context) {
	// userId берём из токена
	userID := c.GetString("user_id")

	medias, err := h.svc.GetAvatars(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, medias)
}
