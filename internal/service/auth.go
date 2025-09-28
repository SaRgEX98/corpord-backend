package service

import (
	"context"
	"corpord-api/internal/logger"
	"corpord-api/internal/repository/pg"
	"corpord-api/internal/token"
	"corpord-api/model"
	"errors"
	"golang.org/x/crypto/bcrypt"
)

type Auth interface {
	// Register creates a new user account
	Register(ctx context.Context, user *model.UserCreate) (*model.UserResponse, error)
	// Login authenticates a user and returns a JWT token
	Login(ctx context.Context, credentials model.UserLogin) (string, error)
	// ValidateToken validates a JWT token and returns the user ID
	ValidateToken(tokenString string) (int, error)
}

type auth struct {
	token    token.Manager
	logger   *logger.Logger
	authRepo pg.AuthRepository
}

// NewAuth creates a new auth service
func NewAuth(logger *logger.Logger, token token.Manager, authRepo pg.AuthRepository) Auth {
	return &auth{
		token:    token,
		logger:   logger,
		authRepo: authRepo,
	}
}

// Register creates a new user account with default user role
func (s *auth) Register(ctx context.Context, input *model.UserCreate) (*model.UserResponse, error) {
	s.logger.Info("Registering new user", "email", input.Email)

	existing, err := s.authRepo.GetUserByEmail(ctx, input.Email)
	if err != nil {
		s.logger.Error("Error checking existing user", "error", err, "email", input.Email)
		return nil, errors.New("ошибка при проверке существующего пользователя")
	}

	if existing != nil {
		s.logger.Warn("User already exists", "email", input.Email)
		return nil, errors.New("пользователь с таким email уже существует")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("Failed to hash password", "error", err)
		return nil, errors.New("ошибка при создании пользователя")
	}

	input.Password = string(hashedPassword)

	userID, err := s.authRepo.CreateUser(ctx, input)
	if err != nil {
		s.logger.Error("Failed to create user", "error", err, "email", input.Email)
		return nil, errors.New("не удалось создать пользователя")
	}

	createdUser, err := s.authRepo.GetUserByID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to fetch created user", "error", err, "user_id", userID)
		return nil, errors.New("пользователь создан, но не удалось получить его данные")
	}

	response := &model.UserResponse{
		ID:        createdUser.ID,
		Email:     createdUser.Email,
		Name:      createdUser.Name,
		CreatedAt: createdUser.CreatedAt,
		UpdatedAt: createdUser.UpdatedAt,
	}

	s.logger.Info("User registered successfully", "user_id", userID, "email", input.Email)
	return response, nil
}

// Login authenticates a user and returns a JWT token
func (s *auth) Login(ctx context.Context, credentials model.UserLogin) (string, error) {
	s.logger.Info("Login attempt", "email", credentials.Email)

	if credentials.Email == "" || credentials.Password == "" {
		s.logger.Warn("Login attempt with empty email or password")
		return "", errors.New("email и пароль обязательны")
	}

	u, err := s.authRepo.GetUserByEmail(ctx, credentials.Email)
	if err != nil || u == nil {
		s.logger.Warn("User not found", "email", credentials.Email)
		return "", errors.New("неверный email или пароль")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(credentials.Password)); err != nil {
		s.logger.Warn("Invalid password", "email", credentials.Email)
		return "", errors.New("неверный email или пароль")
	}

	t, err := s.token.Generate(u.ID, u.Email, u.Role)
	if err != nil {
		s.logger.Error("Failed to generate token", "error", err, "user_id", u.ID)
		return "", errors.New("ошибка при аутентификации")
	}

	s.logger.Info("User logged in successfully", "user_id", u.ID, "email", u.Email)
	return t, nil
}

// ValidateToken validates a JWT token and returns the user ID
func (s *auth) ValidateToken(tokenString string) (int, error) {
	claims, err := s.token.Validate(tokenString)
	if err != nil {
		s.logger.Warn("Token validation failed", "error", err)
		return 0, errors.New("недействительный токен")
	}

	u, err := s.authRepo.GetUserByID(context.Background(), claims.UserID)
	if err != nil || u == nil {
		s.logger.Warn("User from token not found", "user_id", claims.UserID)
		return 0, errors.New("пользователь не найден")
	}

	return claims.UserID, nil
}
