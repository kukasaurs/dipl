package repository

import (
	"cleaning-app/media-service/internal/models"
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
func (r *MediaRepository) FindByID(ctx context.Context, id string) (*models.Media, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var media models.Media
	err = r.col.FindOne(ctx, bson.M{"_id": objID}).Decode(&media)
	if err != nil {
		return nil, err
	}
	return &media, nil
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
	filter := bson.M{
		"user_id": userID, // или "userId" в точности как в БД
		"type":    models.AvatarMedia,
	}
	cursor, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Гарантируем [] вместо null
	res := make([]models.Media, 0)
	if err := cursor.All(ctx, &res); err != nil {
		return nil, err
	}
	return res, nil
}
