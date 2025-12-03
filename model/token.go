package model

import "time"

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type RefreshSession struct {
	ID        string
	UserID    int
	TokenHash string
	UserAgent string
	IP        string
	ExpiresAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}
