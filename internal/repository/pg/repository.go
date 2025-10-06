package pg

import (
	"corpord-api/internal/logger"
	"corpord-api/pkg/dbx"
)

type PostgresRepository struct {
	logger *logger.Logger
	User   UserRepository
	Auth   AuthRepository
	Bus    BusRepository
}

func New(logger *logger.Logger, qb *dbx.QueryBuilder) *PostgresRepository {
	return &PostgresRepository{
		logger: logger,
		User:   NewUserRepository(logger, qb),
		Auth:   NewAuthRepository(logger, qb),
		Bus:    NewBusRepository(logger, qb),
	}
}
