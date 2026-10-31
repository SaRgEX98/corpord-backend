package service

import (
	"context"
	"corpord-api/internal/repository/pg"
	"corpord-api/internal/sso"
	"corpord-api/model"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

func (s *auth) SSOLogin(ctx context.Context, providerName, code, codeVerifier, userAgent, ip string) (*model.TokenPair, error) {
	ctx, cancel := context.WithTimeout(ctx, sso.LoginTimeout)
	defer cancel()
	p, err := s.sso.Get(providerName)
	if err != nil {
		return nil, ErrProviderNotSupported
	}
	tok, err := p.Exchange(ctx, code, codeVerifier)
	if err != nil {
		return nil, fmt.Errorf("exchange failed: %w", err)
	}
	info, err := p.GetUserInfo(ctx, tok)
	if err != nil {
		return nil, fmt.Errorf("get userinfo failed: %w", err)
	}
	if err := sso.ValidateUserInfo(info); err != nil {
		return nil, err
	}
	if info.Provider != providerName {
		return nil, errors.New("provider identity mismatch")
	}
	u, err := s.findOrCreateUserFromSSO(ctx, info)
	if err != nil {
		return nil, err
	}
	return s.finalizeSSOLogin(ctx, u, info.Provider, info.ProviderID, userAgent, ip)
}

// An email match is not proof of account ownership. Existing accounts require
// their original login method; explicit identity linking is a separate flow.
func (s *auth) findOrCreateUserFromSSO(ctx context.Context, info *sso.UserInfo) (*model.UserDB, error) {
	u, err := s.findUserByIdentity(ctx, info)
	if err != nil || u != nil {
		return u, err
	}
	existing, err := s.authRepo.GetUserByEmail(ctx, info.Email)
	if err != nil {
		return nil, fmt.Errorf("lookup SSO email: %w", err)
	}
	if existing != nil {
		// Another callback may have committed this identity between the first
		// identity lookup and the email lookup. Authorize only by that identity.
		found, lookupErr := s.findUserByIdentity(ctx, info)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if found != nil {
			return found, nil
		}
		return nil, ErrSSOAccountExists
	}
	name := strings.TrimSpace(info.Name)
	if name == "" {
		name = info.Email
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate identity ID: %w", err)
	}
	u, err = s.userIdentity.CreateUserWithIdentity(ctx, &model.UserCreate{Email: info.Email, Name: name}, &model.UserIdentity{
		ID: id, Provider: info.Provider, ProviderID: info.ProviderID,
	})
	if pg.IsPgError(err, pg.ErrorCodeUniqueViolation) {
		// A concurrent callback may have created this exact identity. The failed
		// transaction rolled its user back, so no orphaned account remains.
		found, lookupErr := s.findUserByIdentity(ctx, info)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if found != nil {
			return found, nil
		}
		return nil, ErrSSOAccountExists
	}
	if err != nil {
		return nil, fmt.Errorf("create SSO user and identity: %w", err)
	}
	return u, nil
}

func (s *auth) findUserByIdentity(ctx context.Context, info *sso.UserInfo) (*model.UserDB, error) {
	identity, err := s.userIdentity.GetProviderByID(ctx, info.Provider, info.ProviderID)
	if err != nil && !errors.Is(err, pg.ErrIdentityNotFound) {
		return nil, fmt.Errorf("lookup identity: %w", err)
	}
	if identity == nil {
		return nil, nil
	}
	u, err := s.authRepo.GetUserByID(ctx, identity.UserID)
	if err != nil {
		return nil, fmt.Errorf("lookup identity user: %w", err)
	}
	if u == nil {
		return nil, ErrUserNotFound
	}
	return u, nil
}

func (s *auth) finalizeSSOLogin(ctx context.Context, u *model.UserDB, provider, providerID, userAgent, ip string) (*model.TokenPair, error) {
	if u == nil {
		return nil, ErrUserNotFound
	}
	u.Provider = provider
	u.ProviderID = providerID
	tokens, err := s.issueTokens(ctx, u, userAgent, ip, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to issue tokens: %w", err)
	}
	return tokens, nil
}
