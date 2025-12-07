package token

import (
	"corpord-api/internal/config"
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
	AccessTTL() time.Duration
	RefreshTTL() time.Duration
}

type manager struct {
	cfg *config.JWT
}

// Parameters for generating JWT (supports SSO)
type GenerateParams struct {
	UserID     int
	Email      string
	Role       string
	Provider   string
	ProviderID string
	AMR        []string
	AuthTime   time.Time
}

// Create token manager
func NewManager(cfg *config.JWT) Manager {
	m := &manager{cfg: cfg}
	m.mapSigningMethod()
	return m
}

// Generate new JWT access token
func (m *manager) Generate(params GenerateParams) (string, error) {
	expiresAt := time.Now().Add(m.cfg.AccessTokenTTL)

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

	token := jwt.NewWithClaims(m.mapSigningMethod(), claims)
	return token.SignedString([]byte(m.cfg.Secret))
}

// Generate refresh token and its hash
func (m *manager) GenerateRefreshToken() (string, []byte, error) {
	b := make([]byte, 32) // 256-bit token
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}

	token := base64.RawURLEncoding.EncodeToString(b)
	hash := sha256.Sum256([]byte(token))

	return token, hash[:], nil
}

// Validate JWT and return claims
func (m *manager) Validate(tokenString string) (*model.Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&model.Claims{},
		func(token *jwt.Token) (interface{}, error) {

			// Проверка алгоритма
			if token.Method.Alg() != m.cfg.SigningAlgorithm {
				return nil, errors.New("unexpected signing method: " + token.Method.Alg())
			}

			return []byte(m.cfg.Secret), nil
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

// Token TTL getters
func (m *manager) AccessTTL() time.Duration  { return m.cfg.AccessTokenTTL }
func (m *manager) RefreshTTL() time.Duration { return m.cfg.RefreshTokenTTL }

// Map algorithm from config to JWT signing method
func (m *manager) mapSigningMethod() jwt.SigningMethod {
	switch m.cfg.SigningAlgorithm {
	case "HS256":
		return jwt.SigningMethodHS256
	case "HS384":
		return jwt.SigningMethodHS384
	case "HS512":
		return jwt.SigningMethodHS512
	default:
		panic("unsupported signing algorithm: " + m.cfg.SigningAlgorithm)
	}
}
