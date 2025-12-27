package model

type SSOLoginRequest struct {
	Provider   string `json:"provider" binding:"required"`
	ProviderID string `json:"provider_id" binding:"required"`
	Email      string `json:"email" binding:"required,email"`
	Name       string `json:"name" binding:"required"`
}
