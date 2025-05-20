package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type Review struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	OrderID    string             `bson:"order_id" json:"order_id"`
	ReviewerID string             `bson:"reviewer_id" json:"reviewer_id"`
	TargetID   string             `bson:"target_id" json:"target_id"`
	TargetRole string             `bson:"target_role" json:"target_role"` // cleaner or client
	Rating     int                `bson:"rating" json:"rating"`
	Comment    string             `bson:"comment" json:"comment"`
	CreatedAt  time.Time          `bson:"created_at" json:"created_at"`
}
