package pg

import (
	"context"
	"corpord-api/internal/logger"
	"corpord-api/model"
	"corpord-api/pkg/dbx"
	"errors"

	sq "github.com/Masterminds/squirrel"
)

var ErrRefreshTokenNotFound = errors.New("refresh token not found")

type RefreshTokenRepository interface {
	Save(ctx context.Context, rt *model.RefreshSession) error
	FindByHash(ctx context.Context, hash string) (*model.RefreshSession, error)
	Revoke(ctx context.Context, id string) error
	RevokeAllByUser(ctx context.Context, userID int) error
}

type refreshTokenRepo struct {
	logger *logger.Logger
	qb     *dbx.QueryBuilder
}

func NewRefreshTokenRepo(logger *logger.Logger, qb *dbx.QueryBuilder) RefreshTokenRepository {
	return &refreshTokenRepo{
		logger: logger,
		qb:     qb,
	}
}

// Save сохраняет refresh токен в БД
func (r *refreshTokenRepo) Save(ctx context.Context, token *model.RefreshSession) error {
	query, args, err := r.qb.Sq.Insert(TableRefreshToken).
		Columns(
			"user_id",
			"token_hash",
			"expires_at",
			"ip",
			"user_agent",
		).
		Values(
			token.UserID,
			token.TokenHash,
			token.ExpiresAt,
			token.IP,
			token.UserAgent,
		).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		r.logger.Error(err)
		return err
	}

	_, err = r.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.Error(err)
	}

	return err
}

// FindByHash ищет refresh токен по хешу
func (r *refreshTokenRepo) FindByHash(ctx context.Context, hash string) (*model.RefreshSession, error) {
	rt := &model.RefreshSession{}
	query, args, err := r.qb.Sq.Select(
		"id",
		"user_id",
		"token_hash",
		"expires_at",
		"created_at",
		"updated_at",
		"revoked",
		"ip",
		"user_agent",
	).From(TableRefreshToken).
		Where(sq.Eq{"token_hash": hash}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		r.logger.Error(err)
		return nil, err
	}

	err = r.qb.DB.GetContext(ctx, rt, query, args...)
	if err != nil {
		r.logger.Error(err)
		return nil, ErrRefreshTokenNotFound
	}

	return rt, nil
}

// Revoke помечает конкретный токен как отозванный
func (r *refreshTokenRepo) Revoke(ctx context.Context, id string) error {
	query, args, err := r.qb.Sq.Update(TableRefreshToken).
		Set("revoked", true).
		Set("updated_at", sq.Expr("now()")).
		Where(sq.Eq{"id": id}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		r.logger.Error(err)
		return err
	}

	_, err = r.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.Error(err)
	}

	return err
}

// RevokeAllByUser отзывает все токены пользователя
func (r *refreshTokenRepo) RevokeAllByUser(ctx context.Context, userID int) error {
	query, args, err := r.qb.Sq.Update(TableRefreshToken).
		Set("revoked", true).
		Set("updated_at", sq.Expr("now()")).
		Where(sq.Eq{"user_id": userID}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		r.logger.Error(err)
		return err
	}

	_, err = r.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.Error(err)
	}

	return err
}
