package service

import "errors"

var (
	ErrNoFields            = errors.New("no fields")
	ErrUserNotFound        = errors.New("user not found")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrEmailExists         = errors.New("email already exists")
	ErrBusNotFound         = errors.New("bus not found")
	ErrBusCategoryNotFound = errors.New("bus category not found")
	ErrBusCategoryExists   = errors.New("bus category already exists")
	ErrBusStatusNotFound   = errors.New("bus status not found")
	ErrBusStatusExists     = errors.New("bus status already exists")
)
