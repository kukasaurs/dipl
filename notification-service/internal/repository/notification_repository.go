package repository

import (
	"cleaning-app/notification-service/internal/models"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type NotificationRepository interface {
	Create(ctx context.Context, notif *models.Notification) error
	List(ctx context.Context, userID string) ([]models.Notification, error)
	MarkAsRead(ctx context.Context, id string) error
}

type mongoRepo struct {
	col *mongo.Collection
}

func NewMongoNotificationRepo(col *mongo.Database) NotificationRepository {
	return &mongoRepo{col: col}
}

func (r *mongoRepo) Create(ctx context.Context, notif *models.Notification) error {
	notif.CreatedAt = time.Now()
	notif.Read = false
	_, err := r.col.InsertOne(ctx, notif)
	return err
}

func (r *mongoRepo) List(ctx context.Context, userID string) ([]models.Notification, error) {
	filter := bson.M{"user_id": userID}
	cursor, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var notifications []models.Notification
	if err = cursor.All(ctx, &notifications); err != nil {
		return nil, err
	}
	return notifications, nil
}

func (r *mongoRepo) MarkAsRead(ctx context.Context, id string) error {
	_, err := r.col.UpdateByID(ctx, id, bson.M{"$set": bson.M{"read": true}})
	return err
}
