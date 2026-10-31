package sso

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleProvider struct {
	config      *oauth2.Config
	client      *http.Client
	userInfoURL string
}

// конструктор провайдера
func NewGoogleProvider(clientID, clientSecret, redirectURL string) Provider {
	return &GoogleProvider{
		client:      providerHTTPClient(),
		userInfoURL: "https://www.googleapis.com/oauth2/v3/userinfo",
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes: []string{
				"email",
				"profile",
				"openid",
			},
			Endpoint: google.Endpoint,
		},
	}
}

// формирует URL куда пользователь уйдёт для авторизации
func (g *GoogleProvider) AuthURL(state, codeVerifier string) string {
	return g.config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(codeVerifier))
}

// обмен кода на токен
func (g *GoogleProvider) Exchange(ctx context.Context, code, codeVerifier string) (*oauth2.Token, error) {
	if code == "" || codeVerifier == "" {
		return nil, errors.New("missing authorization code or PKCE verifier")
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, g.client)
	tok, err := g.config.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		// OAuth errors may contain the provider response body. Keep credentials out of logs.
		return nil, errors.New("google code exchange failed")
	}
	return tok, nil
}

// структура ответа Google API
type googleUser struct {
	ID            string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func (g *GoogleProvider) GetUserInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
	if token == nil || token.AccessToken == "" {
		return nil, errors.New("missing provider access token")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", g.userInfoURL, nil)
	if err != nil {
		return nil, errors.New("invalid google userinfo endpoint")
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google request failed: %w", err)
	}
	defer resp.Body.Close()

	var gu googleUser
	if err := decodeUserInfo(resp, &gu); err != nil {
		return nil, fmt.Errorf("decode google userinfo failed: %w", err)
	}

	info := &UserInfo{
		Provider:      "google",
		ProviderID:    gu.ID,
		Email:         gu.Email,
		EmailVerified: gu.EmailVerified,
		Name:          gu.Name,
	}
	if err := ValidateUserInfo(info); err != nil {
		return nil, err
	}
	return info, nil
}

// Config возвращает oauth2 конфигурацию
func (g *GoogleProvider) Config() *oauth2.Config {
	return g.config
}
