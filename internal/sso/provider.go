package sso

import (
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

type Provider interface {
	// URL куда отправляем пользователя для авторизации
	AuthURL(state string) string

	// обмен кода на токен
	Exchange(ctx context.Context, code string) (*oauth2.Token, error)

	// получение инфо о пользователе
	GetUserInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error)
}
