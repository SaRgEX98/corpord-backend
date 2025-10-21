package service

import (
	"corpord-api/internal/logger"
	"corpord-api/internal/repository/pg"
)

type BusStatus interface {
}

type busStatus struct {
	logger *logger.Logger
	repo   pg.BusStatus
}

func NewBusStatus(logger *logger.Logger, repo pg.BusStatus) BusStatus {
	return &busStatus{
		logger: logger,
		repo:   repo,
	}
}
