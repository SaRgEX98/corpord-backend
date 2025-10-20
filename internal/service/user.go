package service

import (
	"context"
	"errors"
	"time"

	"corpord-api/internal/logger"
	"corpord-api/internal/repository/pg"
	"corpord-api/model"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailExists        = errors.New("email already exists")
)

type User interface {
	GetAll(ctx context.Context) ([]*model.UserResponse, error)
	GetByID(ctx context.Context, id int) (*model.UserResponse, error)
	GetByEmail(ctx context.Context, email string) (*model.UserDB, error)
	Create(ctx context.Context, user *model.UserCreate) (*model.UserResponse, error)
	Update(ctx context.Context, id int, update *model.UserUpdate) (*model.UserResponse, error)
	Delete(ctx context.Context, id int) error
}

type user struct {
	logger *logger.Logger
	r      pg.UserRepository
}

func NewUser(logger *logger.Logger, r pg.UserRepository) User {
	return &user{
		logger: logger,
		r:      r,
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
