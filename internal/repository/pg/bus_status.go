package pg

import (
	"corpord-api/internal/logger"
	"corpord-api/pkg/dbx"
)

type BusStatus interface {
}

type busStatus struct {
	logger *logger.Logger
	qb     *dbx.QueryBuilder
}

func NewBusStatus(logger *logger.Logger, qb *dbx.QueryBuilder) BusStatus {
	return &busStatus{
		logger: logger,
		qb:     qb,
	}
}
