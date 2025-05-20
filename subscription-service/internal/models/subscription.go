package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SubscriptionStatus string

const (
	StatusActive    SubscriptionStatus = "active"
	StatusExpired   SubscriptionStatus = "expired"
	StatusCancelled SubscriptionStatus = "cancelled"
)

type Subscription struct {
	ID                 primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ClientID           string             `bson:"client_id" json:"client_id"`
	ServiceIDs         []string           `bson:"service_ids" json:"service_ids"`
	DaysOfWeek         []string           `bson:"days_of_week" json:"days_of_week"`               // ["Monday", "Wednesday"]
	TotalCleanings     int                `bson:"total_cleanings" json:"total_cleanings"`         // сколько оплатил
	RemainingCleanings int                `bson:"remaining_cleanings" json:"remaining_cleanings"` // сколько осталось
	LastOrderDate      *time.Time         `bson:"last_order_date,omitempty" json:"last_order_date,omitempty"`
	NextPlannedDate    *time.Time         `bson:"next_planned_date,omitempty" json:"next_planned_date,omitempty"`
	Status             SubscriptionStatus `bson:"status" json:"status"`
	CreatedAt          time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt          time.Time          `bson:"updated_at" json:"updated_at"`
}
