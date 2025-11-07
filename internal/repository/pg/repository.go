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
	Bc     BusCategory
	Bs     BusStatus
	Ds     DriverStatus
	Driver Driver
	Trip   Trip
}

func New(logger *logger.Logger, qb *dbx.QueryBuilder) *PostgresRepository {
	return &PostgresRepository{
		logger: logger,
		User:   NewUserRepository(logger, qb),
		Auth:   NewAuthRepository(logger, qb),
		Bus:    NewBusRepository(logger, qb),
		Bc:     NewBusCategory(logger, qb),
		Bs:     NewBusStatus(logger, qb),
		Ds:     NewDriverStatus(logger, qb),
		Driver: NewDriver(logger, qb),
		Trip:   NewTrip(logger, qb),
	}
}
