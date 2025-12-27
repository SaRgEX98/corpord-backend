package service

import (
	"context"
	"corpord-api/internal/repository/pg"
	"corpord-api/internal/sso"
	"corpord-api/model"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// SSOLogin — оркеструет вход через SSO-провайдера:
// 1) получает провайдера из реестра
// 2) делает обмен (exchange) и получает userinfo
// 3) находит или создаёт пользователя и identity
// 4) финализирует — выдаёт токены
func (s *auth) SSOLogin(
	ctx context.Context,
	providerName, providerCodeOrID, email, name, userAgent, ip string,
) (*model.TokenPair, error) {

	// 1) получить провайдера из реестра
	p, err := s.sso.Get(providerName)
	if err != nil {
		return nil, ErrProviderNotSupported
	}

	// 2) exchange (code -> token). В зависимости от реализации провайдера
	// providerCodeOrID обычно — auth code из callback.
	tok, err := p.Exchange(ctx, providerCodeOrID)
	if err != nil {
		return nil, fmt.Errorf("exchange failed: %w", err)
	}

	// 3) получить user info
	info, err := p.GetUserInfo(ctx, tok)
	if err != nil {
		return nil, fmt.Errorf("get userinfo failed: %w", err)
	}

	// 4) найти или создать пользователя + identity
	u, err := s.findOrCreateUserFromSSO(ctx, info, email, name)
	if err != nil {
		return nil, err
	}

	// 5) финализировать: пометить provider и providerID в user (для claims) и выдать токены
	return s.finalizeSSOLogin(ctx, u, info.Provider, info.ProviderID, userAgent, ip)
}

// findOrCreateUserFromSSO ищет user по identity.provider+provider_id,
// если не находит — пытается найти по email и привязать identity,
// если и по email нет — создаёт нового пользователя и identity.
//
// Параметр email/name — это данные, которые мы могли получить у провайдера или из фронта.
// Если info.Email пустой, используем переданный email (если есть).
func (s *auth) findOrCreateUserFromSSO(
	ctx context.Context,
	info *sso.UserInfo,
	fallbackEmail, fallbackName string,
) (*model.UserDB, error) {

	// 1) Попробовать найти identity по provider+provider_id
	identity, err := s.userIdentity.GetProviderByID(ctx, info.Provider, info.ProviderID)
	if err != nil && !errors.Is(err, pg.ErrIdentityNotFound) {
		return nil, fmt.Errorf("failed to lookup identity: %w", err)
	}

	if identity != nil {
		// identity нашлась — вернуть пользователя
		u, err := s.authRepo.GetUserByID(ctx, identity.UserID)
		if err != nil || u == nil {
			return nil, ErrUserNotFound
		}
		return u, nil
	}

	// 2) identity не найдена — попробуем найти пользователя по email
	emailToUse := info.Email
	if emailToUse == "" {
		emailToUse = fallbackEmail
	}

	if emailToUse != "" {
		u, err := s.authRepo.GetUserByEmail(ctx, emailToUse)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup user by email: %w", err)
		}
		if u != nil {
			// пользователь найден по email → привязать identity
			uid, err := uuid.NewV7()
			if err != nil {
				s.logger.Errorf("failed to generate uuid: %v", err)
				return nil, fmt.Errorf("failed to generate uuid: %w", err)
			}
			newIdentity := &model.UserIdentity{
				ID:         uid,
				UserID:     u.ID,
				Provider:   info.Provider,
				ProviderID: info.ProviderID,
			}
			if err := s.userIdentity.AddIdentity(ctx, newIdentity); err != nil {
				return nil, fmt.Errorf("failed to add identity: %w", err)
			}
			return u, nil
		}
	}

	// 3) пользователь не найден по email → создаём нового
	nameToUse := info.Name
	if nameToUse == "" {
		nameToUse = fallbackName
	}

	userCreate := &model.UserCreate{
		Email:    emailToUse,
		Name:     nameToUse,
		Password: nil, // для SSO ставим пустой пароль; в БД password_hash может быть NULL
	}

	uid, err := s.authRepo.CreateUser(ctx, userCreate)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// создать identity для нового пользователя
	id, err := uuid.NewV7()
	if err != nil {
		s.logger.Errorf("failed to generate uuid: %v", err)
		return nil, fmt.Errorf("failed to generate uuid: %w", err)
	}
	newIdentity := &model.UserIdentity{
		ID:         id,
		UserID:     uid,
		Provider:   info.Provider,
		ProviderID: info.ProviderID,
	}
	if err := s.userIdentity.AddIdentity(ctx, newIdentity); err != nil {
		return nil, fmt.Errorf("failed to add identity for new user: %w", err)
	}

	// вернуть только-что созданного пользователя
	created, err := s.authRepo.GetUserByID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created user: %w", err)
	}
	return created, nil
}

// finalizeSSOLogin помечает пользователя как вошедшего через provider/providerID
// и выдаёт комплект токенов (access + refresh) через s.issueTokens
func (s *auth) finalizeSSOLogin(
	ctx context.Context,
	u *model.UserDB,
	provider, providerID, userAgent, ip string,
) (*model.TokenPair, error) {

	// Помещаем провайдерные данные в user (они попадут в claims)
	u.Provider = provider
	u.ProviderID = providerID

	// issueTokens выдаёт access и refresh (и создаёт refresh session)
	tokens, err := s.issueTokens(ctx, u, userAgent, ip, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to issue tokens: %w", err)
	}
	return tokens, nil
}
