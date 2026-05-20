package sso

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
)

type YandexProvider struct {
	cfg    *oauth2.Config
	client *http.Client
}

func NewYandexProvider(clientID, clientSecret, redirectURL string) Provider {
	return &YandexProvider{
		cfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"login:info", "login:email"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://oauth.yandex.ru/authorize",
				TokenURL: "https://oauth.yandex.ru/token",
			},
		},
		client: &http.Client{},
	}
}

func (p *YandexProvider) Name() string {
	return "yandex"
}

// Ссылка на авторизацию
func (p *YandexProvider) AuthURL(state string) string {
	return p.cfg.AuthCodeURL(state)
}

// Обмен code -> token
func (p *YandexProvider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return p.cfg.Exchange(ctx, code)
}

// Запрос user info
func (p *YandexProvider) GetUserInfo(ctx context.Context, tok *oauth2.Token) (*UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://login.yandex.ru/info?format=json", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "OAuth "+tok.AccessToken)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("yandex userinfo failed: %w", err)
	}
	defer resp.Body.Close()

	var data struct {
		ID        string `json:"id"`
		Login     string `json:"login"`
		RealName  string `json:"real_name"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Email     string `json:"default_email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}

	name := data.RealName
	if name == "" {
		name = strings.TrimSpace(data.FirstName + " " + data.LastName)
	}

	return &UserInfo{
		Provider:   "yandex",
		ProviderID: data.ID,
		Email:      data.Email,
		Name:       name,
	}, nil
}
