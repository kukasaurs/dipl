package models

import "time"

type Message struct {
	ID         string    `bson:"_id,omitempty" json:"id"`
	SenderID   string    `bson:"sender_id" json:"sender_id"`
	ReceiverID string    `bson:"receiver_id" json:"receiver_id"`
	Role       string    `bson:"role" json:"role"` // client / cleaner / support
	Text       string    `bson:"text" json:"text"`
	Timestamp  time.Time `bson:"timestamp" json:"timestamp"`
}
