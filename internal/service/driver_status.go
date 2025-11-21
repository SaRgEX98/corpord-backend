package service

import (
	"context"
	"corpord-api/internal/logger"
	"corpord-api/internal/repository/pg"
	"corpord-api/model"
)

type DriverStatus interface {
	All(ctx context.Context) []model.DriverStatus
	ById(ctx context.Context, id int) (model.DriverStatus, error)
	Create(ctx context.Context, status *model.DriverStatus) error
	Update(ctx context.Context, status *model.DriverStatus) error
	Delete(ctx context.Context, id int) error
}

type driverStatus struct {
	logger *logger.Logger
	repo   pg.DriverStatus
}

func NewDriverStatus(logger *logger.Logger, repo pg.DriverStatus) DriverStatus {
	return &driverStatus{
		logger: logger,
		repo:   repo,
	}
}

func (d *driverStatus) All(ctx context.Context) []model.DriverStatus {
	return d.repo.All(ctx)
}

func (d *driverStatus) ById(ctx context.Context, id int) (model.DriverStatus, error) {
	return d.repo.ById(ctx, id)
}

func (d *driverStatus) Create(ctx context.Context, status *model.DriverStatus) error {
	return d.repo.Create(ctx, status)
}

func (d *driverStatus) Update(ctx context.Context, status *model.DriverStatus) error {
	if err := status.Validate(); err != nil {
		return err
	}
	return d.repo.Update(ctx, status)
}

func (d *driverStatus) Delete(ctx context.Context, id int) error {
	return d.repo.Delete(ctx, id)
}
