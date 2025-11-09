package pg

import (
	"corpord-api/internal/logger"
	"corpord-api/model"
	"corpord-api/pkg/dbx"
	sq "github.com/Masterminds/squirrel"
	"golang.org/x/net/context"
)

type Trip interface {
	All(ctx context.Context) ([]*model.TripResponse, error)
	AllShort(ctx context.Context) ([]*model.TripShortInfo, error)
	ByID(ctx context.Context, id int) (*model.TripResponse, error)
	Create(ctx context.Context, trip *model.Trip) error
	Update(ctx context.Context, trip *model.TripUpdate) error
	Delete(ctx context.Context, id int) error
}

type trip struct {
	logger *logger.Logger
	qb     *dbx.QueryBuilder
}

func NewTrip(logger *logger.Logger, qb *dbx.QueryBuilder) Trip {
	return &trip{
		logger: logger,
		qb:     qb,
	}
}

func (t *trip) All(ctx context.Context) ([]*model.TripResponse, error) {
	query, args, err := t.qb.Sq.Select(
		"b.license_plate",
		"b.brand",
		"b.capacity",
		"bc.name as category_name",
		"bs.name as status_name",
		"d.first_name",
		"d.last_name",
		"d.middle_name",
		"d.phone_number",
		"ds.name as d_status_name",
		"start_time",
		"end_time",
		"trips.status",
		"base_price",
		"(SELECT JSON_AGG(name, address, latitude, longitude) FROM stops WHERE id = 1)",
		"trips.created_at",
		"trips.updated_at").
		From(TableTrip).
		Join("bus b ON b.id = trips.bus_id").
		Join("bus_statuses bs ON bs.id = b.status_id").
		Join("bus_categories bc ON bc.id = b.category_id").
		Join("drivers d ON d.id = trips.driver_id").
		Join("driver_status ds ON ds.id = d.status").
		Join("trip_stops ts ON trips.id = ts.trip_id").
		Join("stops s ON s.id = ts.stop_id").
		ToSql()
	if err != nil {
		t.logger.Error(err)
		return nil, err
	}
	var trips []*model.TripResponse
	err = t.qb.DB.SelectContext(ctx, &trips, query, args...)
	if err != nil {
		t.logger.Error(err)
		return nil, err
	}
	return trips, nil
}

func (t *trip) AllShort(ctx context.Context) ([]*model.TripShortInfo, error) {
	var result []*model.TripShortInfo
	query, args, err := t.qb.Sq.Select(
		"trips.id AS trip_id",
		"b.license_plate",
		"b.brand",
		"d.first_name || ' ' || d.last_name AS driver_name",
		"s_start.name AS start_stop",
		"s_end.name AS end_stop",
		"ts_start.arrival_time AS start_time",
		"ts_end.departure_time AS end_time",
		"base_price").
		From(TableTrip).
		Join("trip_stops ts_start ON ts_start.trip_id = trips.id AND ts_start.stop_order = 1").
		Join("trip_stops ts_end ON ts_end.trip_id = trips.id AND ts_end.stop_order = (SELECT MAX(ts2.stop_order) FROM trip_stops ts2 WHERE ts2.trip_id = trips.id)").
		Join("stops s_start ON s_start.id = ts_start.stop_id").
		Join("stops s_end ON s_end.id = ts_end.stop_id").
		Join("bus b ON b.id = trips.bus_id").
		Join("drivers d ON d.id = trips.driver_id").
		ToSql()
	if err != nil {
		t.logger.Error(err)
		return nil, err
	}
	err = t.qb.DB.SelectContext(ctx, &result, query, args...)
	if err != nil {
		t.logger.Error(err)
		return nil, err
	}

	return result, nil
}

func (t *trip) ByID(ctx context.Context, id int) (*model.TripResponse, error) {
	query, args, err := t.qb.Sq.Select(
		"b.license_plate",
		"b.brand",
		"b.capacity",
		"bc.name as category_name",
		"bs.name as status_name",
		"d.first_name",
		"d.last_name",
		"d.middle_name",
		"d.phone_number",
		"ds.name as d_status_name",
		"start_time",
		"end_time",
		"trips.status",
		"base_price",
		"trips.created_at",
		"trips.updated_at").
		From(TableTrip).
		Join("bus b ON b.id = trips.bus_id").
		Join("bus_statuses bs ON bs.id = b.status_id").
		Join("bus_categories bc ON bc.id = b.category_id").
		Join("drivers d ON d.id = trips.driver_id").
		Join("driver_status ds ON ds.id = d.status").
		ToSql()
	if err != nil {
		t.logger.Error(err)
		return nil, err
	}
	var result model.TripResponse
	err = t.qb.DB.GetContext(ctx, &result, query, args...)
	if err != nil {
		t.logger.Error(err)
		return nil, err
	}
	return &result, nil
}

func (t *trip) Create(ctx context.Context, trip *model.Trip) error {
	query, args, err := t.qb.Sq.Insert(TableTrip).Columns(
		"bus_id",
		"driver_id",
		"start_time",
		"end_time",
		"status",
		"base_price").
		Values(trip.BusID, trip.DriverID, trip.StartTime, trip.EndTime, trip.Status, trip.BasePrice).
		ToSql()
	if err != nil {
		t.logger.Error(err)
		return err
	}
	_, err = t.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		t.logger.Error(err)
		return err
	}
	return nil
}

func (t *trip) Update(ctx context.Context, trip *model.TripUpdate) error {
	query, args, err := t.qb.Sq.Update(TableTrip).
		SetMap(trip.ToMap()).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"trips.id": trip.ID}).
		ToSql()
	if err != nil {
		t.logger.Error(err)
		return err
	}
	_, err = t.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		t.logger.Error(err)
		return err
	}
	return nil
}

func (t *trip) Delete(ctx context.Context, id int) error {
	query, args, err := t.qb.Sq.Delete(TableTrip).
		Where(sq.Eq{"trips.id": id}).
		ToSql()
	if err != nil {
		t.logger.Error(err)
		return err
	}
	_, err = t.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		t.logger.Error(err)
		return err
	}
	return nil
}
