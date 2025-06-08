package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type MediaType string

const (
	ReportMedia MediaType = "report"
	AvatarMedia MediaType = "avatar"
)

type Media struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	FileName  string             `bson:"file_name"`
	ObjectKey string             `bson:"object_key"`
	URL       string             `bson:"url"`
	Type      MediaType          `bson:"type"`
	OrderID   string             `bson:"order_id,omitempty"` // для фотоотчёта
	UserID    string             `bson:"user_id,omitempty"`  // для аватарки
	CreatedAt time.Time          `bson:"created_at"`
}
