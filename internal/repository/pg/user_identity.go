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

var ErrIdentityNotFound = errors.New("user identity not found")

// UserIdentitiesRepository defines operations for user external identities
type UserIdentitiesRepository interface {
	GetByUserID(ctx context.Context, userID int) ([]*model.UserIdentity, error)
	GetByProvider(ctx context.Context, userID int, provider string) (*model.UserIdentity, error)
	GetProviderByID(ctx context.Context, provider, providerID string) (*model.UserIdentity, error)
	AddIdentity(ctx context.Context, identity *model.UserIdentity) error
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
