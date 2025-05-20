package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TicketStatus string

const (
	StatusOpen       TicketStatus = "open"
	StatusInProgress TicketStatus = "in_progress"
	StatusClosed     TicketStatus = "closed"
)

type Ticket struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ClientID  string             `bson:"client_id" json:"client_id"`
	Subject   string             `bson:"subject" json:"subject"`
	Status    TicketStatus       `bson:"status" json:"status"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}

type Message struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TicketID   primitive.ObjectID `bson:"ticket_id" json:"ticket_id"`
	SenderID   string             `bson:"sender_id" json:"sender_id"`
	SenderRole string             `bson:"sender_role" json:"sender_role"` // "user" or "manager"
	Text       string             `bson:"text" json:"text"`
	Timestamp  time.Time          `bson:"timestamp" json:"timestamp"`
}
