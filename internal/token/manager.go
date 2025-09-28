package token

import (
	"corpord-api/model"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Manager provides token operations
type Manager interface {
	Generate(userID int, email string, role string) (string, error)
	Validate(tokenString string) (*model.Claims, error)
}

type manager struct {
	secretKey     string
	signingMethod jwt.SigningMethod
	tokenTTL      time.Duration
}

// NewManager creates a new token manager
func NewManager(secretKey string, tokenTTL time.Duration) Manager {
	return &manager{
		secretKey:     secretKey,
		signingMethod: jwt.SigningMethodHS256,
		tokenTTL:      tokenTTL,
	}
}

// Generate creates a new JWT token with the given user details and role
func (m *manager) Generate(userID int, email string, role string) (string, error) {
	expiresAt := time.Now().Add(m.tokenTTL)

	claims := model.NewClaims(model.NewClaimsParams{
		UserID:    userID,
		Role:      role,
		ExpiresAt: expiresAt,
	})

	claims.Email = email

	token := jwt.NewWithClaims(m.signingMethod, claims)
	return token.SignedString([]byte(m.secretKey))
}

// Validate verifies the token and returns the claims
func (m *manager) Validate(tokenString string) (*model.Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&model.Claims{},
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return []byte(m.secretKey), nil
		},
	)

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*model.Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
