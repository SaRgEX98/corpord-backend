package service

import (
	"corpord-api/internal/logger"
	"corpord-api/internal/repository/pg"
	"corpord-api/model"
	"golang.org/x/net/context"
)

type TripStop interface {
	All(ctx context.Context) ([]*model.TripStop, error)
	ByID(ctx context.Context, id int) (*model.TripStop, error)
	Create(ctx context.Context, trip *model.TripStop) error
	Update(ctx context.Context, trip *model.TripStopUpdate) error
	Delete(ctx context.Context, id int) error
}

type tripStop struct {
	logger *logger.Logger
	repo   pg.TripStop
}

func NewTripStop(logger *logger.Logger, repo pg.TripStop) TripStop {
	return &tripStop{
		logger: logger,
		repo:   repo,
	}
}

func (t *tripStop) All(ctx context.Context) ([]*model.TripStop, error) {
	return t.repo.All(ctx)
}

func (t *tripStop) ByID(ctx context.Context, id int) (*model.TripStop, error) {
	return t.repo.ByID(ctx, id)
}

func (t *tripStop) Create(ctx context.Context, trip *model.TripStop) error {
	return t.repo.Create(ctx, trip)
}

func (t *tripStop) Update(ctx context.Context, trip *model.TripStopUpdate) error {
	if err := trip.Validate(); err != nil {
		return err
	}
	return t.repo.Update(ctx, trip)
}

func (t *tripStop) Delete(ctx context.Context, id int) error {
	return t.repo.Delete(ctx, id)
}
