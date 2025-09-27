package repository

import (
	"corpord-api/internal/logger"
	"corpord-api/internal/repository/pg"
	"github.com/jmoiron/sqlx"
)

type Repository struct {
	logger       *logger.Logger
	PgRepository *pg.PostgresRepository
}

func New(logger *logger.Logger, db *sqlx.DB) *Repository {
	return &Repository{
		logger:       logger,
		PgRepository: pg.New(logger, db),
	}
}
