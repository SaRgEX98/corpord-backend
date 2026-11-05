package booking

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

type queryCounter struct{ n atomic.Int64 }

func (q *queryCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	q.n.Add(1)
	return ctx
}
func (*queryCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// This fixture never connects to application configuration. Each test owns a
// fresh random schema inside an explicitly supplied disposable PostgreSQL DB.
func fixture(t *testing.T, capacity int) (*Service, *queryCounter) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL required for isolated PostgreSQL tests")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin := sqlx.NewDb(stdlib.OpenDB(*cfg), "pgx")
	tx, err := admin.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`SELECT pg_advisory_xact_lock(1146376787); CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public; CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public; CREATE EXTENSION IF NOT EXISTS btree_gist WITH SCHEMA public`); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	schema := "booking_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	cfg.RuntimeParams["search_path"] = schema + ",public"
	cfg.RuntimeParams["timezone"] = "UTC"
	cfg.RuntimeParams["jit"] = "off"
	count := &queryCounter{}
	cfg.Tracer = count
	db := sqlx.NewDb(stdlib.OpenDB(*cfg), "pgx")
	db.SetMaxOpenConns(14)
	t.Cleanup(func() {
		db.Close()
		if _, err := admin.Exec("DROP SCHEMA " + schema + " CASCADE"); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	paths, err := filepath.Glob("../../db/migrations/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		up := strings.Split(string(b), "-- +goose Down")[0]
		up = strings.ReplaceAll(up, `CREATE EXTENSION "uuid-ossp";`, "")
		if _, err = db.Exec(up); err != nil {
			t.Fatalf("migration %s: %v", p, err)
		}
	}
	mustExec(t, db, `INSERT INTO users(id,email,password_hash,name) SELECT n,'booking-'||n||'@example.invalid','test-only','Test User' FROM generate_series(1,24)n;
 INSERT INTO bus(id,license_plate,brand,capacity,category_id,status_id) VALUES(1,'BOOKING-TEST','Test',`+fmt.Sprint(capacity)+`,1,1);
 INSERT INTO drivers(id,first_name,last_name,middle_name,phone_number,status) VALUES(1,'Test','Driver','','0',1);
 INSERT INTO stops(id,name,timezone,city_id) VALUES(1,'Начало','Asia/Yekaterinburg',2),(2,'Пересадка','Asia/Yekaterinburg',2),(3,'Конец','Europe/Moscow',1);
 INSERT INTO trips(id,bus_id,driver_id,start_time,end_time,status,base_price,is_demo) SELECT n,1,1,(clock_timestamp()+interval '1 day') AT TIME ZONE 'UTC',(clock_timestamp()+interval '1 day 3 hours') AT TIME ZONE 'UTC','scheduled',300.50,true FROM generate_series(1,12)n;
 INSERT INTO trip_stops(trip_id,stop_id,arrival_time,departure_time,stop_order,price_to_next) SELECT t.id,n,(t.start_time+(n-1)*interval '1 hour'),(t.start_time+(n-1)*interval '1 hour'),n,CASE WHEN n=1 THEN 100.12 WHEN n=2 THEN 200.38 ELSE NULL END FROM trips t CROSS JOIN generate_series(1,3)n;`)
	return New(db), count
}
func mustExec(t *testing.T, db *sqlx.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatal(err)
	}
}
func request(tripID, from, to int) CreateRequest {
	name := "Иван Тестовый"
	return CreateRequest{TripID: tripID, BoardingStopID: from, AlightingStopID: to, PassengerName: &name}
}
func errorCode(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

func TestConcurrentLastSeatAndAdjacentSegments(t *testing.T) {
	s, _ := fixture(t, 1)
	ctx := context.Background()
	// A read must not materialize inventory for any of the twelve trips.
	av, err := s.Availability(ctx, 1, 1, 2)
	if err != nil || av.Seats == nil || *av.Seats != 1 || !av.Bookable {
		t.Fatalf("availability %+v %v", av, err)
	}
	var n int
	if err = s.DB.Get(&n, `SELECT count(*) FROM booking_inventory`); err != nil || n != 0 {
		t.Fatalf("eager inventory %d %v", n, err)
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	var created atomic.Int32
	failures := make(chan error, 16)
	for user := 1; user <= 16; user++ {
		wg.Add(1)
		go func(user int) {
			defer wg.Done()
			<-start
			_, fresh, err := s.Create(ctx, user, fmt.Sprintf("last-seat-%d", user), request(1, 1, 2))
			if err == nil && fresh {
				created.Add(1)
			} else if errorCode(err) != "sold_out" {
				failures <- fmt.Errorf("user %d unexpected %v", user, err)
			}
		}(user)
	}
	close(start)
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	if created.Load() != 1 {
		t.Fatalf("last seat sold %d times", created.Load())
	}
	av, err = s.Availability(ctx, 1, 1, 3)
	if err != nil || av.Status != "sold_out" || *av.Seats != 0 {
		t.Fatalf("overlapping availability %+v %v", av, err)
	}
	adjacent, fresh, err := s.Create(ctx, 17, "adjacent-slot", request(1, 2, 3))
	if err != nil || !fresh || adjacent.SeatNumber != 1 {
		t.Fatalf("adjacent interval cannot reuse seat: %+v %v", adjacent, err)
	}
	if _, _, err = s.Create(ctx, 18, "overlap-slot", request(1, 1, 3)); errorCode(err) != "sold_out" {
		t.Fatalf("overlap accepted %v", err)
	}
	// Exclusion is also a DB invariant if another writer bypasses the service.
	_, err = s.DB.Exec(`INSERT INTO booking_holds(user_id,trip_id,passenger_name,boarding_stop_id,alighting_stop_id,boarding_order,alighting_order,boarding_name,alighting_name,departure_time,arrival_time,timezone,arrival_timezone,seat_number,amount_minor,is_demo,status,created_at,expires_at,idempotency_key,payload_hash) SELECT 20,trip_id,passenger_name,boarding_stop_id,alighting_stop_id,boarding_order,alighting_order,boarding_name,alighting_name,departure_time,arrival_time,timezone,arrival_timezone,seat_number,amount_minor,is_demo,status,created_at,expires_at,'bypass-service',payload_hash FROM booking_holds WHERE id=$1`, adjacent.ID)
	var pgerr *pgconn.PgError
	if !errors.As(err, &pgerr) || pgerr.Code != "23P01" {
		t.Fatalf("exclusion missing: %v", err)
	}
}
func TestConcurrentOwnerLimitAcrossTripsAndIdempotency(t *testing.T) {
	s, _ := fixture(t, 8)
	ctx := context.Background()
	start := make(chan struct{})
	var wg sync.WaitGroup
	var created atomic.Int32
	failures := make(chan error, 10)
	for tripID := 1; tripID <= 10; tripID++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			<-start
			_, fresh, err := s.Create(ctx, 1, fmt.Sprintf("cross-trip-%d", id), request(id, 1, 3))
			if err == nil && fresh {
				created.Add(1)
			} else if errorCode(err) != "hold_limit" {
				failures <- fmt.Errorf("trip %d unexpected %v", id, err)
			}
		}(tripID)
	}
	close(start)
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	if created.Load() != 5 {
		t.Fatalf("owner cap: %d", created.Load())
	}
	// Same owner/key races are serialized even when the original request is in flight.
	ids := make(chan int64, 12)
	start = make(chan struct{})
	created.Store(0)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			b, fresh, err := s.Create(ctx, 2, "retry-same-key", request(12, 1, 3))
			if err != nil {
				t.Errorf("retry %v", err)
				return
			}
			if fresh {
				created.Add(1)
			}
			ids <- b.ID
		}()
	}
	close(start)
	wg.Wait()
	close(ids)
	var first int64
	for id := range ids {
		if first == 0 {
			first = id
		}
		if id != first {
			t.Errorf("different booking for retry %d/%d", id, first)
		}
	}
	if created.Load() != 1 {
		t.Fatalf("idempotent created %d", created.Load())
	}
	if _, _, err := s.Create(ctx, 2, "retry-same-key", request(11, 1, 3)); errorCode(err) != "idempotency_conflict" {
		t.Fatalf("different payload accepted %v", err)
	}
	// The same textual key is independent for another authenticated owner.
	if _, fresh, err := s.Create(ctx, 3, "retry-same-key", request(12, 1, 3)); err != nil || !fresh {
		t.Fatalf("key not owner-scoped %v", err)
	}
}
func TestExpiryWithoutWorkerAndCancellation(t *testing.T) {
	s, _ := fixture(t, 1)
	ctx := context.Background()
	b, _, err := s.Create(ctx, 1, "expire-first", request(1, 1, 3))
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, s.DB, `UPDATE booking_holds SET created_at=clock_timestamp()-interval '12 minutes',expires_at=clock_timestamp()-interval '1 minute' WHERE id=$1`, b.ID)
	old, err := s.Get(ctx, 1, b.ID)
	if err != nil || old.Status != "expired" {
		t.Fatalf("effective expiry %+v %v", old, err)
	}
	av, err := s.Availability(ctx, 1, 1, 3)
	if err != nil || av.Seats == nil || *av.Seats != 1 {
		t.Fatalf("worker required for stock %+v %v", av, err)
	}
	retry, fresh, err := s.Create(ctx, 1, "expire-first", request(1, 1, 3))
	if err != nil || fresh || retry.Status != "expired" || !retry.ExpiresAt.Equal(old.ExpiresAt) {
		t.Fatalf("retry extended expired hold %+v %v", retry, err)
	}
	newer, _, err := s.Create(ctx, 2, "reuse-expired", request(1, 1, 3))
	if err != nil || newer.SeatNumber != 1 {
		t.Fatalf("stale exclusion blocked reuse %v", err)
	}
	cancelled, err := s.Cancel(ctx, 2, newer.ID)
	if err != nil || cancelled.Status != "cancelled" || cancelled.CancelledAt == nil {
		t.Fatalf("cancel %+v %v", cancelled, err)
	}
	repeated, err := s.Cancel(ctx, 2, newer.ID)
	if err != nil || !repeated.CancelledAt.Equal(*cancelled.CancelledAt) {
		t.Fatalf("cancel is not idempotent %+v %v", repeated, err)
	}
	expired, err := s.Cancel(ctx, 1, b.ID)
	if err != nil || expired.Status != "expired" || expired.CancelledAt != nil {
		t.Fatalf("expired hold became cancelled %+v %v", expired, err)
	}
	if _, err = s.Cancel(ctx, 3, newer.ID); errorCode(err) != "not_found" {
		t.Fatalf("foreign cancel %v", err)
	}
	// Bounded cleanup expires only its requested batch, without touching live holds.
	for id := 2; id <= 4; id++ {
		c, _, err := s.Create(ctx, id+3, fmt.Sprintf("cleanup-key-%d", id), request(id, 1, 3))
		if err != nil {
			t.Fatal(err)
		}
		mustExec(t, s.DB, `UPDATE booking_holds SET created_at=clock_timestamp()-interval '12 minutes',expires_at=clock_timestamp()-interval '1 minute' WHERE id=$1`, c.ID)
	}
	if err = s.Expire(ctx, 2); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err = s.DB.Get(&remaining, `SELECT count(*) FROM booking_holds WHERE status='held' AND expires_at<clock_timestamp()`); err != nil || remaining != 1 {
		t.Fatalf("unbounded expiry %d %v", remaining, err)
	}
	if err = s.Expire(ctx, 2); err != nil {
		t.Fatal(err)
	}
	var inconsistent int
	if err = s.DB.Get(&inconsistent, `SELECT count(*) FROM booking_holds WHERE (status='cancelled' AND cancelled_at IS NULL) OR (status='expired' AND cancelled_at IS NOT NULL)`); err != nil || inconsistent != 0 {
		t.Fatalf("cancellation timestamp/state mismatch %d %v", inconsistent, err)
	}
}
func TestPassengerOwnershipAndImmutableBookingSnapshot(t *testing.T) {
	s, _ := fixture(t, 3)
	ctx := context.Background()
	p, err := s.SavePassenger(ctx, 1, 0, "  Анна Тестовая  ")
	if err != nil || p.FullName != "Анна Тестовая" {
		t.Fatalf("passenger %+v %v", p, err)
	}
	r := request(1, 1, 3)
	r.PassengerName = nil
	r.PassengerID = &p.ID
	if _, _, err = s.Create(ctx, 2, "foreign-passenger", r); errorCode(err) != "not_found" {
		t.Fatalf("foreign passenger %v", err)
	}
	b, _, err := s.Create(ctx, 1, "passenger-booking", r)
	if err != nil || b.AmountMinor != 30050 || b.PassengerName != p.FullName || b.Currency != "RUB" {
		t.Fatalf("snapshot %+v %v", b, err)
	}
	if _, err = s.SavePassenger(ctx, 2, p.ID, "Changed"); errorCode(err) != "not_found" {
		t.Fatalf("foreign edit %v", err)
	}
	if err = s.DeletePassenger(ctx, 2, p.ID); errorCode(err) != "not_found" {
		t.Fatalf("foreign delete %v", err)
	}
	if _, err = s.SavePassenger(ctx, 1, p.ID, "Новое имя"); err != nil {
		t.Fatal(err)
	}
	if err = s.DeletePassenger(ctx, 1, p.ID); err != nil {
		t.Fatal(err)
	}
	mustExec(t, s.DB, `UPDATE stops SET name='Переименовано' WHERE id=1; UPDATE bus SET capacity=10 WHERE id=1`)
	if _, err = s.DB.Exec(`UPDATE bus SET capacity=1 WHERE id=1`); err == nil {
		t.Fatal("bus shrink with active holds accepted")
	}
	if _, err = s.DB.Exec(`UPDATE trips SET status='cancelled' WHERE id=1`); err == nil {
		t.Fatal("trip cancellation with active holds accepted")
	}
	if _, err = s.DB.Exec(`UPDATE trips SET bus_id=bus_id,updated_at=now() WHERE id=1`); err != nil {
		t.Fatalf("nonmaterial trip update rejected %v", err)
	}
	got, err := s.Get(ctx, 1, b.ID)
	if err != nil || got.PassengerName != p.FullName || got.BoardingName != "Начало" || got.AmountMinor != 30050 {
		t.Fatalf("snapshot drift %+v %v", got, err)
	}
	retry, fresh, err := s.Create(ctx, 1, "passenger-booking", r)
	if err != nil || fresh || retry.ID != b.ID {
		t.Fatalf("deleted passenger broke idempotency %v", err)
	}
	av, err := s.Availability(ctx, 1, 1, 3)
	if err != nil || av.Seats == nil || *av.Seats != 2 {
		t.Fatalf("capacity snapshot changed %+v %v", av, err)
	}
	if _, err = s.Get(ctx, 2, b.ID); errorCode(err) != "not_found" {
		t.Fatalf("foreign history %v", err)
	}
	// Timetable cannot alter active intervals or prices, even through legacy CRUD.
	_, err = s.DB.Exec(`UPDATE trip_stops SET price_to_next=999 WHERE trip_id=1 AND stop_order=1`)
	if err == nil {
		t.Fatal("active timetable changed")
	}
	if _, err = s.Cancel(ctx, 1, b.ID); err != nil {
		t.Fatal(err)
	}
	mustExec(t, s.DB, `UPDATE bus SET capacity=1 WHERE id=1`)
	av, err = s.Availability(ctx, 1, 1, 3)
	if err != nil || av.Seats == nil || *av.Seats != 1 {
		t.Fatalf("smaller bus ignored %+v %v", av, err)
	}
	if _, _, err = s.Create(ctx, 2, "smaller-bus-first", request(1, 1, 3)); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.Create(ctx, 3, "smaller-bus-second", request(1, 1, 3)); errorCode(err) != "sold_out" {
		t.Fatalf("smaller bus overbooked %v", err)
	}
	smallPage, err := s.List(ctx, 2, 30, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Cancel(ctx, 2, smallPage.Items[0].ID); err != nil {
		t.Fatal(err)
	}
	mustExec(t, s.DB, `UPDATE trip_stops SET price_to_next=999 WHERE trip_id=1 AND stop_order=1`)
	got, err = s.Get(ctx, 1, b.ID)
	if err != nil || got.AmountMinor != 30050 {
		t.Fatalf("fare snapshot drift %+v %v", got, err)
	}
}
func TestUnknownRealInventoryRepeatedStopsAndTimetableLock(t *testing.T) {
	s, _ := fixture(t, 2)
	ctx := context.Background()
	mustExec(t, s.DB, `UPDATE trips SET is_demo=false WHERE id=1`)
	av, err := s.Availability(ctx, 1, 1, 3)
	if err != nil || av.Status != "unknown" || av.Bookable || av.Seats != nil {
		t.Fatalf("invented real inventory %+v %v", av, err)
	}
	if _, _, err = s.Create(ctx, 1, "real-inventory", request(1, 1, 3)); errorCode(err) != "inventory_unavailable" {
		t.Fatalf("real hold accepted %v", err)
	}
	mustExec(t, s.DB, `INSERT INTO trip_stops(trip_id,stop_id,arrival_time,departure_time,stop_order,price_to_next) SELECT 2,1,end_time,end_time,4,10 FROM trips WHERE id=2; INSERT INTO trip_stops(trip_id,stop_id,arrival_time,departure_time,stop_order) SELECT 2,3,end_time+interval '1 hour',end_time+interval '1 hour',5 FROM trips WHERE id=2`)
	if _, _, err = s.Create(ctx, 1, "ambiguous-trip", request(2, 1, 3)); errorCode(err) != "ambiguous_segment" {
		t.Fatalf("loop guessed %v", err)
	}
	// A concurrent timetable writer holds the same trip row. Hold creation must
	// observe its committed new fare, rather than a stale pre-lock quote.
	tx, err := s.DB.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`UPDATE trip_stops SET price_to_next=12.34 WHERE trip_id=3 AND stop_order=1`); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct {
		b   Booking
		err error
	}, 1)
	go func() {
		b, _, err := s.Create(ctx, 1, "concurrent-fare", request(3, 1, 3))
		done <- struct {
			b   Booking
			err error
		}{b, err}
	}()
	waitForBlockedBooking(t, s.DB)
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	result := <-done
	if result.err != nil || result.b.AmountMinor != 21272 {
		t.Fatalf("stale fare after concurrent write %+v %v", result.b, result.err)
	}
	// Status changes are serialized by the trip row as well.
	tx, err = s.DB.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`UPDATE trips SET status='cancelled' WHERE id=4`); err != nil {
		t.Fatal(err)
	}
	go func() {
		b, _, err := s.Create(ctx, 2, "concurrent-status", request(4, 1, 3))
		done <- struct {
			b   Booking
			err error
		}{b, err}
	}()
	waitForBlockedBooking(t, s.DB)
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	result = <-done
	if errorCode(result.err) != "trip_unavailable" {
		t.Fatalf("stale status accepted %v", result.err)
	}
}
func waitForBlockedBooking(t *testing.T, db *sqlx.DB) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var n int
		err := db.Get(&n, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%LEAST(i.capacity,b.capacity)%'`)
		if err != nil {
			t.Fatal(err)
		}
		if n > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("booking did not wait for timetable lock")
}
func TestListsOneQueryAndOwnerScopedKeyset(t *testing.T) {
	s, counter := fixture(t, 5)
	ctx := context.Background()
	b, _, err := s.Create(ctx, 1, "history-fixture", request(1, 1, 3))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Cancel(ctx, 1, b.ID); err != nil {
		t.Fatal(err)
	}
	mustExec(t, s.DB, `INSERT INTO booking_holds(user_id,trip_id,passenger_name,boarding_stop_id,alighting_stop_id,boarding_order,alighting_order,boarding_name,alighting_name,departure_time,arrival_time,timezone,arrival_timezone,seat_number,amount_minor,is_demo,status,created_at,expires_at,idempotency_key,payload_hash) SELECT h.user_id,h.trip_id,h.passenger_name,h.boarding_stop_id,h.alighting_stop_id,h.boarding_order,h.alighting_order,h.boarding_name,h.alighting_name,h.departure_time,h.arrival_time,h.timezone,h.arrival_timezone,h.seat_number,h.amount_minor,h.is_demo,h.status,h.created_at,h.expires_at,'fixture-page-'||n,h.payload_hash FROM booking_holds h CROSS JOIN generate_series(1,234)n WHERE h.id=$1`, b.ID)
	mustExec(t, s.DB, `INSERT INTO booking_passengers(user_id,full_name) SELECT 1,'Пассажир '||n FROM generate_series(1,235)n`)
	for _, limit := range []int{1, 30, 100} {
		counter.n.Store(0)
		p, err := s.List(ctx, 1, limit, "")
		if err != nil || len(p.Items) != limit || p.NextCursor == nil {
			t.Fatalf("list %d %+v %v", limit, p, err)
		}
		if counter.n.Load() != 1 {
			t.Fatalf("history N+1 %d", counter.n.Load())
		}
		if _, err = s.List(ctx, 2, limit, *p.NextCursor); errorCode(err) != "invalid_request" {
			t.Fatalf("cross owner cursor %v", err)
		}
		counter.n.Store(0)
		passengers, err := s.Passengers(ctx, 1, limit, "")
		if err != nil || len(passengers.Items) != limit || counter.n.Load() != 1 {
			t.Fatalf("passenger N+1 %d %+v %v", counter.n.Load(), passengers, err)
		}
		seen := map[int64]bool{}
		cursor := ""
		for {
			page, err := s.List(ctx, 1, limit, cursor)
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range page.Items {
				if seen[row.ID] {
					t.Fatalf("duplicate %d", row.ID)
				}
				seen[row.ID] = true
			}
			if page.NextCursor == nil {
				break
			}
			cursor = *page.NextCursor
		}
		if len(seen) != 235 {
			t.Fatalf("keyset lost %d", len(seen))
		}
	}
}
func TestMigrationDownUpAndCancellation(t *testing.T) {
	s, _ := fixture(t, 1)
	b, err := os.ReadFile("../../db/migrations/20261105000100_booking_holds.sql")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(b), "-- +goose Down")
	checkoutMigration, err := os.ReadFile("../../db/migrations/20261105000300_checkout.sql")
	if err != nil {
		t.Fatal(err)
	}
	checkoutParts := strings.Split(string(checkoutMigration), "-- +goose Down")
	mustExec(t, s.DB, checkoutParts[1])
	mustExec(t, s.DB, parts[1])
	mustExec(t, s.DB, parts[0])
	mustExec(t, s.DB, checkoutParts[0])
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err = s.Create(ctx, 1, "cancelled-context", request(1, 1, 3)); err == nil {
		t.Fatal("cancelled request ran")
	}
	var n int
	if err = s.DB.Get(&n, `SELECT count(*) FROM booking_holds`); err != nil || n != 0 {
		t.Fatalf("cancelled request persisted %d %v", n, err)
	}
}
func TestInputValidation(t *testing.T) {
	for _, name := range []string{"", "   ", "A\nB", "A\x00B", strings.Repeat("я", 161), string([]byte{0xff})} {
		if _, err := NormalizeName(name); err == nil {
			t.Errorf("accepted invalid name %q", name)
		}
	}
	for _, name := range []string{"Я", strings.Repeat("я", 160), "  Иван Иванов  "} {
		if _, err := NormalizeName(name); err != nil {
			t.Error(err)
		}
	}
	r := request(1, 1, 3)
	for _, key := range []string{"short", "space key", strings.Repeat("a", 129), "русский-ключ"} {
		if r.validate(key) == nil {
			t.Error("invalid key accepted")
		}
	}
	id := int64(1)
	r.PassengerID = &id
	if r.validate("valid-key") == nil {
		t.Error("both passenger forms accepted")
	}
}

