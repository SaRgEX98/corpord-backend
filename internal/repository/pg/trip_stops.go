package pg

import (
	"corpord-api/internal/logger"
	"corpord-api/model"
	"corpord-api/pkg/dbx"
	sq "github.com/Masterminds/squirrel"
	"golang.org/x/net/context"
)

type TripStop interface {
	All(ctx context.Context) ([]*model.TripStop, error)
	ByID(ctx context.Context, id int) (*model.TripStop, error)
	Create(ctx context.Context, tripStop *model.TripStop) error
	Update(ctx context.Context, tripStop *model.TripStopUpdate) error
	Delete(ctx context.Context, id int) error
}

type tripStop struct {
	logger *logger.Logger
	qb     *dbx.QueryBuilder
}

func NewTripStop(logger *logger.Logger, qb *dbx.QueryBuilder) TripStop {
	return &tripStop{
		logger: logger,
		qb:     qb,
	}
}

func (ts *tripStop) All(ctx context.Context) ([]*model.TripStop, error) {
	var tripStops []*model.TripStop
	query, args, err := ts.qb.Sq.Select(
		"trip_id",
		"stop_id",
		"arrival_time",
		"departure_time",
		"stop_order",
		"price_to_next",
	).From(TableTripStop).ToSql()
	if err != nil {
		ts.logger.Error(err)
		return nil, err
	}

	err = ts.qb.DB.SelectContext(ctx, &tripStops, query, args...)
	if err != nil {
		ts.logger.Error(err)
		return nil, err
	}
	return tripStops, nil
}

func (ts *tripStop) ByID(ctx context.Context, id int) (*model.TripStop, error) {
	var stop *model.TripStop
	query, args, err := ts.qb.Sq.Select(
		"trip_id",
		"stop_id",
		"arrival_time",
		"departure_time",
		"stop_order",
		"price_to_next",
	).From(TableTripStop).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		ts.logger.Error(err)
		return nil, err
	}

	err = ts.qb.DB.GetContext(ctx, &stop, query, args...)
	if err != nil {
		ts.logger.Error(err)
		return nil, err
	}
	return stop, nil
}

func (ts *tripStop) Create(ctx context.Context, tripStop *model.TripStop) error {
	query, args, err := ts.qb.Sq.Insert(TableTripStop).Columns(
		"trip_id",
		"stop_id",
		"arrival_time",
		"departure_time",
		"stop_order",
		"price_to_next").
		Values(
			tripStop.TripID,
			tripStop.StopID,
			tripStop.ArrivalTime,
			tripStop.DepartureTime,
			tripStop.StopOrder,
			tripStop.PriceToNext).
		ToSql()
	if err != nil {
		ts.logger.Error(err)
		return err
	}
	_, err = ts.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		ts.logger.Error(err)
		return err
	}
	return nil
}

func (ts *tripStop) Update(ctx context.Context, tripStop *model.TripStopUpdate) error {
	query, args, err := ts.qb.Sq.Update(TableTripStop).
		SetMap(tripStop.ToMap()).
		Where(sq.Eq{"id": tripStop.ID}).
		ToSql()
	if err != nil {
		ts.logger.Error(err)
		return err
	}
	_, err = ts.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		ts.logger.Error(err)
		return err
	}
	return nil
}

func (ts *tripStop) Delete(ctx context.Context, id int) error {
	query, args, err := ts.qb.Sq.Delete(TableTripStop).Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		ts.logger.Error(err)
		return err
	}
	_, err = ts.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		ts.logger.Error(err)
		return err
	}
	return nil
}
