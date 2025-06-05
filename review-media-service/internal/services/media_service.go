package services

import (
	"cleaning-app/review-media-service/internal/config"
	"cleaning-app/review-media-service/internal/models"
	"context"
	"github.com/minio/minio-go/v7"
	_ "io"
	"mime/multipart"
	"time"
)

type MediaService struct {
	repo   MediaRepository
	minio  *minio.Client
	bucket string
	cfg    *config.Config
}

type MediaRepository interface {
	Save(ctx context.Context, media *models.Media) error
	GetByOrderID(ctx context.Context, orderID string) ([]models.Media, error)
}

func NewMediaService(r MediaRepository, minioClient *minio.Client, bucket string, cfg *config.Config) *MediaService {
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

	return url, nil
}
func (s *MediaService) GetMediaByOrder(ctx context.Context, orderID string) ([]models.Media, error) {
	return s.repo.GetByOrderID(ctx, orderID)
}
