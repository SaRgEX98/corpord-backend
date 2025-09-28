package model

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the JWT claims structure
type Claims struct {
	UserID int    `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// NewClaims creates a new Claims instance
type NewClaimsParams struct {
	UserID    int
	Role      string
	ExpiresAt time.Time
}

// NewClaims creates a new Claims instance with the provided parameters
func NewClaims(params NewClaimsParams) *Claims {
	return &Claims{
		UserID: params.UserID,
		Role:   params.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(params.ExpiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
}
