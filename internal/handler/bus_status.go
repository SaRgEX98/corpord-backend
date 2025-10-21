package handler

import (
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
)

type BusStatusHandler struct {
	logger *logger.Logger
	bs     service.BusStatus
}

func NewBusStatus(logger *logger.Logger, bs service.BusStatus) *BusStatusHandler {
	return &BusStatusHandler{
		logger: logger,
		bs:     bs,
	}
}
