package service

import (
	"corpord-api/internal/logger"
	"corpord-api/internal/repository/pg"
	"corpord-api/model"
	"golang.org/x/net/context"
)

type Stop interface {
	All(ctx context.Context) ([]*model.Stop, error)
	ByID(ctx context.Context, id int) (*model.Stop, error)
	Create(ctx context.Context, stop *model.Stop) error
	Update(ctx context.Context, stop *model.StopUpdate) error
	Delete(ctx context.Context, id int) error
}

type stop struct {
	logger *logger.Logger
	repo   pg.Stop
}

func NewStop(logger *logger.Logger, repo pg.Stop) Stop {
	return &stop{
		logger: logger,
		repo:   repo,
	}
}

func (s *stop) All(ctx context.Context) ([]*model.Stop, error) {
	return s.repo.All(ctx)
}

func (s *stop) ByID(ctx context.Context, id int) (*model.Stop, error) {
	return s.repo.ByID(ctx, id)
}

func (s *stop) Create(ctx context.Context, stop *model.Stop) error {
	return s.repo.Create(ctx, stop)
}

func (s *stop) Update(ctx context.Context, stop *model.StopUpdate) error {
	if err := stop.Validate(); err != nil {
		return err
	}
	return s.repo.Update(ctx, stop)
}

func (s *stop) Delete(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}
