package checkout

import (
	"context"
	"corpord-api/internal/booking"
	"corpord-api/internal/payments"
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
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

type queryCounter struct{ n atomic.Int64 }

func (q *queryCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	q.n.Add(1)
	return ctx
}
func (*queryCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

type fakeProvider struct {
	mu                                                                       sync.Mutex
	payments                                                                 map[string]payments.Payment
	createKeys                                                               map[string]string
	captureKeys                                                              map[string]bool
	refundKeys                                                               map[string]payments.Refund
	creates, gets, captures, captureEffects, cancels, refunds, refundEffects int
	loseCreate, loseCapture, loseRefund, unknownCapture                      bool
	refundStatus                                                             string
	captureStarted                                                           chan struct{}
	captureGate                                                              chan struct{}
	gateOnce                                                                 sync.Once
	beforeNetwork                                                            func()
}

func newFake() *fakeProvider {
	return &fakeProvider{payments: map[string]payments.Payment{}, createKeys: map[string]string{}, captureKeys: map[string]bool{}, refundKeys: map[string]payments.Refund{}}
}
func (f *fakeProvider) CreateAuthorization(_ context.Context, key string, r payments.AuthorizationRequest) (payments.Payment, error) {
	if f.beforeNetwork != nil {
		f.beforeNetwork()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.creates++
	if id, ok := f.createKeys[key]; ok {
		return f.payments[id], nil
	}
	p := payments.Payment{ID: "payment-" + r.IntentID, IntentID: r.IntentID, Status: "pending", AmountMinor: r.AmountMinor, Currency: r.Currency, Test: true, MerchantID: "test-shop", ConfirmationURL: "https://yoomoney.ru/checkout/test-" + r.IntentID}
	f.payments[p.ID] = p
	f.createKeys[key] = p.ID
	if f.loseCreate {
		f.loseCreate = false
		return payments.Payment{}, &payments.Error{Kind: payments.Unknown}
	}
	return p, nil
}
func (f *fakeProvider) GetPayment(_ context.Context, id string) (payments.Payment, error) {
	if f.beforeNetwork != nil {
		f.beforeNetwork()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gets++
	p, ok := f.payments[id]
	if !ok {
		return p, &payments.Error{Kind: payments.Rejected}
	}
	return p, nil
}
func (f *fakeProvider) Capture(ctx context.Context, key, id string, amount int64) (payments.Payment, error) {
	if f.beforeNetwork != nil {
		f.beforeNetwork()
	}
	f.mu.Lock()
	f.captures++
	p, ok := f.payments[id]
	if !ok || amount != p.AmountMinor {
		f.mu.Unlock()
		return payments.Payment{}, &payments.Error{Kind: payments.Security}
	}
	if f.unknownCapture {
		f.mu.Unlock()
		return payments.Payment{}, &payments.Error{Kind: payments.Unknown}
	}
	if !f.captureKeys[key] {
		f.captureKeys[key] = true
		f.captureEffects++
		p.Status = "succeeded"
		f.payments[id] = p
	}
	lost := f.loseCapture
	f.loseCapture = false
	gate := f.captureGate
	started := f.captureStarted
	f.mu.Unlock()
	if gate != nil {
		first := false
		f.gateOnce.Do(func() { first = true })
		if first {
			close(started)
			select {
			case <-gate:
			case <-ctx.Done():
				return payments.Payment{}, ctx.Err()
			}
		}
	}
	if lost {
		return payments.Payment{}, &payments.Error{Kind: payments.Unknown}
	}
	return p, nil
}
func (f *fakeProvider) CancelAuthorization(_ context.Context, key, id string) (payments.Payment, error) {
	if f.beforeNetwork != nil {
		f.beforeNetwork()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancels++
	p, ok := f.payments[id]
	if !ok || key == "" {
		return p, &payments.Error{Kind: payments.Security}
	}
	if p.Status == "waiting_for_capture" {
		p.Status = "canceled"
		f.payments[id] = p
	}
	return p, nil
}
func (f *fakeProvider) CreateRefund(_ context.Context, key string, r payments.RefundRequest) (payments.Refund, error) {
	if f.beforeNetwork != nil {
		f.beforeNetwork()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refunds++
	if v, ok := f.refundKeys[key]; ok {
		return v, nil
	}
	p, ok := f.payments[r.PaymentID]
	if !ok || p.Status != "succeeded" || p.AmountMinor != r.AmountMinor || p.IntentID != r.IntentID {
		return payments.Refund{}, &payments.Error{Kind: payments.Security}
	}
	v := payments.Refund{ID: "refund-" + r.IntentID, PaymentID: r.PaymentID, Status: "succeeded", AmountMinor: r.AmountMinor, Currency: r.Currency}
	if f.refundStatus != "" {
		v.Status = f.refundStatus
	}
	f.refundKeys[key] = v
	f.refundEffects++
	if f.loseRefund {
		f.loseRefund = false
		return payments.Refund{}, &payments.Error{Kind: payments.Unknown}
	}
	return v, nil
}
func (f *fakeProvider) GetRefund(_ context.Context, id string) (payments.Refund, error) {
	if f.beforeNetwork != nil {
		f.beforeNetwork()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, v := range f.refundKeys {
		if v.ID == id {
			return v, nil
		}
	}
	return payments.Refund{}, &payments.Error{Kind: payments.Rejected}
}
func (f *fakeProvider) set(id, status string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.payments[id]
	p.Status = status
	f.payments[id] = p
}
func (f *fakeProvider) alter(id string, fn func(*payments.Payment)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.payments[id]
	fn(&p)
	f.payments[id] = p
}

func fixture(t *testing.T, capacity int) (*Service, *booking.Service, *fakeProvider, *queryCounter) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL required; isolated disposable PostgreSQL schemas only")
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
	schema := "checkout_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	cfg.RuntimeParams["search_path"] = schema + ",public"
	cfg.RuntimeParams["application_name"] = schema
	cfg.RuntimeParams["timezone"] = "UTC"
	cfg.RuntimeParams["jit"] = "off"
	counter := &queryCounter{}
	cfg.Tracer = counter
	db := sqlx.NewDb(stdlib.OpenDB(*cfg), "pgx")
	db.SetMaxOpenConns(12)
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
		v, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		up := strings.Split(string(v), "-- +goose Down")[0]
		up = strings.ReplaceAll(up, `CREATE EXTENSION "uuid-ossp";`, "")
		mustExec(t, db, up)
	}
	mustExec(t, db, `INSERT INTO users(id,email,password_hash,name) SELECT n,'checkout-'||n||'@example.invalid','test-only','Test' FROM generate_series(1,24)n;
 INSERT INTO bus(id,license_plate,brand,capacity,category_id,status_id) VALUES(1,'CHECKOUT-TEST','Test',`+fmt.Sprint(capacity)+`,1,1);
 INSERT INTO drivers(id,first_name,last_name,middle_name,phone_number,status) VALUES(1,'Test','Driver','','0',1);
 INSERT INTO stops(id,name,timezone,city_id) VALUES(1,'Начало','Asia/Yekaterinburg',2),(2,'Середина','Asia/Yekaterinburg',2),(3,'Конец','Europe/Moscow',1);
 INSERT INTO trips(id,bus_id,driver_id,start_time,end_time,status,base_price,is_demo) SELECT n,1,1,(clock_timestamp()+interval '1 day') AT TIME ZONE 'UTC',(clock_timestamp()+interval '1 day 3 hours') AT TIME ZONE 'UTC','scheduled',300.50,true FROM generate_series(1,12)n;
 INSERT INTO trip_stops(trip_id,stop_id,arrival_time,departure_time,stop_order,price_to_next) SELECT t.id,n,t.start_time+(n-1)*interval '1 hour',t.start_time+(n-1)*interval '1 hour',n,CASE WHEN n=1 THEN 100.12 WHEN n=2 THEN 200.38 ELSE NULL END FROM trips t CROSS JOIN generate_series(1,3)n;`)
	f := newFake()
	b := booking.New(db)
	b.CheckoutEnabled = true
	s := New(db, f, Options{Enabled: true, ReturnOrigin: "http://localhost:3000"})
	f.beforeNetwork = func() {
		var n int
		err := db.Get(&n, `SELECT count(*) FROM pg_stat_activity WHERE application_name=$1 AND state='idle in transaction'`, schema)
		if err != nil {
			t.Errorf("network lock check %v", err)
		}
		if n != 0 {
			t.Errorf("provider called with %d open idle transactions", n)
		}
	}
	return s, b, f, counter
}
func mustExec(t *testing.T, db *sqlx.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatal(err)
	}
}
func code(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	var b *booking.Error
	if errors.As(err, &b) {
		return b.Code
	}
	return ""
}
func hold(t *testing.T, b *booking.Service, owner, trip, from, to int) booking.Booking {
	t.Helper()
	name := "Тестовый Пассажир"
	v, _, err := b.Create(context.Background(), owner, uuid.NewString(), booking.CreateRequest{TripID: trip, BoardingStopID: from, AlightingStopID: to, PassengerName: &name})
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func intent(t *testing.T, s *Service, b *booking.Service, owner, trip, from, to int) Detail {
	t.Helper()
	h := hold(t, b, owner, trip, from, to)
	d, fresh, err := s.Create(context.Background(), owner, uuid.NewString(), h.ID)
	if err != nil || !fresh {
		t.Fatalf("checkout %+v %v", d, err)
	}
	return d
}
func ready(t *testing.T, s *Service) {
	t.Helper()
	mustExec(t, s.DB, `UPDATE checkout_jobs SET available_at=clock_timestamp()-interval '1 second' WHERE state='pending'; UPDATE checkout_inbox SET available_at=clock_timestamp()-interval '1 second' WHERE state='pending'`)
}
func run(t *testing.T, s *Service, limit int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.Worker(ctx, limit); err != nil {
		t.Fatal(err)
	}
}
func detail(t *testing.T, s *Service, owner int, id int64) Detail {
	t.Helper()
	d, err := s.Get(context.Background(), owner, id)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
func expire(t *testing.T, s *Service, d Detail) {
	t.Helper()
	mustExec(t, s.DB, `UPDATE booking_holds SET created_at=clock_timestamp()-interval '12 minutes',expires_at=clock_timestamp()-interval '1 minute' WHERE id=$1`, d.HoldID)
	mustExec(t, s.DB, `UPDATE checkout_orders SET hold_expires_at=clock_timestamp()-interval '1 minute' WHERE id=$1`, d.ID)
}

func TestCheckoutIntentIdempotencyOwnershipAndSnapshot(t *testing.T) {
	s, b, _, _ := fixture(t, 4)
	ctx := context.Background()
	h := hold(t, b, 1, 1, 1, 3)
	key := "checkout-same-key"
	start := make(chan struct{})
	var wg sync.WaitGroup
	var created atomic.Int32
	ids := make(chan int64, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			d, fresh, err := s.Create(ctx, 1, key, h.ID)
			if err != nil {
				t.Errorf("intent race %v", err)
				return
			}
			if fresh {
				created.Add(1)
			}
			ids <- d.ID
		}()
	}
	close(start)
	wg.Wait()
	close(ids)
	var id int64
	for v := range ids {
		if id == 0 {
			id = v
		}
		if v != id {
			t.Fatal("duplicate checkout intent")
		}
	}
	if created.Load() != 1 {
		t.Fatalf("created %d", created.Load())
	}
	if _, _, err := s.Create(ctx, 1, "different-key", h.ID); code(err) != "checkout_exists" {
		t.Fatalf("second intent accepted %v", err)
	}
	h2 := hold(t, b, 1, 2, 1, 3)
	if _, _, err := s.Create(ctx, 1, key, h2.ID); code(err) != "idempotency_conflict" {
		t.Fatalf("key payload conflict %v", err)
	}
	if _, _, err := s.Create(ctx, 2, "foreign-hold", h2.ID); code(err) != "not_found" {
		t.Fatalf("foreign hold %v", err)
	}
	if _, err := s.Get(ctx, 2, id); code(err) != "not_found" {
		t.Fatalf("foreign detail %v", err)
	}
	if _, err := s.Cancel(ctx, 2, id); code(err) != "not_found" {
		t.Fatalf("foreign cancel %v", err)
	}
	if err := s.Reconcile(ctx, 2, id); code(err) != "not_found" {
		t.Fatalf("foreign reconcile %v", err)
	}
	got, err := b.Get(ctx, 1, h.ID)
	if err != nil || got.CheckoutID == nil || *got.CheckoutID != id || !got.CheckoutEnabled {
		t.Fatalf("booking capability/link %+v %v", got, err)
	}
	if _, err = b.Cancel(ctx, 1, h.ID); code(err) != "checkout_in_progress" {
		t.Fatalf("legacy cancel freed checkout %v", err)
	}
	d := detail(t, s, 1, id)
	if d.AmountMinor != 30050 || d.PassengerName != h.PassengerName || !d.Actions.CanPay || d.Payment.ConfirmationURL != nil {
		t.Fatalf("bad snapshot/actions %+v", d)
	}
	if err = s.Reconcile(ctx, 1, id); err != nil {
		t.Fatal(err)
	}
	if err = s.Reconcile(ctx, 1, id); code(err) != "rate_limited" {
		t.Fatalf("reconcile not throttled %v", err)
	}
}
func TestPendingPollDoesNotSpinAndGetNeverCallsProvider(t *testing.T) {
	s, b, f, counter := fixture(t, 3)
	d := intent(t, s, b, 1, 1, 1, 3)
	run(t, s, 25)
	if f.creates != 1 || f.gets != 0 {
		t.Fatalf("pending spun create=%d get=%d", f.creates, f.gets)
	}
	var future bool
	if err := s.DB.Get(&future, `SELECT available_at>clock_timestamp() FROM checkout_jobs WHERE order_id=$1 AND kind='reconcile'`, d.ID); err != nil || !future {
		t.Fatalf("poll not delayed %v %v", future, err)
	}
	ready(t, s)
	run(t, s, 25)
	if f.gets != 1 {
		t.Fatalf("pending GET spun %d", f.gets)
	}
	counter.n.Store(0)
	got, err := s.Get(context.Background(), 1, d.ID)
	if err != nil || counter.n.Load() != 1 || got.Payment.ConfirmationURL == nil || !got.Actions.CanViewConfirmation {
		t.Fatalf("detail %+v sql=%d %v", got, counter.n.Load(), err)
	}
	if f.gets != 1 {
		t.Fatal("GET detail called provider")
	}
	counter.n.Store(0)
	p, err := s.List(context.Background(), 1, 30, "", "")
	if err != nil || len(p.Items) != 1 || counter.n.Load() != 1 || f.gets != 1 {
		t.Fatalf("list sql/provider N+1 %+v %d %v", p, counter.n.Load(), err)
	}
}
func TestConfirmedSeatSurvivesTTLAndAdjacentReuse(t *testing.T) {
	s, b, f, _ := fixture(t, 1)
	d := intent(t, s, b, 1, 1, 1, 2)
	run(t, s, 1)
	f.set(fmt.Sprintf("payment-%d", d.ID), "waiting_for_capture")
	ready(t, s)
	run(t, s, 10)
	got := detail(t, s, 1, d.ID)
	if got.Status != "paid" || got.PaymentStatus != "succeeded" || got.Actions.CanCancel {
		t.Fatalf("not paid %+v", got)
	}
	expire(t, s, d)
	if err := b.Expire(context.Background(), 100); err != nil {
		t.Fatal(err)
	}
	h, err := b.Get(context.Background(), 1, d.HoldID)
	if err != nil || h.Status != "confirmed" {
		t.Fatalf("confirmed expired %+v %v", h, err)
	}
	av, err := b.Availability(context.Background(), 1, 1, 3)
	if err != nil || av.Seats == nil || *av.Seats != 0 {
		t.Fatalf("confirmed not counted %+v %v", av, err)
	}
	adjacent := hold(t, b, 2, 1, 2, 3)
	if adjacent.SeatNumber != 1 {
		t.Fatal("adjacent seat not reused")
	}
	name := "Other"
	if _, _, err = b.Create(context.Background(), 3, "overlap-paid", booking.CreateRequest{TripID: 1, BoardingStopID: 1, AlightingStopID: 3, PassengerName: &name}); code(err) != "sold_out" {
		t.Fatalf("confirmed resold %v", err)
	}
	if _, err = b.Cancel(context.Background(), 1, d.HoldID); code(err) != "paid_order" {
		t.Fatalf("confirmed legacy cancelled %v", err)
	}
	if _, err = s.Cancel(context.Background(), 1, d.ID); code(err) != "paid_order" {
		t.Fatalf("paid checkout cancelled %v", err)
	}
	for _, q := range []string{`UPDATE trips SET status='cancelled' WHERE id=1`, `UPDATE trip_stops SET price_to_next=5 WHERE trip_id=1 AND stop_order=1`, `UPDATE bus SET capacity=0 WHERE id=1`, `UPDATE bus SET deleted_at=now() WHERE id=1`} {
		if _, err = s.DB.Exec(q); err == nil {
			t.Fatalf("confirmed guard bypassed %s", q)
		}
	}
	// Delayed older provider facts cannot regress a paid order.
	f.set(fmt.Sprintf("payment-%d", d.ID), "pending")
	if err = s.IngestWebhook(context.Background(), "payment.waiting_for_capture", fmt.Sprintf("payment-%d", d.ID)); err != nil {
		t.Fatal(err)
	}
	run(t, s, 10)
	if detail(t, s, 1, d.ID).Status != "paid" {
		t.Fatal("stale notification regressed paid")
	}
}
func TestCancelAndExpiryWinLateAuthorization(t *testing.T) {
	for _, mode := range []string{"cancel", "expiry"} {
		t.Run(mode, func(t *testing.T) {
			s, b, f, _ := fixture(t, 1)
			d := intent(t, s, b, 1, 1, 1, 3)
			run(t, s, 1)
			if mode == "cancel" {
				if _, err := s.Cancel(context.Background(), 1, d.ID); err != nil {
					t.Fatal(err)
				}
			} else {
				expire(t, s, d)
				if err := b.Expire(context.Background(), 100); err != nil {
					t.Fatal(err)
				}
			}
			replacement := hold(t, b, 2, 1, 1, 3)
			f.set(fmt.Sprintf("payment-%d", d.ID), "waiting_for_capture")
			ready(t, s)
			run(t, s, 10)
			if f.captures != 0 || f.cancels != 1 {
				t.Fatalf("late authorization captured=%d canceled=%d", f.captures, f.cancels)
			}
			got := detail(t, s, 1, d.ID)
			if got.Status != "cancelled" && got.Status != "expired" {
				t.Fatalf("late auth state %+v", got)
			}
			h, err := b.Get(context.Background(), 2, replacement.ID)
			if err != nil || h.Status != "held" {
				t.Fatalf("replacement harmed %+v %v", h, err)
			}
		})
	}
}
func TestUnknownCaptureNeverReleasesAndLostCaptureReconciles(t *testing.T) {
	for _, mode := range []string{"unknown", "lost_success"} {
		t.Run(mode, func(t *testing.T) {
			s, b, f, _ := fixture(t, 1)
			d := intent(t, s, b, 1, 1, 1, 3)
			run(t, s, 1)
			f.set(fmt.Sprintf("payment-%d", d.ID), "waiting_for_capture")
			ready(t, s)
			run(t, s, 1)
			f.unknownCapture = mode == "unknown"
			f.loseCapture = mode == "lost_success"
			run(t, s, 1)
			got := detail(t, s, 1, d.ID)
			if got.Status != "committing" || got.PaymentStatus != "unknown" {
				t.Fatalf("unknown outcome falsely terminal %+v", got)
			}
			expire(t, s, d)
			if err := b.Expire(context.Background(), 100); err != nil {
				t.Fatal(err)
			}
			av, err := b.Availability(context.Background(), 1, 1, 3)
			if err != nil || av.Seats == nil || *av.Seats != 0 {
				t.Fatalf("unknown capture seat released %+v %v", av, err)
			}
			if _, err = b.Cancel(context.Background(), 1, d.HoldID); code(err) != "checkout_in_progress" {
				t.Fatalf("unknown capture cancel %v", err)
			}
			ready(t, s)
			run(t, s, 1)
			if mode == "lost_success" && detail(t, s, 1, d.ID).Status != "paid" {
				t.Fatal("lost capture success not recovered")
			}
			if mode == "unknown" {
				mustExec(t, s.DB, `UPDATE checkout_jobs SET failures=19,available_at=clock_timestamp() WHERE kind='capture' AND order_id=$1`, d.ID)
				run(t, s, 1)
				if detail(t, s, 1, d.ID).Status != "review_required" {
					t.Fatal("unknown retry budget not visible")
				}
				h, err := b.Get(context.Background(), 1, d.HoldID)
				if err != nil || h.Status != "payment_committing" {
					t.Fatalf("review released allocation %+v %v", h, err)
				}
			}
		})
	}
}
func TestTwoWorkersLeaseTakeoverAndStaleResultFenced(t *testing.T) {
	s, b, f, _ := fixture(t, 1)
	d := intent(t, s, b, 1, 1, 1, 3)
	run(t, s, 1)
	f.set(fmt.Sprintf("payment-%d", d.ID), "waiting_for_capture")
	ready(t, s)
	run(t, s, 1)
	f.captureStarted = make(chan struct{})
	f.captureGate = make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.Worker(ctx, 1) }()
	select {
	case <-f.captureStarted:
	case <-ctx.Done():
		t.Fatal("capture did not start")
	}
	mustExec(t, s.DB, `UPDATE checkout_jobs SET lease_until=clock_timestamp()-interval '1 second' WHERE order_id=$1 AND kind='capture' AND state='leased'`, d.ID)
	run(t, s, 1)
	if detail(t, s, 1, d.ID).Status != "paid" {
		t.Fatal("takeover failed to recover provider success")
	}
	close(f.captureGate)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if f.captureEffects != 1 || f.captures != 2 {
		t.Fatalf("duplicate capture effects=%d calls=%d", f.captureEffects, f.captures)
	}
	if detail(t, s, 1, d.ID).Status != "paid" {
		t.Fatal("stale lease regressed final state")
	}
}
func TestLostCreateAndCompensationRefundAreIdempotent(t *testing.T) {
	s, b, f, _ := fixture(t, 1)
	f.loseCreate = true
	d := intent(t, s, b, 1, 1, 1, 3)
	run(t, s, 1)
	if detail(t, s, 1, d.ID).PaymentStatus != "unknown" {
		t.Fatal("lost create reported rejection")
	}
	before := f.gets
	if err := s.IngestWebhook(context.Background(), "payment.succeeded", fmt.Sprintf("payment-%d", d.ID)); err != nil {
		t.Fatal(err)
	}
	var inboxRows int
	if err := s.DB.Get(&inboxRows, `SELECT count(*) FROM checkout_inbox`); err != nil || inboxRows != 0 {
		t.Fatalf("unknown notification persisted %d %v", inboxRows, err)
	}
	run(t, s, 1)
	if f.gets != before {
		t.Fatal("unknown webhook ID used as authenticated provider proxy")
	}
	ready(t, s)
	run(t, s, 1)
	if f.creates != 2 || len(f.createKeys) != 1 {
		t.Fatalf("lost create duplicated effects %d/%d", f.creates, len(f.createKeys))
	}
	expire(t, s, d)
	if err := b.Expire(context.Background(), 100); err != nil {
		t.Fatal(err)
	}
	replacement := hold(t, b, 2, 1, 1, 3)
	f.set(fmt.Sprintf("payment-%d", d.ID), "succeeded")
	f.loseRefund = true
	ready(t, s)
	run(t, s, 2)
	got := detail(t, s, 1, d.ID)
	if got.Status != "refund_pending" {
		t.Fatalf("unexpected success fulfilled %+v", got)
	}
	ready(t, s)
	run(t, s, 5)
	if detail(t, s, 1, d.ID).Status != "refunded" {
		t.Fatal("lost refund response not recovered")
	}
	if f.refundEffects != 1 || f.refunds != 2 {
		t.Fatalf("duplicate refunds effects=%d calls=%d", f.refundEffects, f.refunds)
	}
	h, err := b.Get(context.Background(), 2, replacement.ID)
	if err != nil || h.Status != "held" {
		t.Fatalf("compensation harmed new allocation %+v %v", h, err)
	}
	if err = s.IngestWebhook(context.Background(), "payment.succeeded", fmt.Sprintf("payment-%d", d.ID)); err != nil {
		t.Fatal(err)
	}
	run(t, s, 5)
	if f.refundEffects != 1 {
		t.Fatal("duplicate webhook refunded twice")
	}
}
func TestWebhookKnownIdentityCannotRouteToForeignIntent(t *testing.T) {
	s, b, f, _ := fixture(t, 4)
	a := intent(t, s, b, 1, 1, 1, 3)
	z := intent(t, s, b, 2, 2, 1, 3)
	run(t, s, 2)
	f.alter(fmt.Sprintf("payment-%d", a.ID), func(p *payments.Payment) { p.IntentID = fmt.Sprint(z.ID); p.Status = "waiting_for_capture" })
	if err := s.IngestWebhook(context.Background(), "payment.waiting_for_capture", fmt.Sprintf("payment-%d", a.ID)); err != nil {
		t.Fatal(err)
	}
	run(t, s, 1)
	if detail(t, s, 1, a.ID).Status != "review_required" {
		t.Fatal("mismatch did not quarantine owning checkout")
	}
	if detail(t, s, 2, z.ID).Status != "awaiting_payment" {
		t.Fatal("metadata routed facts into another checkout")
	}
	if f.captures != 0 {
		t.Fatal("mismatched intent captured")
	}
}
func TestPaymentFactsAreComparedToImmutableSnapshot(t *testing.T) {
	for _, mode := range []string{"amount", "currency", "test", "merchant", "payment_id"} {
		t.Run(mode, func(t *testing.T) {
			s, b, f, _ := fixture(t, 2)
			d := intent(t, s, b, 1, 1, 1, 3)
			run(t, s, 1)
			id := fmt.Sprintf("payment-%d", d.ID)
			f.alter(id, func(p *payments.Payment) {
				p.Status = "waiting_for_capture"
				switch mode {
				case "amount":
					p.AmountMinor++
				case "currency":
					p.Currency = "USD"
				case "test":
					p.Test = false
				case "merchant":
					p.MerchantID = "other-shop"
				case "payment_id":
					p.ID = "foreign-payment"
				}
			})
			ready(t, s)
			run(t, s, 1)
			if detail(t, s, 1, d.ID).Status != "review_required" || f.captures != 0 {
				t.Fatalf("unsafe payment facts accepted mode=%s", mode)
			}
		})
	}
}
func TestNoBlindCreateAfterProviderIdempotencyWindow(t *testing.T) {
	s, b, f, _ := fixture(t, 1)
	f.loseCreate = true
	d := intent(t, s, b, 1, 1, 1, 3)
	run(t, s, 1)
	mustExec(t, s.DB, `UPDATE checkout_jobs SET created_at=clock_timestamp()-interval '24 hours',available_at=clock_timestamp() WHERE order_id=$1 AND kind='create'`, d.ID)
	run(t, s, 1)
	if f.creates != 1 || detail(t, s, 1, d.ID).Status != "review_required" {
		t.Fatal("unknown create replayed outside idempotency window")
	}
}
func TestCheckoutMigrationPreservesHoldsAndRefusesHistoryRollback(t *testing.T) {
	s, b, _, _ := fixture(t, 2)
	h := hold(t, b, 1, 1, 1, 3)
	raw, err := os.ReadFile("../../db/migrations/20261105000300_checkout.sql")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(raw), "-- +goose Down")
	mustExec(t, s.DB, parts[1])
	mustExec(t, s.DB, parts[0])
	after, err := b.Get(context.Background(), 1, h.ID)
	if err != nil || after.PassengerName != h.PassengerName || !after.ExpiresAt.Equal(h.ExpiresAt) {
		t.Fatalf("migration damaged existing hold %+v %v", after, err)
	}
	var constraints int
	if err = s.DB.Get(&constraints, `SELECT count(*) FROM pg_constraint WHERE conrelid='booking_holds'::regclass AND contype='x'`); err != nil || constraints != 1 {
		t.Fatalf("seat has %d independent exclusions %v", constraints, err)
	}
	if _, _, err = s.Create(context.Background(), 1, "rollback-intent", h.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec(parts[1]); err == nil {
		t.Fatal("rollback silently deleted payment history")
	}
	if _, err = s.Get(context.Background(), 1, 1); err != nil {
		t.Fatal("refused rollback damaged history")
	}
}
func TestFinishedConfirmedTripAllowsReuseOfBusCapacity(t *testing.T) {
	s, b, _, _ := fixture(t, 45)
	h := hold(t, b, 1, 1, 1, 3)
	// Historical confirmed occupancy remains in its own trip ledger, while a bus
	// can be reused after the completed trip. Unknown committing stays protected.
	mustExec(t, s.DB, `UPDATE booking_holds SET status='confirmed',created_at=clock_timestamp()-interval '2 days',expires_at=clock_timestamp()-interval '1 day',departure_time=clock_timestamp()-interval '1 day',arrival_time=clock_timestamp()-interval '1 hour' WHERE id=$1`, h.ID)
	mustExec(t, s.DB, `UPDATE bus SET capacity=2 WHERE id=1`)
	future := hold(t, b, 2, 2, 1, 3)
	mustExec(t, s.DB, `UPDATE booking_holds SET status='confirmed' WHERE id=$1`, future.ID)
	if _, err := s.DB.Exec(`UPDATE bus SET capacity=1 WHERE id=1`); err == nil {
		t.Fatal("future confirmed trip lost bus protection")
	}
}
func TestNotPayableBoundsAndDisabledMode(t *testing.T) {
	s, b, _, _ := fixture(t, 3)
	ctx := context.Background()
	for i, amount := range []int64{0, 99, 35000001} {
		h := hold(t, b, 1, i+1, 1, 3)
		mustExec(t, s.DB, `UPDATE booking_holds SET amount_minor=$2 WHERE id=$1`, h.ID, amount)
		if _, _, err := s.Create(ctx, 1, fmt.Sprintf("not-payable-%d", i), h.ID); code(err) != "not_payable" {
			t.Fatalf("invalid provider amount %d %v", amount, err)
		}
	}
	disabled := New(s.DB, nil, Options{})
	if _, _, err := disabled.Create(ctx, 1, "disabled-intent", 1); code(err) != "payments_disabled" {
		t.Fatalf("disabled checkout %v", err)
	}
	if err := disabled.Worker(ctx, 10); err != nil {
		t.Fatal(err)
	}
}

func TestBackgroundRecoverySurvivesSoftDeletedOwner(t *testing.T) {
	s, b, f, _ := fixture(t, 1)
	d := intent(t, s, b, 1, 1, 1, 3)
	run(t, s, 1)
	f.set(fmt.Sprintf("payment-%d", d.ID), "waiting_for_capture")
	ready(t, s)
	run(t, s, 1)
	f.loseCapture = true
	run(t, s, 1)
	mustExec(t, s.DB, `UPDATE users SET deleted_at=clock_timestamp() WHERE id=1`)
	if _, err := s.Cancel(context.Background(), 1, d.ID); code(err) != "not_found" {
		t.Fatalf("disabled owner could mutate %v", err)
	}
	ready(t, s)
	run(t, s, 2)
	var state string
	if err := s.DB.Get(&state, `SELECT status FROM checkout_orders WHERE id=$1`, d.ID); err != nil || state != "paid" {
		t.Fatalf("deleted user stranded money %s %v", state, err)
	}
}
func TestDepartureStopsCaptureAndCompensatesLateSuccess(t *testing.T) {
	s, b, f, _ := fixture(t, 2)
	d := intent(t, s, b, 1, 1, 1, 3)
	run(t, s, 1)
	f.set(fmt.Sprintf("payment-%d", d.ID), "waiting_for_capture")
	ready(t, s)
	run(t, s, 1)
	// The original authorization was valid, but dispatch has been delayed until
	// after departure. A new capture must not be sent merely because TTL is held.
	mustExec(t, s.DB, `UPDATE checkout_orders SET departure_time=clock_timestamp()-interval '1 second' WHERE id=$1`, d.ID)
	run(t, s, 5)
	if f.captures != 0 || detail(t, s, 1, d.ID).Status != "cancelled" {
		t.Fatalf("departed trip captured %d %+v", f.captures, detail(t, s, 1, d.ID))
	}
	// A separately lost in-flight success is compensated after departure, while
	// its committing allocation remains protected until refund verification.
	late := intent(t, s, b, 2, 2, 1, 3)
	run(t, s, 1)
	f.set(fmt.Sprintf("payment-%d", late.ID), "waiting_for_capture")
	ready(t, s)
	run(t, s, 1)
	f.loseCapture = true
	run(t, s, 1)
	mustExec(t, s.DB, `UPDATE checkout_orders SET departure_time=clock_timestamp()-interval '1 second' WHERE id=$1`, late.ID)
	ready(t, s)
	run(t, s, 5)
	if detail(t, s, 2, late.ID).Status != "refunded" || f.refundEffects != 1 {
		t.Fatalf("late success not compensated %+v", detail(t, s, 2, late.ID))
	}
}
func TestCheckoutListsBoundedSingleQueryAndScopedCursor(t *testing.T) {
	s, b, f, counter := fixture(t, 4)
	d := intent(t, s, b, 1, 1, 1, 3)
	// Bulk fixture rows are synthetic cancelled holds, not new provider effects.
	mustExec(t, s.DB, `INSERT INTO booking_holds(user_id,trip_id,passenger_name,boarding_stop_id,alighting_stop_id,boarding_order,alighting_order,boarding_name,alighting_name,departure_time,arrival_time,timezone,arrival_timezone,seat_number,amount_minor,is_demo,status,created_at,expires_at,idempotency_key,payload_hash) SELECT h.user_id,h.trip_id,h.passenger_name,h.boarding_stop_id,h.alighting_stop_id,h.boarding_order,h.alighting_order,h.boarding_name,h.alighting_name,h.departure_time,h.arrival_time,h.timezone,h.arrival_timezone,h.seat_number,h.amount_minor,h.is_demo,'cancelled',h.created_at,h.expires_at,'checkout-page-'||n,h.payload_hash FROM booking_holds h CROSS JOIN generate_series(1,234)n WHERE h.id=$1`, d.HoldID)
	mustExec(t, s.DB, `INSERT INTO checkout_orders(user_id,hold_id,trip_id,status,payment_status,amount_minor,currency,is_demo,passenger_name,boarding_name,alighting_name,departure_time,arrival_time,timezone,arrival_timezone,seat_number,hold_expires_at,idempotency_key) SELECT user_id,id,trip_id,'expired','not_started',amount_minor,currency,is_demo,passenger_name,boarding_name,alighting_name,departure_time,arrival_time,timezone,arrival_timezone,seat_number,expires_at,idempotency_key FROM booking_holds WHERE idempotency_key LIKE 'checkout-page-%'`)
	mustExec(t, s.DB, `INSERT INTO checkout_attempts(order_id,intent_id,create_key,capture_key,cancel_key,refund_key) SELECT o.id,o.id::text,uuid_generate_v4(),uuid_generate_v4(),uuid_generate_v4(),uuid_generate_v4() FROM checkout_orders o LEFT JOIN checkout_attempts a ON a.order_id=o.id WHERE a.order_id IS NULL; UPDATE booking_holds h SET checkout_id=o.id FROM checkout_orders o WHERE o.hold_id=h.id`)
	for _, limit := range []int{1, 30, 100} {
		counter.n.Store(0)
		page, err := s.List(context.Background(), 1, limit, "", "")
		if err != nil || len(page.Items) != limit || page.NextCursor == nil || counter.n.Load() != 1 {
			t.Fatalf("list N+1 limit=%d rows=%d calls=%d err=%v", limit, len(page.Items), counter.n.Load(), err)
		}
		if _, err = s.List(context.Background(), 2, limit, "", *page.NextCursor); code(err) != "invalid_request" {
			t.Fatalf("owner cursor reused %v", err)
		}
		if _, err = s.List(context.Background(), 1, limit, "expired", *page.NextCursor); code(err) != "invalid_request" {
			t.Fatalf("filter cursor reused %v", err)
		}
		cursor := ""
		seen := map[int64]bool{}
		for {
			page, err = s.List(context.Background(), 1, limit, "", cursor)
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range page.Items {
				if seen[row.ID] {
					t.Fatal("duplicate cursor page")
				}
				seen[row.ID] = true
			}
			if page.NextCursor == nil {
				break
			}
			cursor = *page.NextCursor
		}
		if len(seen) != 235 {
			t.Fatalf("lost rows %d", len(seen))
		}
	}
	if f.creates != 0 || f.gets != 0 {
		t.Fatal("listing created/read a provider payment")
	}
}

func TestFailedCompensationRetainsSeatAndIgnoresStaleRefund(t *testing.T) {
	s, b, f, _ := fixture(t, 1)
	d := intent(t, s, b, 1, 1, 1, 3)
	run(t, s, 1)
	pid := fmt.Sprintf("payment-%d", d.ID)
	f.set(pid, "waiting_for_capture")
	ready(t, s)
	run(t, s, 1)
	f.loseCapture = true
	run(t, s, 1)
	mustExec(t, s.DB, `UPDATE checkout_orders SET departure_time=clock_timestamp()-interval '1 second' WHERE id=$1`, d.ID)
	f.refundStatus = "canceled"
	ready(t, s)
	run(t, s, 5)
	if detail(t, s, 1, d.ID).Status != "review_required" {
		t.Fatal("failed compensation hidden")
	}
	h, err := b.Get(context.Background(), 1, d.HoldID)
	if err != nil || h.Status != "payment_committing" {
		t.Fatalf("failed compensation released uncertain seat %+v %v", h, err)
	}
	var rid string
	f.mu.Lock()
	for key, v := range f.refundKeys {
		rid = v.ID
		v.Status = "pending"
		f.refundKeys[key] = v
	}
	f.mu.Unlock()
	if err = s.IngestWebhook(context.Background(), "refund.succeeded", rid); err != nil {
		t.Fatal(err)
	}
	run(t, s, 2)
	if detail(t, s, 1, d.ID).Status != "review_required" {
		t.Fatal("stale pending refund regressed terminal review")
	}
	if err = s.IngestWebhook(context.Background(), "payment.succeeded", pid); err != nil {
		t.Fatal(err)
	}
	run(t, s, 3)
	if detail(t, s, 1, d.ID).Status != "review_required" || f.refunds != 1 {
		t.Fatal("repeated success retried terminal failed refund")
	}
}
