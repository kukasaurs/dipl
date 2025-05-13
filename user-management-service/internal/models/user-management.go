package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Role string

const (
	RoleAdmin   Role = "admin"
	RoleManager Role = "manager"
	RoleCleaner Role = "cleaner"
	RoleUser    Role = "user"
)

type User struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Email         string             `bson:"email" json:"email" validate:"required,email"`
	Password      string             `bson:"password" json:"-" validate:"required,min=6"`
	Role          Role               `bson:"role" json:"role" validate:"required,oneof=user cleaner manager admin"`
	Banned        bool               `bson:"banned" json:"banned"`
	FirstName     string             `bson:"first_name" json:"first_name" validate:"omitempty"`
	LastName      string             `bson:"last_name" json:"last_name" validate:"omitempty"`
	Address       string             `bson:"address" json:"address" validate:"omitempty"`
	PhoneNumber   string             `bson:"phone_number" json:"phone_number" validate:"omitempty,e164"`
	DateOfBirth   time.Time          `bson:"date_of_birth" json:"date_of_birth,omitempty" validate:"omitempty"`
	Gender        string             `bson:"gender" json:"gender" validate:"omitempty,oneof=male female other"`
	ResetRequired bool               `bson:"reset_required" json:"reset_required"`
}

func (r Role) IsValid() bool {
	switch r {
	case RoleAdmin, RoleManager, RoleCleaner, RoleUser:
		return true
	}
	return false
}
