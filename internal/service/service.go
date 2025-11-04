package service

import (
	"corpord-api/internal/logger"
	"corpord-api/internal/repository"
	"corpord-api/internal/token"
)

// Service aggregates all service interfaces
type Service struct {
	logger *logger.Logger
	token  token.Manager
	User   User
	Auth   Auth
	Bus    Bus
	BC     BusCategory
	BS     BusStatus
	DS     DriverStatus
}

// New creates a new service instance with all dependencies
func New(logger *logger.Logger, repo *repository.Repository, token token.Manager) *Service {

	return &Service{
		logger: logger,
		token:  token,
		User:   NewUser(logger, repo.PgRepository.User),
		Auth:   NewAuth(logger, token, repo.PgRepository.Auth),
		Bus:    NewBus(logger, repo.PgRepository.Bus),
		BC:     NewBusCategory(logger, repo.PgRepository.Bc),
		BS:     NewBusStatus(logger, repo.PgRepository.Bs),
		DS:     NewDriverStatus(logger, repo.PgRepository.Ds),
	}
}
