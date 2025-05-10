package utils

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

var (
	validate *validator.Validate
	once     sync.Once
)

func GetValidator() *validator.Validate {
	once.Do(initValidator)
	return validate
}

func initValidator() {
	validate = validator.New()
}

func ParseErrors(err error) []string {
	var validationErrors validator.ValidationErrors
	ok := errors.As(err, &validationErrors)
	if !ok {
		return []string{"Unknown error"}
	}

	errs := make([]string, 0)
	for _, e := range validationErrors {
		errs = append(errs, prettyError(e))
	}

	return errs
}

func prettyError(e validator.FieldError) string {
	if strings.Contains(e.Tag(), "eq=") {
		params := e.Tag()
		params = strings.ReplaceAll(params, "|", "")
		splitted := strings.Split(params, "eq=")

		var values []string
		for _, str := range splitted {
			if str != "" {
				values = append(values, str)
			}
		}
		names := strings.Join(values, " or ")

		return fmt.Sprintf("%s must be %s", e.Field(), names)
	}

	switch e.Tag() {
	case "required":
		return e.Field() + " field is required"
	case "min":
		if e.Type().Kind() == reflect.String {
			return fmt.Sprintf("%s length must be greater than or equal to %s", e.Field(), e.Param())
		}
		return fmt.Sprintf("%s must be greater than or equal to %s", e.Field(), e.Param())
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", e.Field(), e.Param())
	default:
		return e.Error()
	}
}
