package repository

import (
	"cleaning-app/notification-service/internal/models"
	"context"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type NotificationRepository struct {
	collection *mongo.Collection
}

func NewNotificationRepository(db *mongo.Database) *NotificationRepository {
	return &NotificationRepository{
		collection: db.Collection("notifications"),
	}
}

func (r *NotificationRepository) Create(ctx context.Context, notification *models.Notification) error {
	notification.CreatedAt = time.Now()
	_, err := r.collection.InsertOne(ctx, notification)
	return err
}

func (r *NotificationRepository) GetByUserID(ctx context.Context, userID string, limit, offset int64) ([]models.Notification, error) {
	filter := bson.M{"user_id": userID}
	opts := options.Find().SetLimit(limit).SetSkip(offset).SetSort(bson.D{{"created_at", -1}})
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var notifications []models.Notification
	for cursor.Next(ctx) {
		var n models.Notification
		if err := cursor.Decode(&n); err != nil {
			return nil, err
		}
		notifications = append(notifications, n)
	}
	return notifications, nil
}

func (r *NotificationRepository) MarkAsRead(ctx context.Context, id primitive.ObjectID) error {
	filter := bson.M{"_id": id}
	update := bson.M{"$set": bson.M{"is_read": true}}
	_, err := r.collection.UpdateOne(ctx, filter, update)
	return err
}
