package service

import (
	"context"
	"corpord-api/internal/logger"
	"corpord-api/internal/repository/pg"
	"corpord-api/model"
)

type BusCategory interface {
	GetAll(ctx context.Context) ([]model.BusCategory, error)
	GetById(ctx context.Context, id int) (*model.BusCategory, error)
	Create(ctx context.Context, category model.BusCategory) error
	Update(ctx context.Context, category model.BusCategory) (model.BusCategory, error)
	Delete(ctx context.Context, id int) error
}

type busCategory struct {
	logger *logger.Logger
	repo   pg.BusCategory
}

func NewBusCategory(logger *logger.Logger, repo pg.BusCategory) BusCategory {
	return &busCategory{
		logger: logger,
		repo:   repo,
	}
}

func (b *busCategory) GetAll(ctx context.Context) ([]model.BusCategory, error) {
	return b.repo.GetAll(ctx)
}

func (b *busCategory) GetById(ctx context.Context, id int) (*model.BusCategory, error) {
	return b.repo.GetById(ctx, id)
}

func (b *busCategory) Create(ctx context.Context, category model.BusCategory) error {
	return b.repo.Create(ctx, category)
}

func (b *busCategory) Update(ctx context.Context, category model.BusCategory) (model.BusCategory, error) {
	if err := category.Validate(); err != nil {
		return model.BusCategory{}, err
	}
	return b.repo.Update(ctx, category)
}

func (b *busCategory) Delete(ctx context.Context, id int) error {
	return b.repo.Delete(ctx, id)
}
