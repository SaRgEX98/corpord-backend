package service

import (
	"context"
	"corpord-api/internal/logger"
	"corpord-api/internal/repository/pg"
	"corpord-api/model"
)

type Bus interface {
	CreateBus(ctx context.Context, bus model.Bus) error
	GetBus(ctx context.Context, id int) (*model.ViewBus, error)
	GetAllBuses(ctx context.Context) ([]model.ViewBus, error)
	UpdateBus(ctx context.Context, bus model.BusUpdate) error
	DeleteBus(ctx context.Context, id int) error
}

type bus struct {
	logger *logger.Logger
	repo   pg.BusRepository
}

func NewBus(logger *logger.Logger, repo pg.BusRepository) Bus {
	return &bus{
		logger: logger,
		repo:   repo,
	}
}

func (b *bus) CreateBus(ctx context.Context, bus model.Bus) error {
	return b.repo.CreateBus(ctx, bus)
}

func (b *bus) GetBus(ctx context.Context, id int) (*model.ViewBus, error) {
	return b.repo.GetBus(ctx, id)
}

func (b *bus) GetAllBuses(ctx context.Context) ([]model.ViewBus, error) {
	return b.repo.GetAllBuses(ctx)
}

func (b *bus) UpdateBus(ctx context.Context, bus model.BusUpdate) error {
	if bus.Validate() != nil {
		return ErrNoFields
	}
	return b.repo.UpdateBus(ctx, &bus)
}

func (b *bus) DeleteBus(ctx context.Context, id int) error {
	return b.repo.DeleteBus(ctx, id)
}
