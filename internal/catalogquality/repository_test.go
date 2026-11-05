package catalogquality

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

type counter struct{ n atomic.Int64 }

func (c *counter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	c.n.Add(1)
	return ctx
}
func (c *counter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
func fixture(t *testing.T) (*Repository, *counter) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL required for isolated PostgreSQL integration")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin := sqlx.NewDb(stdlib.OpenDB(*cfg), "pgx")
	schema := "quality_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	tx, err := admin.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`SELECT pg_advisory_xact_lock(1146376787); CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public; CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public; CREATE EXTENSION IF NOT EXISTS btree_gist WITH SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	cfg.RuntimeParams["search_path"] = schema + ",public"
	cfg.RuntimeParams["application_name"] = schema
	cfg.RuntimeParams["jit"] = "off"
	c := &counter{}
	cfg.Tracer = c
	db := sqlx.NewDb(stdlib.OpenDB(*cfg), "pgx")
	t.Cleanup(func() { db.Close(); admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); admin.Close() })
	paths, err := filepath.Glob("../../db/migrations/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		b, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		up := strings.Split(string(b), "-- +goose Down")[0]
		up = strings.ReplaceAll(up, `CREATE EXTENSION "uuid-ossp";`, "")
		if _, err = db.Exec(up); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	_, err = db.Exec(`INSERT INTO users(id,name,email,password_hash,role_id) VALUES(900,'Test editor','quality-test@example.invalid','unused',(SELECT id FROM roles WHERE name='admin'));
 INSERT INTO stops(id,name,address,latitude,longitude) SELECT 1000+g,'Проверочная остановка '||g,'Тестовый адрес',56.8,60.6 FROM generate_series(1,105)g;
 INSERT INTO bus_statuses(id,name) VALUES(900,'Ready');INSERT INTO bus_categories(id,name)VALUES(900,'Coach');INSERT INTO driver_status(id,name)VALUES(900,'Available');
 INSERT INTO bus(id,license_plate,brand,capacity,category_id,status_id)VALUES(900,'TESTQUALITY','Test',2,900,900);
 INSERT INTO drivers(id,first_name,last_name,middle_name,phone_number,status)VALUES(900,'Test','Test','','900',900);
 INSERT INTO trips(id,bus_id,driver_id,start_time,end_time,status,base_price)VALUES(900,900,900,'2026-12-01 00:00','2026-12-01 01:00','scheduled',10);`)
	if err != nil {
		t.Fatal(err)
	}
	return New(db), c
}
func code(t *testing.T, err error, want string) {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) || e.Code != want {
		t.Fatalf("want %s, got %v", want, err)
	}
}
func TestMappingVersionAuditSearchAndProtection(t *testing.T) {
	r, _ := fixture(t)
	ctx := context.Background()
	var snapshot string
	_, err := r.DB.Exec(`UPDATE stops SET source_key='osm:n:test',source_snapshot='{"city_id":null,"name":"Original"}' WHERE id=1001`)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.DB.Get(&snapshot, `SELECT source_snapshot::text FROM stops WHERE id=1001`); err != nil {
		t.Fatal(err)
	}
	page, err := r.Queue(ctx, "Проверочная", "", 30)
	if err != nil || len(page.Items) != 30 {
		t.Fatalf("page %+v %v", page, err)
	}
	mapped, err := r.Map(ctx, 900, 1001, 2, page.Items[0].Version, "Проверено по муниципальному перечню")
	if err != nil || mapped.CityID == nil || *mapped.CityID != 2 || mapped.Version == page.Items[0].Version {
		t.Fatalf("map %+v %v", mapped, err)
	}
	_, err = r.Map(ctx, 900, 1001, 5, page.Items[0].Version, "Повторная правка")
	code(t, err, "stale_version")
	history, err := r.History(ctx, 1001, 20)
	if err != nil || len(history) != 1 || history[0].ActorID != 900 {
		t.Fatalf("history %+v %v", history, err)
	}
	var unchanged, cityName string
	if err = r.DB.Get(&unchanged, `SELECT source_snapshot::text FROM stops WHERE id=1001`); err != nil || unchanged != snapshot {
		t.Fatal("snapshot was overwritten", err)
	}
	if err = r.DB.Get(&cityName, `SELECT city_name FROM catalog_search WHERE kind='stop' AND entity_id=1001`); err != nil || cityName == "" {
		t.Fatal("catalog trigger stale", err)
	}
	// Importer's ownership predicate sees the manual change against its snapshot.
	var changed bool
	if err = r.DB.Get(&changed, `SELECT city_id IS DISTINCT FROM (source_snapshot->>'city_id')::integer FROM stops WHERE id=1001`); err != nil || !changed {
		t.Fatal("manual ownership protection missing", err)
	}
	_, err = r.DB.Exec(`INSERT INTO trip_stops(trip_id,stop_id,stop_order,arrival_time,departure_time,price_to_next)VALUES(900,1002,1,'2026-12-01','2026-12-01',10);UPDATE stops SET timezone='Asia/Tokyo' WHERE id=1003`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Map(ctx, 900, 1002, 2, "1", "Проверено по источнику")
	code(t, err, "stop_in_use")
	_, err = r.Map(ctx, 900, 1003, 2, "2", "Проверено по источнику")
	code(t, err, "timezone_conflict")
	_, err = r.Map(ctx, 900, 1999, 2, "1", "Проверено по источнику")
	code(t, err, "not_found")
	_, err = r.Map(ctx, 900, 1004, 999999, "1", "Проверено по источнику")
	code(t, err, "not_found")
	history, err = r.History(ctx, 1004, 20)
	if err != nil || len(history) != 0 {
		t.Fatalf("empty history %+v %v", history, err)
	}
	_, err = r.History(ctx, 999999, 20)
	code(t, err, "not_found")
}
func TestQueueOneQueryPaginationAndLiteralSearch(t *testing.T) {
	r, c := fixture(t)
	ctx := context.Background()
	for _, limit := range []int{1, 30, 100} {
		c.n.Store(0)
		p, err := r.Queue(ctx, "Проверочная", "", limit)
		if err != nil || len(p.Items) != limit || p.NextCursor == nil {
			t.Fatalf("page %d %+v %v", limit, p, err)
		}
		if c.n.Load() != 1 {
			t.Fatalf("N+1 %d", c.n.Load())
		}
		_, err = r.Queue(ctx, "другой", *p.NextCursor, limit)
		code(t, err, "invalid_cursor")
	}
	seen := map[int]bool{}
	cursor := ""
	for {
		p, err := r.Queue(ctx, "Проверочная", cursor, 30)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range p.Items {
			if seen[s.ID] {
				t.Fatal("duplicate page")
			}
			seen[s.ID] = true
		}
		if p.NextCursor == nil {
			break
		}
		cursor = *p.NextCursor
	}
	if len(seen) != 105 {
		t.Fatal("lost queue rows", len(seen))
	}
	for _, q := range []string{"%", "_", `\`} {
		p, err := r.Queue(ctx, q, "", 30)
		if err != nil || len(p.Items) != 0 {
			t.Fatalf("wildcard %s %+v %v", q, p, err)
		}
	}
	c.n.Store(0)
	summary, err := r.Summary(ctx)
	if err != nil || summary.Unmapped != 105 || summary.Protected != 0 || c.n.Load() != 1 {
		t.Fatalf("summary %+v %v queries%d", summary, err, c.n.Load())
	}
}
func TestMappingScheduleInsertionRace(t *testing.T) {
	r, _ := fixture(t)
	ctx := context.Background()
	// A schedule insertion holds SHARE; mapping waits, then sees its committed row.
	tx, err := r.DB.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	_, err = tx.Exec(`INSERT INTO trip_stops(trip_id,stop_id,stop_order,arrival_time,departure_time,price_to_next)VALUES(900,1001,1,'2026-12-01','2026-12-01',10)`)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := r.Map(ctx, 900, 1001, 2, "1", "Проверено по источнику")
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("mapping did not wait on schedule insert: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		code(t, err, "stop_in_use")
	case <-time.After(3 * time.Second):
		t.Fatal("mapping deadlocked")
	}
	// Mapping owns UPDATE; schedule insertion cannot bypass the lock.
	tx, err = r.DB.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`SELECT 1 FROM stops WHERE id=1002 FOR UPDATE`); err != nil {
		t.Fatal(err)
	}
	insert := make(chan error, 1)
	go func() {
		_, err := r.DB.Exec(`INSERT INTO trip_stops(trip_id,stop_id,stop_order,arrival_time,departure_time,price_to_next)VALUES(900,1002,2,'2026-12-01','2026-12-01',0)`)
		insert <- err
	}()
	select {
	case err = <-insert:
		t.Fatalf("insert bypassed mapping row lock: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if _, err = tx.Exec(`UPDATE stops SET city_id=2 WHERE id=1002`); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-insert:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("insertion deadlocked")
	}
}
func TestQualityMigrationDownUp(t *testing.T) {
	r, _ := fixture(t)
	b, err := os.ReadFile("../../db/migrations/20261105000200_catalog_quality.sql")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(b), "-- +goose Down")
	for _, sql := range []string{parts[1], parts[0]} {
		if _, err = r.DB.Exec(sql); err != nil {
			t.Fatal(fmt.Errorf("quality migration: %w", err))
		}
	}
}

func TestMappingFollowsImporterCityThenStopLockOrder(t *testing.T) {
	r, _ := fixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var schema string
	if err := r.DB.Get(&schema, `SELECT current_schema()`); err != nil {
		t.Fatal(err)
	}
	writer, err := r.DB.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback()
	// Match import_catalog.py: update a city first, then update an owned stop.
	if _, err = writer.ExecContext(ctx, `UPDATE route_cities SET popularity=popularity+1 WHERE id=2`); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := r.Map(ctx, 900, 1001, 2, "1", "Проверено по муниципальному перечню")
		done <- err
	}()
	// Wait for the actual lock wait, scoped to this disposable schema's
	// application_name. A sleep alone could miss the former stop→city cycle.
	deadline := time.Now().Add(2 * time.Second)
	for {
		var waiting bool
		if err = r.DB.GetContext(ctx, &waiting, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE application_name=$1 AND wait_event_type='Lock' AND query LIKE '%FROM route_cities WHERE id=$1 FOR SHARE%')`, schema); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("mapping did not wait on importer city lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err = writer.ExecContext(ctx, `UPDATE stops SET address='Изменение источника' WHERE id=1001`); err != nil {
		t.Fatalf("inconsistent city/stop lock order: %v", err)
	}
	if err = writer.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		code(t, err, "stale_version")
	case <-ctx.Done():
		t.Fatal("mapping and importer deadlocked")
	}
	history, err := r.History(ctx, 1001, 20)
	if err != nil || len(history) != 0 {
		t.Fatalf("failed mapping wrote audit: %+v %v", history, err)
	}
}
