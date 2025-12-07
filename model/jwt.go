package model

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the JWT claims structure (ready for SSO)
type Claims struct {
	UserID     int       `json:"user_id"`
	Email      string    `json:"email"`
	Role       string    `json:"role"`
	Provider   string    `json:"provider.go"` // "local", "google", "yandex", "azure", etc
	ProviderID string    `json:"provider_id"` // sub claim from SSO provider.go
	AuthTime   time.Time `json:"auth_time"`   // when user authenticated
	AMR        []string  `json:"amr"`         // authentication methods: pwd, otp, mfa, federated
	jwt.RegisteredClaims
}

type NewClaimsParams struct {
	UserID     int
	Email      string
	Role       string
	Provider   string
	ProviderID string
	ExpiresAt  time.Time
	AMR        []string
	AuthTime   time.Time
}

// NewClaims creates a new Claims instance with the provided parameters.
func NewClaims(params NewClaimsParams) *Claims {
	return &Claims{
		UserID:     params.UserID,
		Email:      params.Email,
		Role:       params.Role,
		Provider:   params.Provider,
		ProviderID: params.ProviderID,
		AMR:        params.AMR,
		AuthTime:   params.AuthTime,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   params.ProviderID,
			Audience:  []string{"corpord-web"},
			Issuer:    "corpord-api",
			ExpiresAt: jwt.NewNumericDate(params.ExpiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
}
