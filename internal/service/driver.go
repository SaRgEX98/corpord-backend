package service

import (
	"corpord-api/internal/logger"
	"corpord-api/internal/repository/pg"
	"corpord-api/model"
	"golang.org/x/net/context"
)

type Driver interface {
	All(ctx context.Context) ([]model.Driver, error)
	ByID(ctx context.Context, id int) (*model.Driver, error)
	Create(ctx context.Context, driver model.Driver) (model.Driver, error)
	Update(ctx context.Context, driver model.Driver) (model.Driver, error)
	Delete(ctx context.Context, id int) error
}

type driver struct {
	logger *logger.Logger
	repo   pg.Driver
}

func NewDriver(logger *logger.Logger, repo pg.Driver) Driver {
	return &driver{
		logger: logger,
		repo:   repo,
	}
}

func (d *driver) All(ctx context.Context) ([]model.Driver, error) {
	return d.repo.All(ctx)
}

func (d *driver) ByID(ctx context.Context, id int) (*model.Driver, error) {
	return d.repo.ByID(ctx, id)
}

func (d *driver) Create(ctx context.Context, driver model.Driver) (model.Driver, error) {
	return d.repo.Create(ctx, driver)
}

func (d *driver) Update(ctx context.Context, driver model.Driver) (model.Driver, error) {
	return d.repo.Update(ctx, driver)
}

func (d *driver) Delete(ctx context.Context, id int) error {
	return d.repo.Delete(ctx, id)
}
