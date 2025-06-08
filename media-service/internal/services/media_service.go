package service

import (
	"cleaning-app/media-service/internal/models"
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
)

type MediaService struct {
	repo      MediaRepository
	minio     *minio.Client
	bucket    string
	publicURL string
}

type MediaRepository interface {
	Save(ctx context.Context, m *models.Media) error
	FindByOrderID(ctx context.Context, orderID string) ([]models.Media, error)
	FindByUserID(ctx context.Context, userID string) ([]models.Media, error)
	FindByID(ctx context.Context, id string) (*models.Media, error)
}

func NewMediaService(r MediaRepository, m *minio.Client, bucket string, publicURL string) *MediaService {
	return &MediaService{repo: r, minio: m, bucket: bucket, publicURL: publicURL}
}
func (s *MediaService) GetMediaByID(ctx context.Context, id string) (*models.Media, error) {
	return s.repo.FindByID(ctx, id)
}
func (s *MediaService) Upload(ctx context.Context, reader io.Reader, size int64, contentType, filename string, mType models.MediaType, orderID, userID string) (string, error) {
	objectKey := fmt.Sprintf("%s/%d_%s", mType, time.Now().UnixNano(), filename)
	_, err := s.minio.PutObject(ctx, s.bucket, objectKey, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("%s/%s/%s",
		strings.TrimRight(s.publicURL, "/"), // http://localhost:9000
		s.bucket,                            // media-cleaning
		objectKey,                           // avatar/...
	)
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
func (s *MediaService) GeneratePresignedURL(ctx context.Context, objectName string) (string, error) {
	reqParams := make(url.Values)
	presignedURL, err := s.minio.PresignedGetObject(ctx, s.bucket, objectName, time.Hour, reqParams)
	if err != nil {
		return "", err
	}
	return presignedURL.String(), nil
}
