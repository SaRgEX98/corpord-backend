package model

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// UserCreate представляет данные для создания пользователя
type UserCreate struct {
	Email    string  `json:"email" binding:"required,email"`
	Password *string `json:"password,omitempty"` // pointer для nullable
	Name     string  `json:"name" binding:"required"`
}

// UserUpdate представляет данные для обновления пользователя
type UserUpdate struct {
	Name     *string `json:"name,omitempty"`
	Password *string `json:"password,omitempty"`
	Email    *string `json:"email,omitempty"`
}

// Validate проверяет валидность полей обновления пользователя
func (u *UserUpdate) Validate() error {
	if u.Name != nil && *u.Name == "" {
		return errors.New("имя не может быть пустым")
	}

	if u.Email != nil {
		if *u.Email == "" {
			return errors.New("email не может быть пустым")
		}
		if !strings.Contains(*u.Email, "@") {
			return errors.New("некорректный формат email")
		}
	}

	if u.Name == nil && u.Email == nil && u.Password == nil {
		return errors.New("не указаны поля для обновления")
	}

	return nil
}

// UserResponse представляет данные пользователя для отображения (без чувствительных данных)
type UserResponse struct {
	ID        int       `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	UserAgent string    `json:"user_agent"`
	IP        string    `json:"ip"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserLogin представляет данные для аутентификации
type UserLogin struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required"`
	UserAgent string `json:"user_agent"`
	IP        string `json:"ip"`
}

// UserDB представляет модель пользователя в базе данных
type UserDB struct {
	ID           int        `db:"id"`
	Email        string     `db:"email"`
	PasswordHash *string    `db:"password_hash"`
	Name         string     `db:"name"`
	Role         string     `db:"role_name"`
	UserAgent    string     `db:"user_agent"`
	IP           string     `db:"ip"`
	Provider     string     `db:"provider"`
	ProviderID   string     `db:"provider_id"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`
	DeletedAt    *time.Time `db:"deleted_at"`
}

// UserIdentity
type UserIdentity struct {
	ID         uuid.UUID `db:"id"`
	UserID     int       `db:"user_id"`
	Provider   string    `db:"provider"`
	ProviderID string    `db:"provider_id"`
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}

// ToResponse преобразует UserDB в UserResponse
func (u *UserDB) ToResponse() *UserResponse {
	return &UserResponse{
		ID:        u.ID,
		Email:     u.Email,
		Name:      u.Name,
		Role:      u.Role,
		UserAgent: u.UserAgent,
		IP:        u.IP,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}
