package sso

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleProvider struct {
	config *oauth2.Config
}

// конструктор провайдера
func NewGoogleProvider(clientID, clientSecret, redirectURL string) Provider {
	return &GoogleProvider{
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
func (g *GoogleProvider) AuthURL(state string) string {
	return g.config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// обмен кода на токен
func (g *GoogleProvider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	tok, err := g.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
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
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google request failed: %w", err)
	}
	defer resp.Body.Close()

	var gu googleUser
	if err := json.NewDecoder(resp.Body).Decode(&gu); err != nil {
		return nil, fmt.Errorf("decode google userinfo failed: %w", err)
	}

	return &UserInfo{
		Provider:   "google",
		ProviderID: gu.ID,
		Email:      gu.Email,
		Name:       gu.Name,
	}, nil
}

// Config возвращает oauth2 конфигурацию
func (g *GoogleProvider) Config() *oauth2.Config {
	return g.config
}
