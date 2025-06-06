package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type NotificationType string

const (
	// уже существующие
	TypeAdminAlert    NotificationType = "admin_alert"
	TypeSystemMessage NotificationType = "system_message"
	TypeSupportEvent  NotificationType = "support_event"
	TypeOrderEvent    NotificationType = "order_event"

	TypeWelcome              NotificationType = "welcome"
	TypeSecurity             NotificationType = "security"
	TypeOrderConfirmed       NotificationType = "order_confirmed"
	TypeReminder             NotificationType = "reminder"
	TypeCleaningStarted      NotificationType = "cleaning_started"
	TypeCleaningCompleted    NotificationType = "cleaning_completed"
	TypeReviewRequest        NotificationType = "review_request"
	TypeOrderCancelled       NotificationType = "order_cancelled"
	TypePaymentSuccessful    NotificationType = "payment_successful"
	TypePaymentFailed        NotificationType = "payment_failed"
	TypeSubscriptionUpdated  NotificationType = "subscription_updated"
	TypeSubscriptionExpiring NotificationType = "subscription_expiring"
	TypeSupportMessage       NotificationType = "support_message"
	TypeAssignedOrder        NotificationType = "assigned_order"
	TypeOrderUpdated         NotificationType = "order_updated"
	TypeOrderDeleted         NotificationType = "order_deleted"
	TypeReportUploaded       NotificationType = "report_uploaded"
)

type DeliveryMethod string

const (
	DeliveryPush  DeliveryMethod = "push"
	DeliveryEmail DeliveryMethod = "email"
	DeliverySMS   DeliveryMethod = "sms"
)

// Notification хранит данные об уведомлении, которое отображается пользователю
type Notification struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID       string             `bson:"user_id" json:"user_id"`
	Title        string             `bson:"title" json:"title"`
	Message      string             `bson:"message" json:"message"`
	Type         NotificationType   `bson:"type" json:"type"`
	DeliveryType DeliveryMethod     `bson:"delivery_type" json:"delivery_type"`
	IsRead       bool               `bson:"is_read" json:"is_read"`
	Metadata     map[string]string  `bson:"metadata,omitempty" json:"metadata,omitempty"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
}

// PushNotificationRequest используется для ручной отправки push-уведомления
type PushNotificationRequest struct {
	Token   string `json:"token" binding:"required"`
	Title   string `json:"title" binding:"required"`
	Message string `json:"message" binding:"required"`
}
