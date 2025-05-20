package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type Media struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	OrderID    string             `bson:"order_id" json:"order_id"`
	UploaderID string             `bson:"uploader_id" json:"uploader_id"`
	URL        string             `bson:"url" json:"url"`
	PreviewURL string             `bson:"preview_url" json:"preview_url"`
	UploadedAt time.Time          `bson:"uploaded_at" json:"uploaded_at"`
}
