package models

import (
	"fmt"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"strings"

	"cleaning-app/cleaning-details-service/utils/validator"
)

// CleaningService represents a cleaning service
type CleaningService struct {
	ID        primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Name      string             `json:"name" bson:"name" validate:"required"`
	Price     float64            `json:"price" bson:"price" validate:"required,gt=0"`
	IsActive  bool               `json:"isActive" bson:"isActive"`
	CreatedAt primitive.DateTime `json:"createdAt,omitempty" bson:"createdAt,omitempty"`
	UpdatedAt primitive.DateTime `json:"updatedAt,omitempty" bson:"updatedAt,omitempty"`
}

// Validate validates the CleaningService
func (cs CleaningService) Validate() error {
	validate := validator.GetValidator()
	err := validate.Struct(cs)
	if err != nil {
		errs := validator.ParseErrors(err)
		return fmt.Errorf("%w: %s", ErrValidation, strings.Join(errs, " // "))
	}

	return nil
}

// ServiceStatusUpdate represents a status update request
type ServiceStatusUpdate struct {
	IsActive bool `json:"isActive" bson:"isActive"`
}
