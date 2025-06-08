package repository

import (
	"cleaning-app/media-service/internal/models"
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

type MediaRepository struct {
	col *mongo.Collection
}

func NewMediaRepository(db *mongo.Database) *MediaRepository {
	return &MediaRepository{col: db.Collection("media")}
}

func (r *MediaRepository) Save(ctx context.Context, m *models.Media) error {
	m.CreatedAt = time.Now()
	_, err := r.col.InsertOne(ctx, m)
	return err
}

func (r *MediaRepository) FindByOrderID(ctx context.Context, orderID string) ([]models.Media, error) {
	cursor, err := r.col.Find(ctx, bson.M{"order_id": orderID, "type": models.ReportMedia})
	if err != nil {
		return nil, err
	}
	var res []models.Media
	if err := cursor.All(ctx, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (r *MediaRepository) FindByUserID(ctx context.Context, userID string) ([]models.Media, error) {
	cursor, err := r.col.Find(ctx, bson.M{"user_id": userID, "type": models.AvatarMedia})
	if err != nil {
		return nil, err
	}
	var res []models.Media
	if err := cursor.All(ctx, &res); err != nil {
		return nil, err
	}
	return res, nil
}
