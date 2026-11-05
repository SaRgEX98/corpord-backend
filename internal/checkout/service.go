package checkout

import (
	"context"
	"corpord-api/internal/payments"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Service struct {
	DB       *sqlx.DB
	provider payments.Provider
	options  Options
}

func New(db *sqlx.DB, provider payments.Provider, opts Options) *Service {
	return &Service{DB: db, provider: provider, options: opts}
}

const summaryColumns = `o.id,o.hold_id,o.trip_id,o.status,o.payment_status,o.amount_minor,o.currency,o.is_demo,o.test,o.passenger_name,o.boarding_name,o.alighting_name,o.departure_time,o.arrival_time,o.timezone,o.arrival_timezone,o.seat_number,o.created_at,o.updated_at,o.hold_expires_at`

type record struct {
	Summary
	Owner           int       `db:"user_id"`
	CancelRequested bool      `db:"cancel_requested"`
	HoldStatus      string    `db:"hold_status"`
	IntentID        string    `db:"intent_id"`
	ProviderID      *string   `db:"provider_id"`
	ProviderStatus  *string   `db:"provider_status"`
	MerchantID      *string   `db:"merchant_id"`
	ConfirmationURL *string   `db:"confirmation_url"`
	RefundID        *string   `db:"refund_id"`
	RefundStatus    *string   `db:"refund_status"`
	CreateKey       string    `db:"create_key"`
	CaptureKey      string    `db:"capture_key"`
	CancelKey       string    `db:"cancel_key"`
	RefundKey       string    `db:"refund_key"`
	Now             time.Time `db:"db_now"`
}

const recordQuery = `SELECT ` + summaryColumns + `,o.user_id,o.cancel_requested,h.status hold_status,a.intent_id,a.provider_id,a.provider_status,a.merchant_id,a.confirmation_url,a.refund_id,a.refund_status,a.create_key,a.capture_key,a.cancel_key,a.refund_key,clock_timestamp() db_now FROM checkout_orders o JOIN booking_holds h ON h.id=o.hold_id JOIN checkout_attempts a ON a.order_id=o.id`

func (s *Service) decorate(r record) Detail {
	d := Detail{Summary: r.Summary}
	live := r.HoldStatus == "held" && r.HoldExpiresAt.After(r.Now) && r.DepartureTime.After(r.Now)
	d.Actions.CanPay = s.options.Enabled && r.Status == "awaiting_payment" && !r.CancelRequested && live
	d.Actions.CanCancel = (r.Status == "awaiting_payment" || r.Status == "cancel_pending") && r.HoldStatus != "payment_committing" && r.HoldStatus != "confirmed"
	d.Actions.CanReconcile = s.options.Enabled && !terminal(r.Status)
	d.Actions.CanViewConfirmation = d.Actions.CanPay && r.ConfirmationURL != nil && validConfirmation(*r.ConfirmationURL)
	if d.Actions.CanViewConfirmation {
		d.Payment.ConfirmationURL = r.ConfirmationURL
	}
	if !terminal(r.Status) && r.Status != "review_required" {
		d.NextPollAfterMS = 2000
	}
	return d
}
func validConfirmation(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && len(raw) <= 4096
}
func (s *Service) Get(ctx context.Context, owner int, id int64) (Detail, error) {
	if owner < 1 || id < 1 {
		return Detail{}, failure(400, "invalid_request")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var r record
	err := s.DB.GetContext(ctx, &r, recordQuery+` WHERE o.id=$1 AND o.user_id=$2`, id, owner)
	if errors.Is(err, sql.ErrNoRows) {
		return Detail{}, failure(404, "not_found")
	}
	if err != nil {
		return Detail{}, err
	}
	return s.decorate(r), nil
}
func (s *Service) List(ctx context.Context, owner, limit int, status, cursor string) (Page, error) {
	out := Page{Items: []Summary{}}
	if owner < 1 || limit < 0 || limit > 100 || (status != "" && !statuses[status]) {
		return out, failure(400, "invalid_request")
	}
	if limit == 0 {
		limit = 30
	}
	after, err := decodeCursor(owner, status, cursor)
	if err != nil {
		return out, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	q := recordQuery + ` WHERE o.user_id=$1`
	args := []any{owner, limit + 1}
	if status != "" {
		args = append(args, status)
		q += fmt.Sprintf(" AND o.status=$%d", len(args))
	}
	if after > 0 {
		args = append(args, after)
		q += fmt.Sprintf(" AND o.id<$%d", len(args))
	}
	q += ` ORDER BY o.id DESC LIMIT $2`
	rows := []record{}
	if err = s.DB.SelectContext(ctx, &rows, q, args...); err != nil {
		return out, err
	}
	if len(rows) > limit {
		rows = rows[:limit]
		out.NextCursor = encodeCursor(owner, status, rows[limit-1].ID)
	}
	for _, r := range rows {
		out.Items = append(out.Items, s.decorate(r).Summary)
	}
	return out, nil
}
func lockUser(ctx context.Context, tx *sqlx.Tx, owner int) error {
	var id int
	err := tx.GetContext(ctx, &id, `SELECT id FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, owner)
	if errors.Is(err, sql.ErrNoRows) {
		return failure(404, "not_found")
	}
	return err
}

// Every business transition uses this order: owner → trip/bus → hold → order.
func lockOrder(ctx context.Context, tx *sqlx.Tx, owner int, id int64) (record, error) {
	var r record
	var ids struct {
		Owner int   `db:"user_id"`
		Trip  int   `db:"trip_id"`
		Hold  int64 `db:"hold_id"`
	}
	q := `SELECT user_id,trip_id,hold_id FROM checkout_orders WHERE id=$1`
	args := []any{id}
	if owner > 0 {
		q += ` AND user_id=$2`
		args = append(args, owner)
	}
	if err := tx.GetContext(ctx, &ids, q, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return r, failure(404, "not_found")
		}
		return r, err
	}
	if owner == 0 {
		// Financial recovery must survive soft deletion of the account. Interactive
		// requests still require the existing active-account auth and owner lock.
		var locked int
		if err := tx.GetContext(ctx, &locked, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, ids.Owner); err != nil {
			return r, err
		}
	} else if err := lockUser(ctx, tx, ids.Owner); err != nil {
		return r, err
	}
	var lock int64
	if err := tx.GetContext(ctx, &lock, `SELECT t.id FROM trips t JOIN bus b ON b.id=t.bus_id WHERE t.id=$1 FOR UPDATE OF t FOR SHARE OF b`, ids.Trip); err != nil {
		return r, err
	}
	if err := tx.GetContext(ctx, &lock, `SELECT id FROM booking_holds WHERE id=$1 FOR UPDATE`, ids.Hold); err != nil {
		return r, err
	}
	if err := tx.GetContext(ctx, &r, recordQuery+` WHERE o.id=$1 FOR UPDATE OF o,a`, id); err != nil {
		return r, err
	}
	return r, nil
}
func enqueue(ctx context.Context, tx *sqlx.Tx, order int64, kind string, delay time.Duration) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO checkout_jobs(order_id,kind,available_at) VALUES($1,$2,clock_timestamp()+($3*interval '1 millisecond')) ON CONFLICT(order_id,kind) DO UPDATE SET state=CASE WHEN checkout_jobs.state='leased' THEN 'leased' ELSE 'pending' END,available_at=CASE WHEN checkout_jobs.state='done' THEN EXCLUDED.available_at ELSE LEAST(checkout_jobs.available_at,EXCLUDED.available_at) END`, order, kind, delay.Milliseconds())
	return err
}
func (s *Service) Create(ctx context.Context, owner int, key string, holdID int64) (Detail, bool, error) {
	if owner < 1 || holdID < 1 || !keyPattern.MatchString(key) {
		return Detail{}, false, failure(400, "invalid_request")
	}
	if !s.options.Enabled || s.provider == nil {
		return Detail{}, false, failure(503, "payments_disabled")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return Detail{}, false, err
	}
	defer tx.Rollback()
	if err = lockUser(ctx, tx, owner); err != nil {
		return Detail{}, false, err
	}
	var prev struct {
		ID     int64
		HoldID int64 `db:"hold_id"`
	}
	err = tx.GetContext(ctx, &prev, `SELECT id,hold_id FROM checkout_orders WHERE user_id=$1 AND idempotency_key=$2`, owner, key)
	if err == nil {
		if prev.HoldID != holdID {
			return Detail{}, false, failure(409, "idempotency_conflict")
		}
		if err = tx.Commit(); err != nil {
			return Detail{}, false, err
		}
		d, err := s.Get(ctx, owner, prev.ID)
		return d, false, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Detail{}, false, err
	}
	var hold struct {
		TripID     int `db:"trip_id"`
		Status     string
		Expires    time.Time `db:"expires_at"`
		Departure  time.Time `db:"departure_time"`
		Amount     int64     `db:"amount_minor"`
		Currency   string
		Demo       bool      `db:"is_demo"`
		CheckoutID *int64    `db:"checkout_id"`
		Now        time.Time `db:"db_now"`
	}
	if err = tx.GetContext(ctx, &hold, `SELECT trip_id FROM booking_holds WHERE id=$1 AND user_id=$2`, holdID, owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Detail{}, false, failure(404, "not_found")
		}
		return Detail{}, false, err
	}
	var trip struct {
		ID      int
		Demo    bool `db:"is_demo"`
		Status  string
		Deleted *time.Time `db:"deleted_at"`
	}
	if err = tx.GetContext(ctx, &trip, `SELECT t.id,t.is_demo,t.status,b.deleted_at FROM trips t JOIN bus b ON b.id=t.bus_id WHERE t.id=$1 FOR UPDATE OF t FOR SHARE OF b`, hold.TripID); err != nil {
		return Detail{}, false, err
	}
	if err = tx.GetContext(ctx, &hold, `SELECT trip_id,status,expires_at,departure_time,amount_minor,currency,is_demo,checkout_id,clock_timestamp() db_now FROM booking_holds WHERE id=$1 AND user_id=$2 FOR UPDATE`, holdID, owner); err != nil {
		return Detail{}, false, err
	}
	if hold.CheckoutID != nil {
		return Detail{}, false, failure(409, "checkout_exists")
	}
	if hold.Status != "held" || !hold.Expires.After(hold.Now) || !hold.Departure.After(hold.Now) {
		return Detail{}, false, failure(409, "hold_expired")
	}
	if !hold.Demo || !trip.Demo || trip.Deleted != nil || (trip.Status != "scheduled" && trip.Status != "boarding" && trip.Status != "delayed") || hold.Currency != "RUB" || hold.Amount < 100 || hold.Amount > 35000000 {
		return Detail{}, false, failure(409, "not_payable")
	}
	var id int64
	err = tx.GetContext(ctx, &id, `INSERT INTO checkout_orders(user_id,hold_id,trip_id,amount_minor,currency,is_demo,passenger_name,boarding_name,alighting_name,departure_time,arrival_time,timezone,arrival_timezone,seat_number,hold_expires_at,idempotency_key) SELECT user_id,id,trip_id,amount_minor,currency,is_demo,passenger_name,boarding_name,alighting_name,departure_time,arrival_time,timezone,arrival_timezone,seat_number,expires_at,$3 FROM booking_holds WHERE id=$1 AND user_id=$2 RETURNING id`, holdID, owner, key)
	if err != nil {
		return Detail{}, false, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO checkout_attempts(order_id,intent_id,create_key,capture_key,cancel_key,refund_key) VALUES($1,$2,$3,$4,$5,$6)`, id, fmt.Sprint(id), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString())
	if err != nil {
		return Detail{}, false, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE booking_holds SET checkout_id=$2 WHERE id=$1`, holdID, id); err != nil {
		return Detail{}, false, err
	}
	if err = enqueue(ctx, tx, id, "create", 0); err != nil {
		return Detail{}, false, err
	}
	if err = tx.Commit(); err != nil {
		return Detail{}, false, err
	}
	d, err := s.Get(ctx, owner, id)
	return d, true, err
}
func (s *Service) Cancel(ctx context.Context, owner int, id int64) (Detail, error) {
	if owner < 1 || id < 1 {
		return Detail{}, failure(400, "invalid_request")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return Detail{}, err
	}
	defer tx.Rollback()
	r, err := lockOrder(ctx, tx, owner, id)
	if err != nil {
		return Detail{}, err
	}
	if r.Status == "paid" || r.HoldStatus == "confirmed" {
		return Detail{}, failure(409, "paid_order")
	}
	if r.Status == "committing" || r.HoldStatus == "payment_committing" || r.Status == "review_required" || r.Status == "refund_pending" {
		return Detail{}, failure(409, "checkout_in_progress")
	}
	if r.Status != "cancelled" && r.Status != "expired" && r.Status != "refunded" {
		if _, err = tx.ExecContext(ctx, `UPDATE checkout_orders SET status='cancel_pending',cancel_requested=true,updated_at=clock_timestamp() WHERE id=$1`, id); err != nil {
			return Detail{}, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE booking_holds SET status=CASE WHEN expires_at<=$2 THEN 'expired' ELSE 'cancelled' END,cancelled_at=CASE WHEN expires_at>$2 THEN $2 ELSE cancelled_at END WHERE id=$1 AND status='held'`, r.HoldID, r.Now); err != nil {
			return Detail{}, err
		}
		kind := "reconcile"
		if r.ProviderID == nil {
			kind = "create"
		}
		if err = enqueue(ctx, tx, id, kind, 0); err != nil {
			return Detail{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return Detail{}, err
	}
	return s.Get(ctx, owner, id)
}
func (s *Service) Reconcile(ctx context.Context, owner int, id int64) error {
	if owner < 1 || id < 1 {
		return failure(400, "invalid_request")
	}
	if !s.options.Enabled || s.provider == nil {
		return failure(503, "payments_disabled")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := lockOrder(ctx, tx, owner, id)
	if err != nil {
		return err
	}
	var accepted int64
	err = tx.GetContext(ctx, &accepted, `UPDATE checkout_orders SET last_reconcile_at=clock_timestamp() WHERE id=$1 AND (last_reconcile_at IS NULL OR last_reconcile_at<clock_timestamp()-interval '5 seconds') RETURNING id`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return failure(429, "rate_limited")
	}
	if err != nil {
		return err
	}
	if !terminal(r.Status) {
		kind := "reconcile"
		if r.ProviderID == nil {
			kind = "create"
		}
		if r.Status == "refund_pending" {
			kind = "refund"
			if r.RefundID != nil {
				kind = "reconcile_refund"
			}
		}
		if err = enqueue(ctx, tx, id, kind, 0); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Service) IngestWebhook(ctx context.Context, event, id string) error {
	if !s.options.Enabled || s.provider == nil {
		return failure(503, "payments_disabled")
	}
	if (event != "payment.waiting_for_capture" && event != "payment.succeeded" && event != "payment.canceled" && event != "refund.succeeded") || len(id) < 1 || len(id) > 128 || strings.ContainsAny(id, "\r\n\x00") {
		return failure(400, "invalid_request")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	column := "provider_id"
	if event == "refund.succeeded" {
		column = "refund_id"
	}
	// Public notifications are hints about our own persisted operations, never an
	// authenticated provider lookup proxy or storage for arbitrary foreign IDs.
	_, err := s.DB.ExecContext(ctx, `INSERT INTO checkout_inbox(event,object_id) SELECT $1,$2 WHERE EXISTS(SELECT 1 FROM checkout_attempts WHERE `+column+`=$2) ON CONFLICT(event,object_id) DO NOTHING`, event, id)
	return err
}
