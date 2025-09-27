package service

import (
	"corpord-api/internal/logger"
	"corpord-api/internal/repository"
)

// Service aggregates all service interfaces
type Service struct {
	logger *logger.Logger
	User
}

// New creates a new service instance with all dependencies
func New(logger *logger.Logger, repo *repository.Repository) *Service {
	return &Service{
		logger: logger,
		User:   NewUser(logger, repo.PgRepository.User, "secret"),
	}
}
