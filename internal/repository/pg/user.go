package pg

import (
	"context"
	"corpord-api/internal/logger"
	"database/sql"
	"errors"
	"time"

	"corpord-api/model"
	sq "github.com/Masterminds/squirrel"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

type UserRepository interface {
	Create(ctx context.Context, user *model.UserCreate) (*model.UserResponse, error)
	GetByID(ctx context.Context, id int) (*model.UserResponse, error)
	GetByEmail(ctx context.Context, email string) (*model.UserDB, error)
	Update(ctx context.Context, id int, user *model.UserUpdate) (*model.UserResponse, error)
	Delete(ctx context.Context, id int) error
	GetAll(ctx context.Context) ([]*model.UserResponse, error)
}

type userRepository struct {
	logger *logger.Logger
	db     *sqlx.DB
}

func NewUserRepository(logger *logger.Logger, db *sqlx.DB) UserRepository {
	return &userRepository{
		logger: logger,
		db:     db,
	}
}

func (r *userRepository) Create(ctx context.Context, user *model.UserCreate) (*model.UserResponse, error) {
	r.logger.Infof("creating new user with email: %s", user.Email)

	now := time.Now()

	var userDB model.UserDB

	query, args, err := sq.Insert(TableUsers).
		Columns("email", "password_hash", "name", "created_at", "updated_at").
		Values(user.Email, user.Password /* hashedPassword */, user.Name, now, now).
		Suffix("RETURNING id, email, name, created_at, updated_at").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		r.logger.Errorf("failed to build create user query: %v", err)
		return nil, err
	}

	err = r.db.GetContext(ctx, &userDB, query, args...)
	if err != nil {
		r.logger.Errorf("failed to create user with email %s: %v", user.Email, err)
		return nil, err
	}

	r.logger.Infof("successfully created user with id: %d", userDB.ID)
	return userDB.ToResponse(), nil
}

// GetAll retrieves all users from the database with pagination support
func (r *userRepository) GetAll(ctx context.Context) ([]*model.UserResponse, error) {
	r.logger.Info("fetching all users")

	// Build the base query
	query, args, err := sq.Select(
		"id",
		"name",
		"email",
		"created_at",
		"updated_at",
	).
		From(TableUsers).
		OrderBy("id ASC").
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		r.logger.Errorf("failed to build get all users query: %v", err)
		return nil, err
	}

	// Execute the query
	var users []*model.UserDB
	err = r.db.SelectContext(ctx, &users, query, args...)
	if err != nil {
		r.logger.Errorf("failed to fetch users: %v", err)
		return nil, err
	}

	// Convert to response model
	responses := make([]*model.UserResponse, len(users))
	for i, user := range users {
		responses[i] = user.ToResponse()
	}

	r.logger.Infof("successfully fetched %d users", len(responses))
	return responses, nil
}

func (r *userRepository) GetByID(ctx context.Context, id int) (*model.UserResponse, error) {
	r.logger.Infof("fetching user with id: %d", id)

	query, args, err := sq.Select("id", "name", "email", "created_at", "updated_at").
		From(TableUsers).
		Where(sq.Eq{"id": id}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		r.logger.Errorf("failed to build query for user id %d: %v", id, err)
		return nil, err
	}

	var user model.UserDB
	err = r.db.GetContext(ctx, &user, query, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.logger.Infof("user with id %d not found", id)
			return nil, ErrNotFound
		}
		r.logger.Errorf("failed to fetch user with id %d: %v", id, err)
		return nil, err
	}

	r.logger.Infof("successfully fetched user with id %d", id)
	return &model.UserResponse{
		ID:    user.ID,
		Name:  user.Name,
		Email: user.Email,
	}, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*model.UserDB, error) {
	r.logger.Infof("fetching user by email: %s", email)

	query, args, err := sq.Select("id", "email", "password_hash", "name", "created_at", "updated_at").
		From(TableUsers).
		Where(sq.Eq{"email": email}).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		r.logger.Errorf("failed to build query for user with email %s: %v", email, err)
		return nil, err
	}

	var user model.UserDB
	err = r.db.GetContext(ctx, &user, query, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.logger.Infof("user with email %s not found", email)
			return nil, ErrNotFound
		}
		r.logger.Errorf("failed to fetch user with email %s: %v", email, err)
		return nil, err
	}

	r.logger.Debugf("successfully fetched user with email %s", email)
	return &user, nil
}

func (r *userRepository) Update(ctx context.Context, id int, user *model.UserUpdate) (*model.UserResponse, error) {
	r.logger.Infof("updating user with id: %d", id)

	now := time.Now()

	updateQuery := sq.Update(TableUsers).
		Set("updated_at", now).
		Where(sq.Eq{"id": id}).
		PlaceholderFormat(sq.Dollar)

	if user.Name != nil {
		updateQuery = updateQuery.Set("name", *user.Name)
		r.logger.Debugf("updating name for user %d", id)
	}

	if user.Email != nil {
		updateQuery = updateQuery.Set("email", *user.Email)
		r.logger.Debugf("updating email for user %d", id)
	}

	if user.Password != nil {
		updateQuery = updateQuery.Set("password_hash", *user.Password)
		r.logger.Debugf("updating password for user %d", id)
	}

	query, args, err := updateQuery.ToSql()
	if err != nil {
		r.logger.Errorf("failed to build update query for user %d: %v", id, err)
		return nil, err
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.Errorf("failed to update user %d: %v", id, err)
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		r.logger.Warnf("failed to get rows affected for user %d update: %v", id, err)
	} else if rowsAffected == 0 {
		r.logger.Infof("no rows affected when updating user %d - user not found", id)
		return nil, ErrNotFound
	}

	r.logger.Infof("successfully updated user with id: %d", id)
	return r.GetByID(ctx, id)
}

func (r *userRepository) Delete(ctx context.Context, id int) error {
	r.logger.Infof("deleting user with id: %d", id)

	query, args, err := sq.Delete(TableUsers).
		Where(sq.Eq{"id": id}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		r.logger.Errorf("failed to build delete query for user %d: %v", id, err)
		return err
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.Errorf("failed to delete user %d: %v", id, err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		r.logger.Warnf("failed to get rows affected for user %d deletion: %v", id, err)
	} else if rowsAffected == 0 {
		r.logger.Infof("no rows affected when deleting user %d - user not found", id)
		return ErrNotFound
	}

	r.logger.Infof("successfully deleted user with id: %d", id)
	return nil
}
