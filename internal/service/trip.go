package service

import (
	"corpord-api/internal/logger"
	"corpord-api/internal/repository/pg"
	"corpord-api/model"
	"golang.org/x/net/context"
)

type Trip interface {
	All(ctx context.Context) ([]*model.TripResponse, error)
	AllShort(ctx context.Context) ([]*model.TripShortInfo, error)
	ById(ctx context.Context, id int) (*model.TripResponse, error)
	Create(ctx context.Context, trip *model.Trip) error
	Update(ctx context.Context, trip *model.TripUpdate) error
	Delete(ctx context.Context, id int) error
}

type trip struct {
	logger *logger.Logger
	repo   pg.Trip
}

func NewTrip(logger *logger.Logger, repo pg.Trip) Trip {
	return &trip{
		logger: logger,
		repo:   repo,
	}
}

func (t *trip) All(ctx context.Context) ([]*model.TripResponse, error) {
	return t.repo.All(ctx)
}

func (t *trip) AllShort(ctx context.Context) ([]*model.TripShortInfo, error) {
	return t.repo.AllShort(ctx)
}

func (t *trip) ById(ctx context.Context, id int) (*model.TripResponse, error) {
	return t.repo.ByID(ctx, id)
}

func (t *trip) Create(ctx context.Context, trip *model.Trip) error {
	if trip.Status == "" {
		trip.Status = "scheduled"
	}
	if err := trip.Validate(); err != nil {
		return err
	}
	return t.repo.Create(ctx, trip)
}

func (t *trip) Update(ctx context.Context, trip *model.TripUpdate) error {
	if err := trip.Validate(); err != nil {
		return err
	}
	if trip.StartTime != nil || trip.EndTime != nil {
		current, err := t.repo.ByID(ctx, trip.ID)
		if err != nil {
			return err
		}
		start, end := current.StartTime, current.EndTime
		if trip.StartTime != nil {
			start = *trip.StartTime
		}
		if trip.EndTime != nil {
			end = trip.EndTime
		}
		if end != nil && !end.After(start) {
			return &model.ValidationError{Message: "end_time must be after start_time"}
		}
	}
	return t.repo.Update(ctx, trip)
}

func (t *trip) Delete(ctx context.Context, id int) error {
	return t.repo.Delete(ctx, id)
}
