package service

import (
	"context"
	"errors"
	"time"

	"corpord-api/internal/logger"
	"corpord-api/internal/repository/pg"
	"corpord-api/model"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailExists        = errors.New("email already exists")
)

type User interface {
	// Basic CRUD operations
	GetAll(ctx context.Context) ([]*model.UserResponse, error)
	GetByID(ctx context.Context, id int) (*model.UserResponse, error)
	GetByEmail(ctx context.Context, email string) (*model.UserDB, error)
	Create(ctx context.Context, user *model.UserCreate) (*model.UserResponse, error)
	Update(ctx context.Context, id int, update *model.UserUpdate) (*model.UserResponse, error)
	Delete(ctx context.Context, id int) error

	// Authentication
	Login(ctx context.Context, credentials model.UserLogin) (string, error)
	ValidateToken(tokenString string) (int, error)
}

type user struct {
	logger    *logger.Logger
	r         pg.UserRepository
	jwtSecret string
}

func NewUser(logger *logger.Logger, r pg.UserRepository, jwtSecret string) User {
	return &user{
		logger:    logger,
		r:         r,
		jwtSecret: jwtSecret,
	}
}

// GetAll returns all users
func (s *user) GetAll(ctx context.Context) ([]*model.UserResponse, error) {
	responses, err := s.r.GetAll(ctx)
	if err != nil {
		s.logger.Errorf("failed to get all users: %v", err)
		return nil, err
	}

	return responses, nil
}

// GetByID returns a user by ID
func (s *user) GetByID(ctx context.Context, id int) (*model.UserResponse, error) {
	resp, err := s.r.GetByID(ctx, id)
	if err != nil {
		s.logger.Errorf("failed to get user by id %d: %v", id, err)
		return nil, ErrUserNotFound
	}

	return resp, nil
}

// GetByEmail returns a user by email (internal use only)
func (s *user) GetByEmail(ctx context.Context, email string) (*model.UserDB, error) {
	if email == "" {
		return nil, errors.New("email cannot be empty")
	}

	u, err := s.r.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pg.ErrNotFound) {
			s.logger.Debugf("user with email %s not found", email)
			return nil, ErrUserNotFound
		}
		s.logger.Errorf("failed to get user by email %s: %v", email, err)
		return nil, err
	}

	if u == nil {
		s.logger.Debugf("user with email %s not found (nil response)", email)
		return nil, ErrUserNotFound
	}

	return u, nil
}

// Create creates a new user
func (s *user) Create(ctx context.Context, userCreate *model.UserCreate) (*model.UserResponse, error) {
	// Check if email already exists
	existingUser, err := s.r.GetByEmail(ctx, userCreate.Email)
	if err == nil && existingUser != nil {
		s.logger.Warnf("user with email %s already exists", userCreate.Email)
		return nil, ErrEmailExists
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(userCreate.Password), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Errorf("failed to hash password: %v", err)
		return nil, err
	}

	// Create user in database
	userDB := &model.UserDB{
		Email:        userCreate.Email,
		PasswordHash: string(hashedPassword),
		Name:         userCreate.Name,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Convert to UserCreate to pass to repository
	userToCreate := &model.UserCreate{
		Email:    userDB.Email,
		Password: userDB.PasswordHash,
		Name:     userDB.Name,
	}

	createdUser, err := s.r.Create(ctx, userToCreate)
	if err != nil {
		s.logger.Errorf("failed to create user: %v", err)
		return nil, err
	}

	return createdUser, nil
}

// Update updates a user
func (s *user) Update(ctx context.Context, id int, update *model.UserUpdate) (*model.UserResponse, error) {
	// Validate update data
	if err := update.Validate(); err != nil {
		return nil, err
	}

	// If password is being updated, hash the new password
	if update.Password != nil {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*update.Password), bcrypt.DefaultCost)
		if err != nil {
			s.logger.Errorf("failed to hash password: %v", err)
			return nil, err
		}
		hashedPassStr := string(hashedPassword)
		update.Password = &hashedPassStr
	}

	updatedUser, err := s.r.Update(ctx, id, update)
	if err != nil {
		s.logger.Errorf("failed to update user %d: %v", id, err)
		return nil, err
	}

	return updatedUser, nil
}

// Delete deletes a user
func (s *user) Delete(ctx context.Context, id int) error {
	if err := s.r.Delete(ctx, id); err != nil {
		s.logger.Errorf("failed to delete user %d: %v", id, err)
		return err
	}
	return nil
}

// Login authenticates a user and returns a JWT token
func (s *user) Login(ctx context.Context, credentials model.UserLogin) (string, error) {
	if credentials.Email == "" || credentials.Password == "" {
		s.logger.Warn("login attempt with empty email or password")
		return "", ErrInvalidCredentials
	}

	user, err := s.GetByEmail(ctx, credentials.Email)
	if err != nil {
		s.logger.Warnf("login failed for email %s: %v", credentials.Email, err)
		return "", ErrInvalidCredentials
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(credentials.Password)); err != nil {
		s.logger.Warnf("invalid password for user %s", credentials.Email)
		return "", ErrInvalidCredentials
	}

	// Generate JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"email":   user.Email,
		"exp":     time.Now().Add(time.Hour * 24).Unix(), // Token expires in 24 hours
	})

	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		s.logger.Errorf("failed to generate token: %v", err)
		return "", err
	}

	s.logger.Infof("successful login for user %d", user.ID)
	return tokenString, nil
}

// ValidateToken validates a JWT token and returns the user ID
func (s *user) ValidateToken(tokenString string) (int, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(s.jwtSecret), nil
	})

	if err != nil {
		return 0, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userID := int(claims["user_id"].(float64))
		return userID, nil
	}

	return 0, jwt.ErrInvalidKey
}
