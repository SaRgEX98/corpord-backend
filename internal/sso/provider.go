package sso

import "golang.org/x/net/context"

type Provider interface {
	// URL куда отправляем пользователя для авторизации
	AuthURL(state string) string

	// обмен кода на токен
	Exchange(ctx context.Context, code string) (Token, error)

	// получение инфо о пользователе
	GetUserInfo(ctx context.Context, token Token) (*UserInfo, error)
}
