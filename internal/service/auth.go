package service

import (
	"context"
	"corpord-api/internal/sso"
	"crypto/sha256"
	"encoding/hex"
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
	SSOLogin(ctx context.Context, provider, providerID, email, name, userAgent, ip string) (*model.TokenPair, error)
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

// генерирует Refresh Token и сохраняет сессию
func (s *auth) generateRefreshTokenAndSession(
	ctx context.Context,
	userID int,
	userAgent, ip string,
) (string, error) {
	if userAgent == "" {
		userAgent = "unknown"
	}
	if ip == "" {
		ip = "0.0.0.0"
	}

	raw, hashBytes, err := s.token.GenerateRefreshToken()
	if err != nil {
		return "", err
	}

	id, err := uuid.NewV7()
	if err != nil {
		s.logger.Errorf("generateRefreshTokenAndSession: failed to generate refresh token id: %v", err)
		return "", err
	}
	session := &model.RefreshSession{
		ID:        id,
		UserID:    userID,
		TokenHash: hex.EncodeToString(hashBytes),
		UserAgent: userAgent,
		IP:        ip,
		ExpiresAt: time.Now().Add(s.token.RefreshTTL()),
	}

	if err = s.refreshRepo.Save(ctx, session); err != nil {
		return "", err
	}

	return raw, nil
}

// выдает полный комплект токенов
func (s *auth) issueTokens(
	ctx context.Context,
	u *model.UserDB,
	userAgent, ip, amr string,
) (*model.TokenPair, error) {

	access, err := s.generateAccessToken(u, amr)
	if err != nil {
		return nil, err
	}

	refresh, err := s.generateRefreshTokenAndSession(ctx, u.ID, userAgent, ip)
	if err != nil {
		return nil, err
	}

	return &model.TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
	}, nil
}

// Register создает нового пользователя
func (s *auth) Register(ctx context.Context, input *model.UserCreate, userAgent, ip string) (*model.TokenPair, error) {
	s.logger.Info("Register user", "email", input.Email)

	existing, err := s.authRepo.GetUserByEmail(ctx, input.Email)
	if err != nil {
		return nil, ErrInvalidPass
	}
	if existing != nil {
		return nil, ErrEmailExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(*input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, ErrInvalidPass
	}
	*input.Password = string(hash)

	uid, err := s.authRepo.CreateUser(ctx, input)
	if err != nil {
		return nil, ErrInvalidPass
	}

	u, err := s.authRepo.GetUserByID(ctx, uid)
	if err != nil {
		return nil, ErrUserNotFound
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
	if err != nil || u == nil {
		return nil, ErrInvalidCredentials
	}

	if err = bcrypt.CompareHashAndPassword([]byte(*u.PasswordHash), []byte(credentials.Password)); err != nil {
		return nil, ErrInvalidPass
	}

	u.Provider = "local"
	u.ProviderID = fmt.Sprintf("local:%d", u.ID)

	return s.issueTokens(ctx, u, userAgent, ip, "pwd")
}

// ValidateToken проверяет JWT
func (s *auth) ValidateToken(tokenString string) (int, error) {
	claims, err := s.token.Validate(tokenString)
	if err != nil {
		return 0, ErrInvalidCredentials
	}

	u, err := s.authRepo.GetUserByID(context.Background(), claims.UserID)
	if err != nil || u == nil {
		return 0, ErrUserNotFound
	}

	return claims.UserID, nil
}

// Refresh обновляет токены
func (s *auth) Refresh(
	ctx context.Context,
	rawRefreshToken, userAgent, ip string,
) (*model.TokenPair, error) {

	hash := sha256.Sum256([]byte(rawRefreshToken))
	hashHex := hex.EncodeToString(hash[:])

	// 1. Найти сессию
	session, err := s.refreshRepo.FindByHash(ctx, hashHex)
	if err != nil {
		return nil, ErrInvalidRefreshToken
	}

	// 2. Проверить срок действия
	if time.Now().After(session.ExpiresAt) {
		_ = s.refreshRepo.Revoke(ctx, session.ID)
		return nil, ErrRefreshTokenExpired
	}

	// 3. Получить пользователя
	u, err := s.authRepo.GetUserByID(ctx, session.UserID)
	if err != nil || u == nil {
		_ = s.refreshRepo.Revoke(ctx, session.ID)
		return nil, ErrInvalidRefreshToken
	}

	// 4. Сгенерировать новую пару токенов
	newTokens, err := s.issueTokens(ctx, u, userAgent, ip, "refresh")
	if err != nil {
		return nil, ErrInvalidRefreshToken
	}
	id, err := uuid.NewV7()
	if err != nil {
		s.logger.Errorf("generateRefreshTokenAndSession: failed to generate refresh token id: %v", err)
		return nil, ErrInvalidRefreshToken
	}
	newHash := sha256.Sum256([]byte(newTokens.RefreshToken))
	newHashHex := hex.EncodeToString(newHash[:])
	// 5. Обновить refresh токен в репозитории транзакционно
	newSession := &model.RefreshSession{
		ID:        id,
		UserID:    u.ID,
		TokenHash: newHashHex,
		ExpiresAt: time.Now().Add(s.token.RefreshTTL()),
		IP:        ip,
		UserAgent: userAgent,
	}
	if err := s.refreshRepo.RefreshToken(ctx, hashHex, newSession); err != nil {
		return nil, err
	}

	_ = s.refreshRepo.CleanupExpired(ctx)

	// 6. Вернуть raw токен клиенту
	return newTokens, nil
}

// Logout отзывает один конкретный refresh токен
func (s *auth) Logout(ctx context.Context, rawRefreshToken string) error {
	hash := sha256.Sum256([]byte(rawRefreshToken))
	hashHex := hex.EncodeToString(hash[:])

	session, err := s.refreshRepo.FindByHash(ctx, hashHex)
	if err != nil {
		return ErrInvalidRefreshToken
	}

	return s.refreshRepo.Revoke(ctx, session.ID)
}

// LogoutAll отзывает все токены пользователя
func (s *auth) LogoutAll(ctx context.Context, userID int) error {
	return s.refreshRepo.RevokeAllByUser(ctx, userID)
}
