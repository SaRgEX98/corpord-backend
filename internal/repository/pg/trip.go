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

// selectTrips keeps trip, bus and driver identifiers separate and aggregates stops
// in one query without duplicating trips that have several stops.
func (t *trip) selectTrips() sq.SelectBuilder {
	return t.qb.Sq.Select(
		"trips.id", "b.id AS \"bus.id\"", "b.license_plate AS \"bus.license_plate\"",
		"b.brand AS \"bus.brand\"", "b.capacity AS \"bus.capacity\"",
		"bc.name AS \"bus.category_name\"", "bs.name AS \"bus.status_name\"",
		"d.id AS \"driver.id\"", "d.first_name AS \"driver.first_name\"",
		"d.last_name AS \"driver.last_name\"", "d.middle_name AS \"driver.middle_name\"",
		"d.phone_number AS \"driver.phone_number\"", "ds.name AS \"driver.d_status_name\"",
		"trips.start_time", "trips.end_time", "trips.status", "trips.base_price",
		`COALESCE((SELECT jsonb_agg(jsonb_build_object(
            'id', ts.id, 'trip_id', ts.trip_id, 'stop_id', ts.stop_id,
            'arrival_time', to_char(ts.arrival_time, 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
            'departure_time', to_char(ts.departure_time, 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
            'stop_order', ts.stop_order, 'price_to_next', COALESCE(ts.price_to_next, 0),
            'stop', jsonb_build_object('id', s.id, 'name', s.name, 'address', s.address,
                'latitude', s.latitude, 'longitude', s.longitude)
        ) ORDER BY ts.stop_order, ts.id) FROM trip_stops ts JOIN stops s ON s.id = ts.stop_id
            WHERE ts.trip_id = trips.id), '[]'::jsonb) AS stops`,
		"trips.created_at", "trips.updated_at").
		From(TableTrip).
		Join("bus b ON b.id = trips.bus_id").
		Join("bus_statuses bs ON bs.id = b.status_id").
		Join("bus_categories bc ON bc.id = b.category_id").
		Join("drivers d ON d.id = trips.driver_id").
		Join("driver_status ds ON ds.id = d.status")
}

func (t *trip) All(ctx context.Context) ([]*model.TripResponse, error) {
	query, args, err := t.selectTrips().OrderBy("trips.id").ToSql()
	if err != nil {
		return nil, err
	}
	trips := make([]*model.TripResponse, 0)
	if err := t.qb.DB.SelectContext(ctx, &trips, query, args...); err != nil {
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
	query, args, err := t.selectTrips().Where(sq.Eq{"trips.id": id}).ToSql()
	if err != nil {
		return nil, err
	}
	var result model.TripResponse
	if err := t.qb.DB.GetContext(ctx, &result, query, args...); err != nil {
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
		Suffix("RETURNING id, created_at, updated_at").
		ToSql()
	if err != nil {
		t.logger.Error(err)
		return err
	}
	err = t.qb.DB.QueryRowxContext(ctx, query, args...).Scan(&trip.ID, &trip.CreatedAt, &trip.UpdatedAt)
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
	err = execDomainMutation(ctx, t.qb, query, args...)
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
	err = execDomainMutation(ctx, t.qb, query, args...)
	if err != nil {
		t.logger.Error(err)
		return err
	}
	return nil
}
