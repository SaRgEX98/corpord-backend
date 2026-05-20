package service

import (
	"context"
	"corpord-api/internal/logger"
	"corpord-api/internal/repository/pg"
	"corpord-api/model"
	"errors"

	"golang.org/x/crypto/bcrypt"
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

// GetAll возвращает всех пользователей
func (s *user) GetAll(ctx context.Context) ([]*model.UserResponse, error) {
	responses, err := s.r.GetAll(ctx)
	if err != nil {
		s.logger.Errorf("failed to get all users: %v", err)
		return nil, err
	}
	return responses, nil
}

// GetByID возвращает пользователя по ID
func (s *user) GetByID(ctx context.Context, id int) (*model.UserResponse, error) {
	resp, err := s.r.GetByID(ctx, id)
	if err != nil {
		s.logger.Errorf("failed to get user by id %d: %v", id, err)
		return nil, ErrUserNotFound
	}
	return resp, nil
}

// GetByEmail возвращает пользователя по email (для внутреннего использования)
func (s *user) GetByEmail(ctx context.Context, email string) (*model.UserDB, error) {
	if email == "" {
		return nil, errors.New("email cannot be empty")
	}

	u, err := s.r.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pg.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		s.logger.Errorf("failed to get user by email %s: %v", email, err)
		return nil, err
	}

	if u == nil {
		return nil, ErrUserNotFound
	}

	return u, nil
}

// Create создаёт нового пользователя (локального)
func (s *user) Create(ctx context.Context, userCreate *model.UserCreate) (*model.UserResponse, error) {
	existingUser, err := s.r.GetByEmail(ctx, userCreate.Email)
	if err == nil && existingUser != nil {
		// Локальный пользователь уже есть
		if existingUser.PasswordHash != nil {
			return nil, ErrEmailExists
		}
		// SSO-пользователь с NULL password_hash
		return nil, ErrEmailExists // или отдельная ошибка, если нужно
	}
	if err != nil && !errors.Is(err, pg.ErrNotFound) {
		return nil, err
	}

	// Хэшируем пароль
	if userCreate.Password == nil {
		return nil, errors.New("password required for local user")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*userCreate.Password), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Errorf("failed to hash password: %v", err)
		return nil, err
	}
	hashStr := string(hashedPassword)

	userToCreate := &model.UserCreate{
		Email:    userCreate.Email,
		Password: &hashStr,
		Name:     userCreate.Name,
	}

	return s.r.Create(ctx, userToCreate)
}

// Update обновляет пользователя
func (s *user) Update(ctx context.Context, id int, update *model.UserUpdate) (*model.UserResponse, error) {
	if err := update.Validate(); err != nil {
		return nil, err
	}

	if update.Password != nil {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*update.Password), bcrypt.DefaultCost)
		if err != nil {
			s.logger.Errorf("failed to hash password: %v", err)
			return nil, err
		}
		hashStr := string(hashedPassword)
		update.Password = &hashStr
	}

	return s.r.Update(ctx, id, update)
}

// Delete удаляет пользователя
func (s *user) Delete(ctx context.Context, id int) error {
	return s.r.Delete(ctx, id)
}

// Login проверяет локальные креды пользователя
func (s *user) Login(ctx context.Context, email, password string) (*model.UserResponse, error) {
	user, err := s.r.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pg.ErrNotFound) {
			return nil, ErrInvalidPass
		}
		return nil, err
	}

	if user.PasswordHash == nil {
		return nil, ErrUseSSOLogin
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidPass
	}

	return user.ToResponse(), nil
}
