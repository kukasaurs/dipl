package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type SubscriptionStatus string

const (
	StatusActive   SubscriptionStatus = "active"
	StatusExpired  SubscriptionStatus = "expired"
	StatusCanceled SubscriptionStatus = "canceled"
)

type Frequency string

const (
	Weekly    Frequency = "weekly"
	BiWeekly  Frequency = "biweekly"
	TriWeekly Frequency = "triweekly"
	Monthly   Frequency = "monthly"
)

type ScheduleSpec struct {
	Frequency   Frequency `bson:"frequency"   json:"frequency"`
	DaysOfWeek  []string  `bson:"days_of_week" json:"days_of_week,omitempty"`
	WeekNumbers []int     `bson:"week_numbers" json:"week_numbers,omitempty"`
}

type Subscription struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"           json:"id"`
	OrderID         primitive.ObjectID `bson:"order_id"                json:"order_id"`
	UserID          primitive.ObjectID `bson:"user_id"                 json:"user_id"`
	StartDate       time.Time          `bson:"start_date"              json:"start_date"`
	EndDate         time.Time          `bson:"end_date"                json:"end_date"`
	Schedule        ScheduleSpec       `bson:"schedule"      json:"schedule"`
	Price           float64            `bson:"price"                   json:"price"`
	Status          SubscriptionStatus `bson:"status"                  json:"status"`
	CreatedAt       time.Time          `bson:"created_at"              json:"created_at"`
	UpdatedAt       time.Time          `bson:"updated_at"              json:"updated_at"`
	LastOrderDate   *time.Time         `bson:"last_order_date,omitempty"   json:"last_order_date"`
	NextPlannedDate *time.Time         `bson:"next_planned_date,omitempty" json:"next_planned_date"`
}
