package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NotificationType представляет типы уведомлений
type NotificationType string

const (
	TypeOrderEvent    NotificationType = "order_event"
	TypeSupportEvent  NotificationType = "support_event"
	TypeAdminAlert    NotificationType = "admin_alert"
	TypeSystemMessage NotificationType = "system_message"
)

// DeliveryMethod представляет способ доставки уведомления
type DeliveryMethod string

const (
	DeliveryEmail DeliveryMethod = "email"
	DeliverySMS   DeliveryMethod = "sms"
	DeliveryPush  DeliveryMethod = "push"
)

type Notification struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID       string             `bson:"user_id" json:"user_id"`
	Title        string             `bson:"title" json:"title"`
	Message      string             `bson:"message" json:"message"`
	Type         NotificationType   `bson:"type" json:"type"`
	DeliveryType DeliveryMethod     `bson:"delivery_type" json:"delivery_type"`
	IsRead       bool               `bson:"is_read" json:"is_read"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
	Metadata     map[string]string  `bson:"metadata,omitempty" json:"metadata,omitempty"`
}
