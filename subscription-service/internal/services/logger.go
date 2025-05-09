package services

import (
	"cleaning-app/subscription-service/internal/models"
	"context"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

func LogSubscriptionAction(ctx context.Context, db *mongo.Database, subID primitive.ObjectID, clientID, action string, details map[string]string) error {
	logEntry := models.SubscriptionLog{
		ID:             primitive.NewObjectID(),
		SubscriptionID: subID,
		ClientID:       clientID,
		Action:         action,
		Timestamp:      time.Now(),
		Details:        details,
	}
	_, err := db.Collection("subscription_logs").InsertOne(ctx, logEntry)
	return err
}
