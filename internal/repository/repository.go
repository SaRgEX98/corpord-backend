package repository

import (
	"corpord-api/internal/logger"
	"corpord-api/internal/repository/pg"
	"corpord-api/pkg/dbx"
)

type Repository struct {
	logger       *logger.Logger
	PgRepository *pg.PostgresRepository
}

func New(logger *logger.Logger, qb *dbx.QueryBuilder) *Repository {
	return &Repository{
		logger:       logger,
		PgRepository: pg.New(logger, qb),
	}
}
