package booking

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"

	"github.com/jmoiron/sqlx"
)

type Service struct{ DB *sqlx.DB }

func New(db *sqlx.DB) *Service { return &Service{DB: db} }

const bookingColumns = `h.id,h.trip_id,h.boarding_stop_id,h.alighting_stop_id,h.boarding_name,h.alighting_name,h.departure_time,h.arrival_time,h.timezone,h.arrival_timezone,h.passenger_name,CASE WHEN h.status='held' AND h.expires_at<=clock_timestamp() THEN 'expired' ELSE h.status END AS status,h.seat_number,h.amount_minor,h.currency,h.is_demo,h.created_at,h.expires_at,h.cancelled_at`
const passengerColumns = `id,full_name,created_at,updated_at`

type trip struct {
	ID       int    `db:"id"`
	Demo     bool   `db:"is_demo"`
	Status   string `db:"status"`
	Capacity int    `db:"capacity"`
}
type segment struct {
	BoardingStopID  int       `db:"boarding_stop_id"`
	AlightingStopID int       `db:"alighting_stop_id"`
	BoardingOrder   int       `db:"boarding_order"`
	AlightingOrder  int       `db:"alighting_order"`
	BoardingName    string    `db:"boarding_name"`
	AlightingName   string    `db:"alighting_name"`
	Departure       time.Time `db:"departure_time"`
	Arrival         time.Time `db:"arrival_time"`
	Timezone        string    `db:"timezone"`
	ArrivalTimezone string    `db:"arrival_timezone"`
	Amount          *int64    `db:"amount_minor"`
}

const segmentQuery = `SELECT a.stop_id boarding_stop_id,b.stop_id alighting_stop_id,a.stop_order boarding_order,b.stop_order alighting_order,sa.name boarding_name,sb.name alighting_name,a.departure_time AT TIME ZONE 'UTC' departure_time,b.arrival_time AT TIME ZONE 'UTC' arrival_time,COALESCE(sa.timezone,ca.timezone,'') timezone,COALESCE(sb.timezone,cb.timezone,'') arrival_timezone,fare.amount_minor FROM trip_stops a JOIN trip_stops b ON b.trip_id=a.trip_id AND b.stop_order>a.stop_order JOIN stops sa ON sa.id=a.stop_id JOIN stops sb ON sb.id=b.stop_id LEFT JOIN route_cities ca ON ca.id=sa.city_id LEFT JOIN route_cities cb ON cb.id=sb.city_id CROSS JOIN LATERAL (SELECT CASE WHEN count(*)=count(price_to_next_minor) AND min(price_to_next_minor)>=0 THEN sum(price_to_next_minor)::bigint END amount_minor FROM trip_stops leg WHERE leg.trip_id=a.trip_id AND leg.stop_order>=a.stop_order AND leg.stop_order<b.stop_order) fare WHERE a.trip_id=$1 AND a.stop_id=$2 AND b.stop_id=$3 ORDER BY a.stop_order,b.stop_order LIMIT 2`

