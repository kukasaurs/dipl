package models

import "time"

type Notification struct {
	ID        string    `bson:"_id,omitempty" json:"id"`
	UserID    string    `bson:"user_id" json:"user_id"`
	Role      string    `bson:"role" json:"role"` // client, cleaner, manager, admin
	Title     string    `bson:"title" json:"title"`
	Message   string    `bson:"message" json:"message"`
	Read      bool      `bson:"read" json:"read"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
}
