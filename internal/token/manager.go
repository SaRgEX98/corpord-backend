package token

import (
	"corpord-api/model"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Manager interface {
	Generate(params GenerateParams) (string, error)
	GenerateRefreshToken() (string, []byte, error)
	Validate(tokenString string) (*model.Claims, error)
}

type manager struct {
	secretKey     string
	signingMethod jwt.SigningMethod
	tokenTTL      time.Duration
}

// GenerateParams — входные параметры для создания токена
type GenerateParams struct {
	UserID     int
	Email      string
	Role       string
	Provider   string
	ProviderID string
	AMR        []string
	AuthTime   time.Time
}

// NewManager creates a new token manager
func NewManager(secretKey string, tokenTTL time.Duration) Manager {
	return &manager{
		secretKey:     secretKey,
		signingMethod: jwt.SigningMethodHS256,
		tokenTTL:      tokenTTL,
	}
}

// Generate creates a new JWT token (supports SSO)
func (m *manager) Generate(params GenerateParams) (string, error) {
	expiresAt := time.Now().Add(m.tokenTTL)

	claims := model.NewClaims(model.NewClaimsParams{
		UserID:     params.UserID,
		Email:      params.Email,
		Role:       params.Role,
		Provider:   params.Provider,
		ProviderID: params.ProviderID,
		ExpiresAt:  expiresAt,
		AMR:        params.AMR,
		AuthTime:   params.AuthTime,
	})

	token := jwt.NewWithClaims(m.signingMethod, claims)
	return token.SignedString([]byte(m.secretKey))
}

func (m *manager) GenerateRefreshToken() (string, []byte, error) {
	b := make([]byte, 32) // 256bit
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}

	token := base64.RawURLEncoding.EncodeToString(b)
	hash := sha256.Sum256([]byte(token))

	return token, hash[:], nil
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

	claims, ok := token.Claims.(*model.Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}
