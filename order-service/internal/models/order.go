package models

import (
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type OrderStatus string

const (
	StatusPending   OrderStatus = "pending"
	StatusAssigned  OrderStatus = "assigned"
	StatusCompleted OrderStatus = "completed"
	StatusPaid      OrderStatus = "paid"
	StatusFailed    OrderStatus = "payment_failed"
)

type Order struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ClientID       string             `bson:"client_id" json:"client_id"`
	CleanerID      *string            `bson:"cleaner_id,omitempty" json:"cleaner_id,omitempty"`
	Address        string             `bson:"address" json:"address"`
	ServiceType    string             `bson:"service_type" json:"service_type"`
	ServiceIDs     []string           `bson:"service_ids" json:"service_ids"`
	ServiceDetails []Service          `bson:"service_details,omitempty" json:"service_details,omitempty"`
	Date           time.Time          `bson:"date" json:"date"`
	Status         OrderStatus        `bson:"status" json:"status"`
	PhotoURL       *string            `bson:"photo_url,omitempty" json:"photo_url,omitempty"`
	Comment        string             `bson:"comment,omitempty" json:"comment,omitempty"`
	CreatedAt      time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt      time.Time          `bson:"updated_at" json:"updated_at"`
}

type Service struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

func (o *Order) Validate() error {
	if o.ClientID == "" || o.Address == "" || o.ServiceType == "" || o.Date.IsZero() {
		return errors.New("missing required order fields")
	}
	return nil
}
