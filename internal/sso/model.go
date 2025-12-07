package sso

type Token interface {
	AccessToken() string
}

// UserInfo — всё, что нужно твоей системе.
type UserInfo struct {
	Provider   string
	ProviderID string
	Email      string
	Name       string
}
