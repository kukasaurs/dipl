package service

import (
	"cleaning-app/media-service/internal/models"
	"cleaning-app/media-service/internal/repository"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
)

type MediaService struct {
	repo   *repository.MediaRepository
	minio  *minio.Client
	bucket string
}

func NewMediaService(r *repository.MediaRepository, m *minio.Client, bucket string) *MediaService {
	return &MediaService{repo: r, minio: m, bucket: bucket}
}

func (s *MediaService) Upload(
	ctx context.Context,
	reader io.Reader,
	size int64,
	contentType, filename string,
	mType models.MediaType,
	orderID, userID string,
) (string, error) {
	objectKey := fmt.Sprintf("%s/%d_%s", mType, time.Now().UnixNano(), filename)
	_, err := s.minio.PutObject(ctx, s.bucket, objectKey, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("%s/%s/%s", s.minio.EndpointURL(), s.bucket, objectKey)

	media := &models.Media{
		FileName:  filename,
		ObjectKey: objectKey,
		URL:       url,
		Type:      mType,
		OrderID:   orderID,
		UserID:    userID,
	}
	if err := s.repo.Save(ctx, media); err != nil {
		return "", err
	}
	return url, nil
}

func (s *MediaService) GetReports(ctx context.Context, orderID string) ([]models.Media, error) {
	return s.repo.FindByOrderID(ctx, orderID)
}

func (s *MediaService) GetAvatars(ctx context.Context, userID string) ([]models.Media, error) {
	return s.repo.FindByUserID(ctx, userID)
}
