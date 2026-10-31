package service

import (
	"context"
	"corpord-api/internal/sso"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"corpord-api/internal/logger"
	"corpord-api/internal/repository/pg"
	"corpord-api/internal/token"
	"corpord-api/model"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Auth interface {
	Register(ctx context.Context, user *model.UserCreate, userAgent, ip string) (*model.TokenPair, error)
	Login(ctx context.Context, credentials model.UserLogin, userAgent, ip string) (*model.TokenPair, error)
	SSOLogin(ctx context.Context, provider, code, codeVerifier, userAgent, ip string) (*model.TokenPair, error)
	Authenticate(ctx context.Context, tokenString string) (*model.Claims, error)
	ValidateToken(tokenString string) (int, error)
	Refresh(ctx context.Context, rawRefreshToken, userAgent, ip string) (*model.TokenPair, error)
	Logout(ctx context.Context, rawRefreshToken string) error
	LogoutAll(ctx context.Context, userID int) error
}

type auth struct {
	token        token.Manager
	logger       *logger.Logger
	authRepo     pg.AuthRepository
	refreshRepo  pg.RefreshTokenRepository
	userIdentity pg.UserIdentitiesRepository
	sso          *sso.Registry
}

func NewAuth(
	logger *logger.Logger,
	token token.Manager,
	authRepo pg.AuthRepository,
	refreshRepo pg.RefreshTokenRepository,
	userIdentity pg.UserIdentitiesRepository,
	sso *sso.Registry,
) Auth {
	return &auth{
		token:        token,
		logger:       logger,
		authRepo:     authRepo,
		refreshRepo:  refreshRepo,
		userIdentity: userIdentity,
		sso:          sso,
	}
}

// генерирует Access Token
func (s *auth) generateAccessToken(u *model.UserDB, amr string) (string, error) {
	return s.token.Generate(token.GenerateParams{
		UserID:     u.ID,
		Email:      u.Email,
		Role:       u.Role,
		Provider:   u.Provider,
		ProviderID: u.ProviderID,
		AMR:        []string{amr},
		AuthTime:   time.Now(),
	})
}

// generateTokens only generates credentials and session metadata. Persistence is
// handled by the caller so refresh can consume and replace a session atomically.
func (s *auth) generateTokens(
	u *model.UserDB,
	userAgent, ip, amr string,
) (*model.TokenPair, *model.RefreshSession, error) {
	access, err := s.generateAccessToken(u, amr)
	if err != nil {
		return nil, nil, err
	}
	refresh, session, err := s.generateRefreshSession(u.ID, userAgent, ip)
	if err != nil {
		return nil, nil, err
	}
	return &model.TokenPair{AccessToken: access, RefreshToken: refresh}, session, nil
}

func (s *auth) generateRefreshSession(
	userID int,
	userAgent, ip string,
) (string, *model.RefreshSession, error) {
	if userAgent == "" {
		userAgent = "unknown"
	}
	if ip == "" {
		ip = "0.0.0.0"
	}

	raw, hashBytes, err := s.token.GenerateRefreshToken()
	if err != nil {
		return "", nil, err
	}

	id, err := uuid.NewV7()
	if err != nil {
		s.logger.Errorf("generateRefreshTokenAndSession: failed to generate refresh token id: %v", err)
		return "", nil, err
	}
	session := &model.RefreshSession{
		ID:        id,
		UserID:    userID,
		TokenHash: hex.EncodeToString(hashBytes),
		UserAgent: userAgent,
		IP:        ip,
		ExpiresAt: time.Now().Add(s.token.RefreshTTL()),
	}

	return raw, session, nil
}

// выдает полный комплект токенов
func (s *auth) issueTokens(
	ctx context.Context,
	u *model.UserDB,
	userAgent, ip, amr string,
) (*model.TokenPair, error) {

	tokens, session, err := s.generateTokens(u, userAgent, ip, amr)
	if err != nil {
		return nil, err
	}

	if err = s.refreshRepo.Save(ctx, session); err != nil {
		return nil, err
	}

	return tokens, nil
}

// Register создает нового пользователя
func (s *auth) Register(ctx context.Context, input *model.UserCreate, userAgent, ip string) (*model.TokenPair, error) {
	if input == nil || input.Password == nil || len(*input.Password) == 0 || len(*input.Password) > 72 {
		return nil, ErrInvalidPass
	}
	s.logger.Info("Register user", "email", input.Email)

	existing, err := s.authRepo.GetUserByEmail(ctx, input.Email)
	if err != nil {
		return nil, fmt.Errorf("lookup registration email: %w", err)
	}
	if existing != nil {
		return nil, ErrEmailExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(*input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, ErrInvalidPass
	}
	hashedPassword := string(hash)
	userToCreate := *input
	userToCreate.Password = &hashedPassword

	uid, err := s.authRepo.CreateUser(ctx, &userToCreate)
	if err != nil {
		if pg.IsPgError(err, pg.ErrorCodeUniqueViolation) {
			return nil, ErrEmailExists
		}
		return nil, fmt.Errorf("create local user: %w", err)
	}

	u, err := s.authRepo.GetUserByID(ctx, uid)
	if err != nil || u == nil {
		return nil, fmt.Errorf("load registered user: %w", ErrUserNotFound)
	}

	u.Provider = "local"
	u.ProviderID = fmt.Sprintf("local:%d", u.ID)

	return s.issueTokens(ctx, u, userAgent, ip, "pwd")
}

// Login по email/password
func (s *auth) Login(ctx context.Context, credentials model.UserLogin, userAgent, ip string) (*model.TokenPair, error) {
	s.logger.Info("Login", "email", credentials.Email)

	if credentials.Email == "" || credentials.Password == "" {
		return nil, ErrNoFields
	}

	u, err := s.authRepo.GetUserByEmail(ctx, credentials.Email)
	if err != nil {
		return nil, fmt.Errorf("lookup login user: %w", err)
	}
	if u == nil || u.DeletedAt != nil || u.PasswordHash == nil {
		return nil, ErrInvalidCredentials
	}

	if err = bcrypt.CompareHashAndPassword([]byte(*u.PasswordHash), []byte(credentials.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	u.Provider = "local"
	u.ProviderID = fmt.Sprintf("local:%d", u.ID)

	return s.issueTokens(ctx, u, userAgent, ip, "pwd")
}

// Authenticate validates the JWT and resolves the current user and role. Deleted
// users and permissions removed since token issuance take effect immediately.
func (s *auth) Authenticate(ctx context.Context, tokenString string) (*model.Claims, error) {
	claims, err := s.token.Validate(tokenString)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if claims.UserID <= 0 {
		return nil, ErrInvalidCredentials
	}
	u, err := s.authRepo.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("authenticate user: %w", err)
	}
	if u == nil || u.DeletedAt != nil {
		return nil, ErrUserNotFound
	}
	claims.Role = u.Role
	claims.Email = u.Email
	return claims, nil
}

// ValidateToken is retained for callers that only require the user ID.
func (s *auth) ValidateToken(tokenString string) (int, error) {
	claims, err := s.Authenticate(context.Background(), tokenString)
	if err != nil {
		return 0, err
	}
	return claims.UserID, nil
}

// Refresh обновляет токены
func (s *auth) Refresh(
	ctx context.Context,
	rawRefreshToken, userAgent, ip string,
) (*model.TokenPair, error) {
	if rawRefreshToken == "" {
		return nil, ErrInvalidRefreshToken
	}

	hash := sha256.Sum256([]byte(rawRefreshToken))
	hashHex := hex.EncodeToString(hash[:])

	// 1. Найти сессию
	session, err := s.refreshRepo.FindByHash(ctx, hashHex)
	if err != nil {
		if errors.Is(err, pg.ErrRefreshTokenNotFound) {
			return nil, ErrInvalidRefreshToken
		}
		return nil, fmt.Errorf("lookup refresh session: %w", err)
	}
	if session == nil || session.Revoked {
		return nil, ErrInvalidRefreshToken
	}

	// 2. Проверить срок действия
	if !time.Now().Before(session.ExpiresAt) {
		_ = s.refreshRepo.Revoke(ctx, session.ID)
		return nil, ErrRefreshTokenExpired
	}

	// 3. Получить пользователя
	u, err := s.authRepo.GetUserByID(ctx, session.UserID)
	if err != nil {
		return nil, fmt.Errorf("load refresh user: %w", err)
	}
	if u == nil || u.DeletedAt != nil {
		_ = s.refreshRepo.Revoke(ctx, session.ID)
		return nil, ErrInvalidRefreshToken
	}

	// 4. Сгенерировать новую пару токенов
	newTokens, newSession, err := s.generateTokens(u, userAgent, ip, "refresh")
	if err != nil {
		return nil, fmt.Errorf("generate refreshed tokens: %w", err)
	}
	// Only the transaction persists the replacement. Its conditional UPDATE is
	// the single point that decides whether the old token may be used.
	if err := s.refreshRepo.RefreshToken(ctx, hashHex, newSession); err != nil {
		if errors.Is(err, pg.ErrRefreshTokenNotFound) {
			return nil, ErrInvalidRefreshToken
		}
		return nil, fmt.Errorf("rotate refresh session: %w", err)
	}

	// 6. Вернуть raw токен клиенту
	return newTokens, nil
}

// Logout отзывает один конкретный refresh токен
func (s *auth) Logout(ctx context.Context, rawRefreshToken string) error {
	hash := sha256.Sum256([]byte(rawRefreshToken))
	hashHex := hex.EncodeToString(hash[:])

	session, err := s.refreshRepo.FindByHash(ctx, hashHex)
	if err != nil {
		if errors.Is(err, pg.ErrRefreshTokenNotFound) {
			return ErrInvalidRefreshToken
		}
		return fmt.Errorf("lookup logout session: %w", err)
	}
	if session == nil {
		return ErrInvalidRefreshToken
	}

	if err := s.refreshRepo.Revoke(ctx, session.ID); err != nil {
		if errors.Is(err, pg.ErrRefreshTokenNotFound) {
			return ErrInvalidRefreshToken
		}
		return fmt.Errorf("revoke logout session: %w", err)
	}
	return nil
}

// LogoutAll отзывает все токены пользователя
func (s *auth) LogoutAll(ctx context.Context, userID int) error {
	if userID <= 0 {
		return ErrInvalidCredentials
	}
	return s.refreshRepo.RevokeAllByUser(ctx, userID)
}
