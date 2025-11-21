package pg

import (
	"context"
	"corpord-api/internal/logger"
	"corpord-api/model"
	"corpord-api/pkg/dbx"
	"database/sql"
	"errors"

	sq "github.com/Masterminds/squirrel"
)

// AuthRepository defines the interface for authentication-related database operations
type AuthRepository interface {
	// CreateUser creates a new user in the database
	CreateUser(ctx context.Context, user *model.UserCreate) (int, error)
	// GetUserByEmail retrieves a user by email
	GetUserByEmail(ctx context.Context, email string) (*model.UserDB, error)
	// GetUserByID retrieves a user by ID
	GetUserByID(ctx context.Context, id int) (*model.UserDB, error)
}

type authRepository struct {
	logger *logger.Logger
	qb     *dbx.QueryBuilder
}

// NewAuthRepository creates a new instance of AuthRepository
func NewAuthRepository(logger *logger.Logger, qb *dbx.QueryBuilder) AuthRepository {
	return &authRepository{
		logger: logger,
		qb:     qb,
	}
}

// CreateUser creates a new user in the database
func (r *authRepository) CreateUser(ctx context.Context, user *model.UserCreate) (int, error) {
	query, args, err := r.qb.Sq.Insert(TableUsers).
		Columns("email", "password_hash", "name").
		Values(
			user.Email,
			user.Password,
			user.Name,
		).
		Suffix("RETURNING id").
		ToSql()

	if err != nil {
		r.logger.Error("Failed to build create user query", "error", err)
		return 0, err
	}

	var id int
	err = r.qb.DB.QueryRowContext(ctx, query, args...).Scan(&id)
	if err != nil {
		r.logger.Error("Failed to create user", "error", err, "email", user.Email)
		return 0, err
	}

	return id, nil
}

// GetUserByEmail retrieves a user by email with role name
func (r *authRepository) GetUserByEmail(ctx context.Context, email string) (*model.UserDB, error) {
	query, args, err := r.qb.Sq.Select(
		"u.id",
		"u.email",
		"u.password_hash",
		"u.name",
		"r.name as role_name",
		"u.created_at",
		"u.updated_at",
	).
		From("users u").
		Join("roles r ON u.role_id = r.id").
		Where(sq.Eq{"u.email": email, "u.deleted_at": nil}).
		ToSql()

	if err != nil {
		r.logger.Error("Failed to build get user by email query", "error", err)
		return nil, err
	}

	var user model.UserDB
	err = r.qb.DB.GetContext(ctx, &user, query, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		r.logger.Error("Failed to get user by email", "error", err, "email", email)
		return nil, err
	}

	return &user, nil
}

// GetUserByID retrieves a user by ID with role name
func (r *authRepository) GetUserByID(ctx context.Context, id int) (*model.UserDB, error) {
	query, args, err := r.qb.Sq.Select(
		"u.id",
		"u.email",
		"u.password_hash",
		"u.name",
		"r.name as role_name",
		"u.created_at",
		"u.updated_at",
	).
		From("users u").
		Join("roles r ON u.role_id = r.id").
		Where(sq.Eq{"u.id": id, "u.deleted_at": nil}).
		ToSql()

	if err != nil {
		r.logger.Error("Failed to build get user by ID query", "error", err)
		return nil, err
	}

	var user model.UserDB
	err = r.qb.DB.GetContext(ctx, &user, query, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		r.logger.Error("Failed to get user by ID", "error", err, "user_id", id)
		return nil, err
	}

	return &user, nil
}