func loadTrip(ctx context.Context, tx *sqlx.Tx, id int, lock bool) (trip, error) {
	var t trip
	q := `SELECT t.id,t.is_demo,t.status,LEAST(i.capacity,b.capacity) capacity FROM trips t JOIN bus b ON b.id=t.bus_id LEFT JOIN booking_inventory i ON i.trip_id=t.id WHERE t.id=$1 AND b.deleted_at IS NULL`
	if lock {
		q += ` FOR UPDATE OF t FOR SHARE OF b`
	}
	err := tx.GetContext(ctx, &t, q, id)
	if errors.Is(err, sql.ErrNoRows) {
		return t, missing()
	}
	return t, err
}
func loadSegment(ctx context.Context, tx *sqlx.Tx, tripID, boarding, alighting int) (segment, error) {
	rows := []segment{}
	err := tx.SelectContext(ctx, &rows, segmentQuery, tripID, boarding, alighting)
	if err != nil {
		return segment{}, err
	}
	if len(rows) == 0 {
		return segment{}, missing()
	}
	if len(rows) != 1 {
		return segment{}, conflict("ambiguous_segment")
	}
	return rows[0], nil
}
func bookable(t trip, s segment, now time.Time) bool {
	if !t.Demo || t.Capacity < 1 || t.Capacity > 1000 || (t.Status != "scheduled" && t.Status != "boarding" && t.Status != "delayed") || !s.Departure.After(now) || !s.Arrival.After(s.Departure) || s.Amount == nil {
		return false
	}
	if s.Timezone == "" || s.ArrivalTimezone == "" {
		return false
	}
	_, err := time.LoadLocation(s.Timezone)
	if err != nil {
		return false
	}
	_, err = time.LoadLocation(s.ArrivalTimezone)
	return err == nil
}
func lockOwner(ctx context.Context, tx *sqlx.Tx, owner int) error {
	if owner < 1 {
		return invalid()
	}
	var id int
	err := tx.GetContext(ctx, &id, `SELECT id FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, owner)
	if errors.Is(err, sql.ErrNoRows) {
		return missing()
	}
	return err
}
func (s *Service) Availability(ctx context.Context, tripID, boarding, alighting int) (Availability, error) {
	out := Availability{TripID: tripID, BoardingStopID: boarding, AlightingStopID: alighting, Status: "unknown", HoldSeconds: HoldSeconds}
	if tripID < 1 || tripID > math.MaxInt32 || boarding < 1 || boarding > math.MaxInt32 || alighting < 1 || alighting > math.MaxInt32 || boarding == alighting {
		return out, invalid()
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTxx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	t, err := loadTrip(ctx, tx, tripID, false)
	if err != nil {
		return out, err
	}
	out.IsDemo = t.Demo
	seg, err := loadSegment(ctx, tx, tripID, boarding, alighting)
	if err != nil {
		return out, err
	}
	var now time.Time
	if err = tx.GetContext(ctx, &now, `SELECT clock_timestamp()`); err != nil {
		return out, err
	}
	if !bookable(t, seg, now) {
		return out, tx.Commit()
	}
	var capacity int
	err = tx.GetContext(ctx, &capacity, `SELECT capacity FROM booking_inventory WHERE trip_id=$1`, tripID)
	if errors.Is(err, sql.ErrNoRows) {
		capacity = t.Capacity
	} else if err != nil {
		return out, err
	}
	capacity = min(capacity, t.Capacity)
	var occupied int
	err = tx.GetContext(ctx, &occupied, `SELECT count(DISTINCT seat_number) FROM booking_holds WHERE trip_id=$1 AND status='held' AND expires_at>$4 AND occupied_range && int4range($2,$3,'[)')`, tripID, seg.BoardingOrder, seg.AlightingOrder, now)
	if err != nil {
		return out, err
	}
	seats := capacity - occupied
	if seats < 0 {
		seats = 0
	}
	out.Seats = &seats
	out.Bookable = seats > 0
	out.Status = "available"
	if seats == 0 {
		out.Status = "sold_out"
	}
	return out, tx.Commit()
}

// Create serializes per owner first (cross-trip hold limit), then per trip
// (seat allocation and timetable snapshot). No price, seat or user comes from
// the client. A committed idempotency key is never reused, even after expiry.
func (s *Service) Create(ctx context.Context, owner int, key string, r CreateRequest) (Booking, bool, error) {
	var out Booking
	if err := r.validate(key); err != nil {
		return out, false, err
	}
	hash := r.hash()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return out, false, err
	}
	defer tx.Rollback()
	if err = lockOwner(ctx, tx, owner); err != nil {
		return out, false, err
	}
	var existing struct {
		Booking
		PayloadHash string `db:"payload_hash"`
	}
	err = tx.GetContext(ctx, &existing, `SELECT `+bookingColumns+`,h.payload_hash FROM booking_holds h WHERE h.user_id=$1 AND h.idempotency_key=$2`, owner, key)
	if err == nil {
		if existing.PayloadHash != hash {
			return out, false, conflict("idempotency_conflict")
		}
		return existing.Booking, false, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return out, false, err
	}
	name := ""
	if r.PassengerID != nil {
		err = tx.GetContext(ctx, &name, `SELECT full_name FROM booking_passengers WHERE id=$1 AND user_id=$2`, *r.PassengerID, owner)
		if errors.Is(err, sql.ErrNoRows) {
			return out, false, missing()
		}
		if err != nil {
			return out, false, err
		}
	} else {
		name = *r.PassengerName
	}
	var active int
	if err = tx.GetContext(ctx, &active, `SELECT count(*) FROM booking_holds WHERE user_id=$1 AND status='held' AND expires_at>clock_timestamp()`, owner); err != nil {
		return out, false, err
	}
	if active >= 5 {
		return out, false, conflict("hold_limit")
	}
	t, err := loadTrip(ctx, tx, r.TripID, true)
	if err != nil {
		return out, false, err
	}
	seg, err := loadSegment(ctx, tx, r.TripID, r.BoardingStopID, r.AlightingStopID)
	if err != nil {
		return out, false, err
	}
	var now time.Time
	if err = tx.GetContext(ctx, &now, `SELECT clock_timestamp()`); err != nil {
		return out, false, err
	}
	if !t.Demo {
		return out, false, conflict("inventory_unavailable")
	}
	if (t.Status != "scheduled" && t.Status != "boarding" && t.Status != "delayed") || !seg.Departure.After(now) {
		return out, false, conflict("trip_unavailable")
	}
	if !bookable(t, seg, now) {
		return out, false, conflict("inventory_unavailable")
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO booking_inventory(trip_id,capacity) VALUES($1,$2) ON CONFLICT(trip_id) DO NOTHING`, r.TripID, t.Capacity); err != nil {
		return out, false, err
	}
	// Expired exclusion entries cannot block reuse when the worker is offline.
	if _, err = tx.ExecContext(ctx, `UPDATE booking_holds SET status='expired' WHERE trip_id=$1 AND status='held' AND expires_at<=$2`, r.TripID, now); err != nil {
		return out, false, err
	}
	var seat int
	err = tx.GetContext(ctx, &seat, `SELECT n FROM booking_inventory i CROSS JOIN LATERAL generate_series(1,LEAST(i.capacity,$5)) n WHERE i.trip_id=$1 AND NOT EXISTS (SELECT 1 FROM booking_holds h WHERE h.trip_id=i.trip_id AND h.status='held' AND h.expires_at>$4 AND h.seat_number=n AND h.occupied_range && int4range($2,$3,'[)')) ORDER BY n LIMIT 1`, r.TripID, seg.BoardingOrder, seg.AlightingOrder, now, t.Capacity)
	if errors.Is(err, sql.ErrNoRows) {
		return out, false, conflict("sold_out")
	}
	if err != nil {
		return out, false, err
	}
	err = tx.GetContext(ctx, &out, `INSERT INTO booking_holds AS h(user_id,trip_id,passenger_id,passenger_name,boarding_stop_id,alighting_stop_id,boarding_order,alighting_order,boarding_name,alighting_name,departure_time,arrival_time,timezone,arrival_timezone,seat_number,amount_minor,is_demo,created_at,expires_at,idempotency_key,payload_hash) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,true,$19,LEAST($19::timestamptz+interval '10 minutes',$11),$17,$18) RETURNING `+bookingColumns, owner, r.TripID, r.PassengerID, name, seg.BoardingStopID, seg.AlightingStopID, seg.BoardingOrder, seg.AlightingOrder, seg.BoardingName, seg.AlightingName, seg.Departure, seg.Arrival, seg.Timezone, seg.ArrivalTimezone, seat, *seg.Amount, key, hash, now)
	if err != nil {
		return out, false, err
	}
	if err = tx.Commit(); err != nil {
		return Booking{}, false, err
	}
	return out, true, nil
}
func (s *Service) Get(ctx context.Context, owner int, id int64) (Booking, error) {
	var out Booking
	if owner < 1 || id < 1 {
		return out, invalid()
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := s.DB.GetContext(ctx, &out, `SELECT `+bookingColumns+` FROM booking_holds h WHERE h.id=$1 AND h.user_id=$2`, id, owner)
	if errors.Is(err, sql.ErrNoRows) {
		err = missing()
	}
	return out, err
}
func (s *Service) List(ctx context.Context, owner, limit int, cursor string) (Page[Booking], error) {
	out := Page[Booking]{Items: []Booking{}}
	after, limit, err := pagePosition(owner, "bookings", cursor, limit)
	if err != nil {
		return out, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// All display values are immutable snapshots. Exactly one query per page.
	q := `SELECT ` + bookingColumns + ` FROM booking_holds h WHERE h.user_id=$1`
	args := []any{owner, limit + 1}
	if after > 0 {
		q += ` AND h.id<$3`
		args = append(args, after)
	}
	q += ` ORDER BY h.id DESC LIMIT $2`
	if err = s.DB.SelectContext(ctx, &out.Items, q, args...); err != nil {
		return out, err
	}
	if len(out.Items) > limit {
		out.Items = out.Items[:limit]
		out.NextCursor = pageCursor(owner, "bookings", out.Items[limit-1].ID)
	}
	return out, nil
}
func (s *Service) Cancel(ctx context.Context, owner int, id int64) (Booking, error) {
	var out Booking
	if owner < 1 || id < 1 {
		return out, invalid()
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if err = lockOwner(ctx, tx, owner); err != nil {
		return out, err
	}
	err = tx.GetContext(ctx, &out, `WITH db_now AS MATERIALIZED (SELECT clock_timestamp() AS at) UPDATE booking_holds AS h SET status=CASE WHEN status='held' AND expires_at<=db_now.at THEN 'expired' WHEN status='held' THEN 'cancelled' ELSE status END,cancelled_at=CASE WHEN status='held' AND expires_at>db_now.at THEN db_now.at ELSE cancelled_at END FROM db_now WHERE h.id=$1 AND h.user_id=$2 RETURNING `+bookingColumns, id, owner)
	if errors.Is(err, sql.ErrNoRows) {
		return out, missing()
	}
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}

// Expire is a stateless bounded batch suitable for a cancellable scheduler and
// multiple workers. Correct stock does not depend on this cleanup running.
func (s *Service) Expire(ctx context.Context, limit int) error {
	if limit < 1 || limit > 1000 {
		return invalid()
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := s.DB.ExecContext(ctx, `WITH due AS (SELECT id FROM booking_holds WHERE status='held' AND expires_at<=clock_timestamp() ORDER BY expires_at,id FOR UPDATE SKIP LOCKED LIMIT $1) UPDATE booking_holds h SET status='expired' FROM due WHERE h.id=due.id`, limit)
	return err
}
