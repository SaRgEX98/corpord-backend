package sso

import (
	"context"

	"golang.org/x/oauth2"
)

type Provider interface {
	// URL куда отправляем пользователя для авторизации
	AuthURL(state, codeVerifier string) string

	// обмен кода на токен
	Exchange(ctx context.Context, code, codeVerifier string) (*oauth2.Token, error)

	// получение инфо о пользователе
	GetUserInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error)
}
