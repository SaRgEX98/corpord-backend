package service

import "errors"

var (
	ErrNoFields           = errors.New("no fields")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailExists        = errors.New("email already exists")
	ErrBusNotFound        = errors.New("bus not found")
)
