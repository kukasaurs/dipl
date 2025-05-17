package repository

import (
	"cleaning-app/review-media-service/internal/models"
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

type MediaRepository struct {
	collection *mongo.Collection
}

func NewMediaRepository(db *mongo.Database) *MediaRepository {
	return &MediaRepository{collection: db.Collection("media")}
}

func (r *MediaRepository) Save(ctx context.Context, media *models.Media) error {
	media.UploadedAt = time.Now()
	_, err := r.collection.InsertOne(ctx, media)
	return err
}

func (r *MediaRepository) GetByOrderID(ctx context.Context, orderID string) ([]models.Media, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"order_id": orderID})
	if err != nil {
		return nil, err
	}
	var results []models.Media
	err = cursor.All(ctx, &results)
	return results, err
}
