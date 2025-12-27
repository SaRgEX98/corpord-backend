package pg

import (
	"context"
	"corpord-api/internal/logger"
	"corpord-api/model"
	"corpord-api/pkg/dbx"
	"errors"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
)

var ErrRefreshTokenNotFound = errors.New("refresh token not found")

type RefreshTokenRepository interface {
	Save(ctx context.Context, rt *model.RefreshSession) error
	FindByHash(ctx context.Context, hash string) (*model.RefreshSession, error)
	Revoke(ctx context.Context, id uuid.UUID) error
	RevokeAllByUser(ctx context.Context, userID int) error
	RefreshToken(ctx context.Context, oldHash string, newSession *model.RefreshSession) error
	CleanupExpired(ctx context.Context) error
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
			"id",
			"user_id",
			"token_hash",
			"expires_at",
			"ip",
			"user_agent",
		).
		Values(
			token.ID,
			token.UserID,
			token.TokenHash,
			token.ExpiresAt,
			token.IP,
			token.UserAgent,
		).
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
		"revoked",
		"ip",
		"user_agent",
	).From(TableRefreshToken).
		Where(sq.Eq{"token_hash": hash, "revoked": false}).
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
func (r *refreshTokenRepo) Revoke(ctx context.Context, id uuid.UUID) error {
	query, args, err := r.qb.Sq.Update(TableRefreshToken).
		Set("revoked", true).
		Where(sq.Eq{"id": id}).
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
		Where(sq.Eq{"user_id": userID}).
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

func (r *refreshTokenRepo) RefreshToken(
	ctx context.Context,
	oldHash string,
	newSession *model.RefreshSession,
) error {
	tx, err := r.qb.DB.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Найти старый токен
	oldToken := &model.RefreshSession{}
	query, args, _ := r.qb.Sq.Select("id", "revoked").
		From(TableRefreshToken).
		Where(sq.Eq{"token_hash": oldHash, "revoked": false}).
		ToSql()

	if err := tx.GetContext(ctx, oldToken, query, args...); err != nil {
		return ErrRefreshTokenNotFound
	}

	// Отозвать старый токен
	_, err = tx.ExecContext(ctx,
		"UPDATE refresh_tokens SET revoked=true WHERE id=$1", oldToken.ID)
	if err != nil {
		return err
	}

	// Сохранить новый токен (только хеш и метаданные)
	query, args, _ = r.qb.Sq.Insert(TableRefreshToken).
		Columns("id", "user_id", "token_hash", "expires_at", "ip", "user_agent").
		Values(newSession.ID, newSession.UserID, newSession.TokenHash, newSession.ExpiresAt, newSession.IP, newSession.UserAgent).
		ToSql()
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return err
	}

	return tx.Commit()
}

// CleanupExpired удаляет все истёкшие refresh-токены
func (r *refreshTokenRepo) CleanupExpired(ctx context.Context) error {
	query, args, err := r.qb.Sq.
		Delete(TableRefreshToken).
		Where("expires_at < now()").
		ToSql()
	if err != nil {
		r.logger.Error("failed to build cleanup query: ", err)
		return err
	}

	res, err := r.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.Error("failed to execute cleanup query: ", err)
		return err
	}

	count, _ := res.RowsAffected()
	r.logger.Infof("cleanup expired refresh tokens: deleted %d rows", count)
	return nil
}
