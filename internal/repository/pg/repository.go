package pg

import (
	"corpord-api/internal/logger"
	"github.com/jmoiron/sqlx"
)

type PostgresRepository struct {
	logger *logger.Logger
	User   UserRepository
}

func New(logger *logger.Logger, db *sqlx.DB) *PostgresRepository {
	return &PostgresRepository{
		logger: logger,
		User:   NewUserRepository(logger, db),
	}
}
