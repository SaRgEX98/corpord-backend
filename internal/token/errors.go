package token

import "errors"

var (
	ErrInvalidToken      = errors.New("invalid token")
	ErrInvalidAuthHeader = errors.New("invalid auth header")
)
