package models

import "errors"

var (
	ErrNotFound   = errors.New("not found")
	ErrInvalidID  = errors.New("invalid id")
	ErrValidation = errors.New("validation error")
	ErrDuplicate  = errors.New("duplicate service name")
)
