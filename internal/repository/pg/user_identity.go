package pg

import (
	"context"
	"corpord-api/internal/logger"
	"corpord-api/model"
	"corpord-api/pkg/dbx"
	"database/sql"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
)

var ErrIdentityNotFound = errors.New("user identity not found")

// UserIdentitiesRepository defines operations for user external identities
type UserIdentitiesRepository interface {
	GetByUserID(ctx context.Context, userID int) ([]*model.UserIdentity, error)
	GetByProvider(ctx context.Context, userID int, provider string) (*model.UserIdentity, error)
	GetProviderByID(ctx context.Context, provider, providerID string) (*model.UserIdentity, error)
	AddIdentity(ctx context.Context, identity *model.UserIdentity) error
	CreateUserWithIdentity(ctx context.Context, user *model.UserCreate, identity *model.UserIdentity) (*model.UserDB, error)
	RemoveIdentity(ctx context.Context, identityID string) error
}

type userIdentitiesRepo struct {
	logger *logger.Logger
	qb     *dbx.QueryBuilder
}

func NewUserIdentitiesRepo(logger *logger.Logger, qb *dbx.QueryBuilder) UserIdentitiesRepository {
	return &userIdentitiesRepo{
		logger: logger,
		qb:     qb,
	}
}

// GetByUserID retrieves all identities for a user
func (r *userIdentitiesRepo) GetByUserID(ctx context.Context, userID int) ([]*model.UserIdentity, error) {
	query, args, err := r.qb.Sq.Select(
		"id",
		"user_id",
		"provider",
		"provider_id",
		"created_at",
		"updated_at",
	).From("user_identities").
		Where(sq.Eq{"user_id": userID}).
		ToSql()
	if err != nil {
		r.logger.Error("failed to build query for GetByUserID", "error", err)
		return nil, err
	}

	var identities []*model.UserIdentity
	err = r.qb.DB.SelectContext(ctx, &identities, query, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []*model.UserIdentity{}, nil
		}
		r.logger.Error("failed to get user identities", "error", err, "user_id", userID)
		return nil, err
	}

	return identities, nil
}

// GetByProvider retrieves identity by userID and provider.go
func (r *userIdentitiesRepo) GetByProvider(ctx context.Context, userID int, provider string) (*model.UserIdentity, error) {
	query, args, err := r.qb.Sq.Select(
		"id",
		"user_id",
		"provider",
		"provider_id",
		"created_at",
		"updated_at",
	).From("user_identities").
		Where(sq.Eq{"user_id": userID, "provider": provider}).
		ToSql()
	if err != nil {
		r.logger.Error("failed to build query for GetByProvider", "error", err)
		return nil, err
	}

	var identity model.UserIdentity
	err = r.qb.DB.GetContext(ctx, &identity, query, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrIdentityNotFound
		}
		r.logger.Error("failed to get user identity by provider.go", "error", err)
		return nil, err
	}

	return &identity, nil
}

// GetProviderByID retrieves identity by provider.go and providerID (без userID)
func (r *userIdentitiesRepo) GetProviderByID(ctx context.Context, provider, providerID string) (*model.UserIdentity, error) {
	query, args, err := r.qb.Sq.Select(
		"id",
		"user_id",
		"provider",
		"provider_id",
		"created_at",
		"updated_at",
	).From("user_identities").
		Where(sq.Eq{"provider": provider, "provider_id": providerID}).
		ToSql()
	if err != nil {
		r.logger.Error("failed to build query for GetProviderByID", "error", err)
		return nil, err
	}

	var identity model.UserIdentity
	err = r.qb.DB.GetContext(ctx, &identity, query, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrIdentityNotFound
		}
		r.logger.Error("failed to get user identity by providerID", "error", err)
		return nil, err
	}

	return &identity, nil
}

// CreateUserWithIdentity commits both records together. A duplicate identity or
// email rolls back the inserted user rather than leaving an orphaned account.
func (r *userIdentitiesRepo) CreateUserWithIdentity(ctx context.Context, user *model.UserCreate, identity *model.UserIdentity) (*model.UserDB, error) {
	if user == nil || identity == nil {
		return nil, errors.New("missing SSO user or identity")
	}
	tx, err := r.qb.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var userID int
	if err := tx.QueryRowContext(ctx, `INSERT INTO users (email, password_hash, name) VALUES ($1, NULL, $2) RETURNING id`, user.Email, user.Name).Scan(&userID); err != nil {
		return nil, fmt.Errorf("insert SSO user: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_identities (id, user_id, provider, provider_id) VALUES ($1, $2, $3, $4)`, identity.ID, userID, identity.Provider, identity.ProviderID); err != nil {
		return nil, fmt.Errorf("insert SSO identity: %w", err)
	}
	var created model.UserDB
	if err := tx.GetContext(ctx, &created, `SELECT u.id, u.email, u.password_hash, u.name, r.name AS role_name, u.created_at, u.updated_at, u.deleted_at FROM users u JOIN roles r ON r.id = u.role_id WHERE u.id = $1`, userID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	identity.UserID = userID
	return &created, nil
}

// AddIdentity adds a new identity for a user
func (r *userIdentitiesRepo) AddIdentity(ctx context.Context, identity *model.UserIdentity) error {
	query, args, err := r.qb.Sq.Insert("user_identities").
		Columns("id", "user_id", "provider", "provider_id").
		Values(identity.ID, identity.UserID, identity.Provider, identity.ProviderID).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		r.logger.Error("failed to build AddIdentity query", "error", err)
		return err
	}

	_, err = r.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.Error("failed to insert user identity", "error", err)
	}

	return err
}

// RemoveIdentity deletes an identity by its ID
func (r *userIdentitiesRepo) RemoveIdentity(ctx context.Context, identityID string) error {
	query, args, err := r.qb.Sq.Delete("user_identities").
		Where(sq.Eq{"id": identityID}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		r.logger.Error("failed to build RemoveIdentity query", "error", err)
		return err
	}

	_, err = r.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.Error("failed to delete user identity", "error", err)
	}

	return err
}
