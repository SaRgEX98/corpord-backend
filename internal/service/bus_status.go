package service

import (
	"context"
	"corpord-api/internal/logger"
	"corpord-api/internal/repository/pg"
	"corpord-api/model"
)

type BusStatus interface {
	All(ctx context.Context) ([]model.BusStatus, error)
	ByID(ctx context.Context, id int) (model.BusStatus, error)
	Create(ctx context.Context, status model.BusStatus) error
	Update(ctx context.Context, status model.BusStatus) (model.BusStatus, error)
	Delete(ctx context.Context, id int) error
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

func (b *busStatus) All(ctx context.Context) ([]model.BusStatus, error) {
	return b.repo.All(ctx)
}

func (b *busStatus) ByID(ctx context.Context, id int) (model.BusStatus, error) {
	return b.repo.ByID(ctx, id)
}

func (b *busStatus) Create(ctx context.Context, status model.BusStatus) error {
	return b.repo.Create(ctx, status)
}

func (b *busStatus) Update(ctx context.Context, status model.BusStatus) (model.BusStatus, error) {
	if status.Validate() != nil {
		return model.BusStatus{}, ErrBusStatusExists
	}
	return b.repo.Update(ctx, status)
}

func (b *busStatus) Delete(ctx context.Context, id int) error {
	return b.repo.Delete(ctx, id)
}
