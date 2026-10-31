package sso

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
)

type YandexProvider struct {
	cfg         *oauth2.Config
	client      *http.Client
	userInfoURL string
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
		client:      providerHTTPClient(),
		userInfoURL: "https://login.yandex.ru/info?format=json",
	}
}

func (p *YandexProvider) Name() string {
	return "yandex"
}

// Ссылка на авторизацию
func (p *YandexProvider) AuthURL(state, codeVerifier string) string {
	return p.cfg.AuthCodeURL(state, oauth2.S256ChallengeOption(codeVerifier))
}

// Обмен code -> token
func (p *YandexProvider) Exchange(ctx context.Context, code, codeVerifier string) (*oauth2.Token, error) {
	if code == "" || codeVerifier == "" {
		return nil, errors.New("missing authorization code or PKCE verifier")
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.client)
	tok, err := p.cfg.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		return nil, errors.New("yandex code exchange failed")
	}
	return tok, nil
}

// Запрос user info
func (p *YandexProvider) GetUserInfo(ctx context.Context, tok *oauth2.Token) (*UserInfo, error) {
	if tok == nil || tok.AccessToken == "" {
		return nil, errors.New("missing provider access token")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", p.userInfoURL, nil)
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
	if err := decodeUserInfo(resp, &data); err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}

	name := data.RealName
	if name == "" {
		name = strings.TrimSpace(data.FirstName + " " + data.LastName)
	}

	info := &UserInfo{
		Provider:   "yandex",
		ProviderID: data.ID,
		Email:      data.Email,
		Name:       name,
	}
	if err := ValidateUserInfo(info); err != nil {
		return nil, err
	}
	return info, nil
}
