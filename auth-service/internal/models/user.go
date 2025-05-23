package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
	"time"
)

type User struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"`
	Email         string             `bson:"email" validate:"required,email"`
	Password      string             `bson:"password" validate:"required,min=6"`
	Role          string             `bson:"role" validate:"required,oneof=user cleaner manager admin"`
	Banned        bool               `bson:"banned"`
	FirstName     string             `bson:"first_name" validate:"omitempty"`
	LastName      string             `bson:"last_name" validate:"omitempty"`
	Address       string             `bson:"address" validate:"omitempty"`
	PhoneNumber   string             `bson:"phone_number" validate:"omitempty,e164"`
	DateOfBirth   time.Time          `bson:"date_of_birth" validate:"omitempty"`
	Gender        string             `bson:"gender" validate:"omitempty,oneof=male female other"`
	ResetRequired bool               `bson:"reset_required"`
	Ratings       []int              `bson:"ratings" validate:"omitempty"`
	AverageRating float64            `bson:"average_rating" validate:"omitempty"`
}

func (u *User) HashPassword() error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashed)
	return nil
}

func (u *User) ComparePassword(password string) error {
	return bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
}
