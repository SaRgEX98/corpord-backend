package model

import (
	"time"

	"github.com/google/uuid"
)

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}
type TokenResponse struct {
	AccessToken string `json:"access_token"`
}

type RefreshSession struct {
	ID        uuid.UUID `db:"id"`
	UserID    int       `db:"user_id"`
	TokenHash string    `db:"token_hash"`
	UserAgent string    `db:"user_agent"`
	IP        string    `db:"ip"`
	Revoked   bool      `db:"revoked"`
	ExpiresAt time.Time `db:"expires_at"`
	CreatedAt time.Time `db:"created_at"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}
