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
	RoleAll     Role = "all"
)

func ToRole(role string) Role {
	switch role {
	case "user":
		return RoleUser
	case "cleaner":
		return RoleCleaner
	case "manager":
		return RoleManager
	case "admin":
		return RoleAdmin
	default:
		return RoleAll
	}
}

type User struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"  json:"id"`
	Email         string             `bson:"email"          json:"email"`
	FirstName     string             `bson:"first_name"     json:"first_name"`
	LastName      string             `bson:"last_name"      json:"last_name"`
	PhoneNumber   string             `bson:"phone_number"   json:"phone_number"`
	Address       string             `bson:"address"        json:"address"`
	DateOfBirth   time.Time          `bson:"date_of_birth"  json:"date_of_birth"`
	Gender        string             `bson:"gender"         json:"gender"`
	Role          Role               `bson:"role"           json:"role"`
	Banned        bool               `bson:"banned"         json:"banned"`
	ResetRequired bool               `bson:"reset_required" json:"reset_required"`
	Password      string             `bson:"password"       json:"-"`
	XPTotal       int                `bson:"xp_total"      json:"xp_total"`
	CurrentLevel  int                `bson:"current_level" json:"current_level"`
}

func (r Role) IsValid() bool {
	switch r {
	case RoleAdmin, RoleManager, RoleCleaner, RoleUser:
		return true
	}
	return false
}
