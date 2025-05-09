package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type SubscriptionLog struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"`
	SubscriptionID primitive.ObjectID `bson:"subscription_id"`
	ClientID       string             `bson:"client_id"`
	Action         string             `bson:"action"` // created, extended, cancelled, expired
	Timestamp      time.Time          `bson:"timestamp"`
	Details        map[string]string  `bson:"details,omitempty"`
}
