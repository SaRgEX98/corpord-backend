package pg

import (
	"context"
	"corpord-api/internal/logger"
	"corpord-api/model"
	"corpord-api/pkg/dbx"
	"database/sql"
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
			token.ExpiresAt.UTC(),
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRefreshTokenNotFound
		}
		r.logger.Error(err)
		return nil, err
	}

	return rt, nil
}

// Revoke помечает конкретный токен как отозванный
func (r *refreshTokenRepo) Revoke(ctx context.Context, id uuid.UUID) error {
	query, args, err := r.qb.Sq.Update(TableRefreshToken).
		Set("revoked", true).
		Set("updated_at", sq.Expr("now()")).
		Where(sq.Eq{"id": id, "revoked": false}).
		ToSql()
	if err != nil {
		r.logger.Error(err)
		return err
	}

	result, err := r.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.Error(err)
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrRefreshTokenNotFound
	}
	return nil
}

// RevokeAllByUser отзывает все токены пользователя
func (r *refreshTokenRepo) RevokeAllByUser(ctx context.Context, userID int) error {
	tx, err := r.qb.DB.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(1146376786, $1)", userID); err != nil {
		return err
	}
	query, args, err := r.qb.Sq.Update(TableRefreshToken).
		Set("revoked", true).
		Set("updated_at", sq.Expr("now()")).
		Where(sq.Eq{"user_id": userID}).
		ToSql()
	if err != nil {
		r.logger.Error(err)
		return err
	}

	_, err = tx.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.Error(err)
		return err
	}
	return tx.Commit()
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
	// Serialize rotation and logout-all for this user. Without a common lock,
	// logout-all can miss a replacement inserted by a concurrent refresh.
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(1146376786, $1)", newSession.UserID); err != nil {
		return err
	}

	// A conditional UPDATE takes the row lock. Concurrent refreshes wait and
	// re-evaluate revoked after the winner commits, so exactly one can succeed.
	var oldID uuid.UUID
	err = tx.QueryRowxContext(ctx, `
		UPDATE refresh_tokens SET revoked = true, updated_at = now()
		WHERE token_hash = $1 AND user_id = $2
		  AND revoked = false AND expires_at > (clock_timestamp() AT TIME ZONE 'UTC')
		RETURNING id`, oldHash, newSession.UserID).Scan(&oldID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrRefreshTokenNotFound
		}
		return err
	}

	// Сохранить новый токен (только хеш и метаданные)
	query, args, err := r.qb.Sq.Insert(TableRefreshToken).
		Columns("id", "user_id", "token_hash", "expires_at", "ip", "user_agent").
		Values(newSession.ID, newSession.UserID, newSession.TokenHash, newSession.ExpiresAt.UTC(), newSession.IP, newSession.UserAgent).
		ToSql()
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return err
	}

	return tx.Commit()
}

// CleanupExpired удаляет все истёкшие refresh-токены
func (r *refreshTokenRepo) CleanupExpired(ctx context.Context) error {
	query, args, err := r.qb.Sq.
		Delete(TableRefreshToken).
		Where("expires_at < (now() AT TIME ZONE 'UTC')").
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
