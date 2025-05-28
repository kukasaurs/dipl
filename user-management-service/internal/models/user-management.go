// user-management-service/internal/models/user.go
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Role string

const (
	RoleUser    Role = "user"
	RoleCleaner Role = "cleaner"
	RoleManager Role = "manager"
	RoleAdmin   Role = "admin"
)

type User struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"  json:"id"`
	Email         string             `bson:"email"         json:"email"`
	FirstName     string             `bson:"first_name"    json:"first_name"`
	LastName      string             `bson:"last_name"     json:"last_name"`
	PhoneNumber   string             `bson:"phone_number"  json:"phone_number"`
	Address       string             `bson:"address"       json:"address"`
	DateOfBirth   time.Time          `bson:"date_of_birth" json:"date_of_birth"`
	Gender        string             `bson:"gender"        json:"gender"`
	Role          Role               `bson:"role"          json:"role"`
	Banned        bool               `bson:"banned"        json:"banned"`
	ResetRequired bool               `bson:"reset_required"json:"reset_required"`
	Password      string             `bson:"password" json:"-"`
}

func (r Role) IsValid() bool {
	switch r {
	case RoleAdmin, RoleManager, RoleCleaner, RoleUser:
		return true
	}
	return false
}
