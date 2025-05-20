package services

import (
	"cleaning-app/review-media-service/internal/config"
	"cleaning-app/review-media-service/internal/models"
	"cleaning-app/review-media-service/internal/repository"
	"cleaning-app/review-media-service/internal/utils"
	"context"
	"github.com/minio/minio-go/v7"
	_ "io"
	"mime/multipart"
	"time"
)

type MediaService struct {
	repo   *repository.MediaRepository
	minio  *minio.Client
	bucket string
	cfg    *config.Config
}

func NewMediaService(r *repository.MediaRepository, minioClient *minio.Client, bucket string, cfg *config.Config) *MediaService {
	return &MediaService{repo: r, minio: minioClient, bucket: bucket, cfg: cfg}
}

func (s *MediaService) UploadMedia(ctx context.Context, orderID, uploaderID string, file multipart.File, header *multipart.FileHeader) (string, error) {
	objectName := time.Now().Format("20060102150405") + "_" + header.Filename
	_, err := s.minio.PutObject(ctx, s.bucket, objectName, file, header.Size, minio.PutObjectOptions{ContentType: header.Header.Get("Content-Type")})
	if err != nil {
		return "", err
	}
	url := "/media/" + objectName
	media := &models.Media{
		OrderID:    orderID,
		UploaderID: uploaderID,
		URL:        url,
		PreviewURL: url, // placeholder
	}
	_ = s.repo.Save(ctx, media)
	_ = utils.SendNotification(ctx, s.cfg, utils.NotificationRequest{
		UserID:       "manager", // можно заменить на ID менеджера, если есть
		Role:         "manager",
		Title:        "Фотоотчёт загружен",
		Message:      "Клинер завершил уборку и загрузил фотоотчёт.",
		Type:         "report_uploaded",
		DeliveryType: "push",
		Metadata: map[string]string{
			"order_id":    orderID,
			"uploader_id": uploaderID,
		},
	})
	return url, nil
}
func (s *MediaService) GetMediaByOrder(ctx context.Context, orderID string) ([]models.Media, error) {
	return s.repo.GetByOrderID(ctx, orderID)
}
