package models

import (
	"cleaning-app/cleaning-details-service/utils"
	"fmt"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"strings"
)

type CleaningService struct {
	ID        primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Name      string             `json:"name" bson:"name" validate:"required"`
	Price     float64            `json:"price" bson:"price" validate:"required,gt=0"`
	IsActive  bool               `json:"isActive" bson:"isActive"`
	CreatedAt primitive.DateTime `json:"createdAt,omitempty" bson:"createdAt,omitempty"`
	UpdatedAt primitive.DateTime `json:"updatedAt,omitempty" bson:"updatedAt,omitempty"`
}

func (cs CleaningService) Validate() error {
	validate := utils.GetValidator()
	err := validate.Struct(cs)
	if err != nil {
		errs := utils.ParseErrors(err)
		return fmt.Errorf("%w: %s", ErrValidation, strings.Join(errs, " // "))
	}

	return nil
}

type ServiceStatusUpdate struct {
	IsActive bool `json:"isActive" bson:"isActive"`
}
