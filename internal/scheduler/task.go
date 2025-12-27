package scheduler

import (
	"context"
	"corpord-api/internal/repository/pg"

	"corpord-api/internal/logger"
)

type CleanupRefreshTokensTask struct {
	repo   pg.RefreshTokenRepository
	logger *logger.Logger
}

func NewCleanupRefreshTokensTask(repo pg.RefreshTokenRepository, logger *logger.Logger) *CleanupRefreshTokensTask {
	return &CleanupRefreshTokensTask{
		repo:   repo,
		logger: logger,
	}
}

func (t *CleanupRefreshTokensTask) Run(ctx context.Context) error {
	if err := t.repo.CleanupExpired(ctx); err != nil {
		t.logger.Warnf("failed to cleanup expired refresh tokens: %v", err)
		return err
	}
	t.logger.Info("expired refresh tokens cleanup completed")
	return nil
}