func TestCapacityWriterSerializesWithFirstAllocation(t *testing.T) {
	s, _ := fixture(t, 45)
	ctx := context.Background()
	tx, err := s.DB.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`UPDATE bus SET capacity=2 WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, _, err := s.Create(ctx, 1, "capacity-writer", request(1, 1, 3)); done <- err }()
	waitForBlockedBooking(t, s.DB)
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	var snapshot int
	if err = s.DB.Get(&snapshot, `SELECT capacity FROM booking_inventory WHERE trip_id=1`); err != nil || snapshot != 2 {
		t.Fatalf("stale bus capacity snapshot %d %v", snapshot, err)
	}
	for _, query := range []string{`UPDATE bus SET capacity=1 WHERE id=1`, `UPDATE bus SET deleted_at=now() WHERE id=1`, `UPDATE trips SET status='cancelled' WHERE id=1`, `UPDATE trips SET is_demo=false WHERE id=1`} {
		_, err = s.DB.Exec(query)
		var pgerr *pgconn.PgError
		if !errors.As(err, &pgerr) || pgerr.Code != "23514" || !strings.HasPrefix(pgerr.Message, "booking_") {
			t.Fatalf("unguarded material write %s: %v", query, err)
		}
	}
	mustExec(t, s.DB, `INSERT INTO bus(id,license_plate,brand,capacity,category_id,status_id) VALUES(2,'BOOKING-OTHER','Other',2,1,1)`)
	if _, err = s.DB.Exec(`UPDATE trips SET bus_id=2 WHERE id=1`); err == nil {
		t.Fatal("bus reassigned with active hold")
	}
	page, err := s.List(ctx, 1, 30, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Cancel(ctx, 1, page.Items[0].ID); err != nil {
		t.Fatal(err)
	}
	mustExec(t, s.DB, `UPDATE bus SET capacity=45 WHERE id=1`)
	av, err := s.Availability(ctx, 1, 1, 3)
	if err != nil || av.Seats == nil || *av.Seats != 2 {
		t.Fatalf("increased bus silently grew snapshot %+v %v", av, err)
	}
	mustExec(t, s.DB, `UPDATE bus SET capacity=1 WHERE id=1`)
	av, err = s.Availability(ctx, 1, 1, 3)
	if err != nil || av.Seats == nil || *av.Seats != 1 {
		t.Fatalf("decreased bus ignored %+v %v", av, err)
	}
}
