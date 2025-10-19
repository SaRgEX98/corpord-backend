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
	Create()
	Update()
	Delete()
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

func (b *busCategory) Create() {
	//TODO implement me
	panic("implement me")
}

func (b *busCategory) Update() {
	//TODO implement me
	panic("implement me")
}

func (b *busCategory) Delete() {
	//TODO implement me
	panic("implement me")
}
