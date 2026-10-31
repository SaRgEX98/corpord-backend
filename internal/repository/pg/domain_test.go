package pg

import (
	"context"
	"corpord-api/internal/logger"
	"corpord-api/model"
	"corpord-api/pkg/dbx"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

// Domain tests use an isolated schema in a disposable PostgreSQL database.
func domainTestDB(t *testing.T, migrate bool) (*sqlx.DB, *logger.Logger) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin := sqlx.NewDb(stdlib.OpenDB(*cfg), "pgx")
	schema := "domain_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	// Extensions are database-scoped. Keep UUID functions in a persistent
	// schema so cleanup of one isolated test cannot break another test process.
	// The lock also serializes CREATE EXTENSION across concurrent test runs.
	var extensionSchema string
	if migrate {
		tx, err := admin.BeginTxx(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if _, err := tx.Exec("SELECT pg_advisory_xact_lock(1146376787)"); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public`); err != nil {
			t.Fatal(err)
		}
		if err := tx.Get(&extensionSchema, `SELECT n.nspname FROM pg_extension e JOIN pg_namespace n ON n.oid=e.extnamespace WHERE e.extname='uuid-ossp'`); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	cfg.RuntimeParams["search_path"] = schema
	if extensionSchema != "" {
		cfg.RuntimeParams["search_path"] += "," + extensionSchema
	}
	db := sqlx.NewDb(stdlib.OpenDB(*cfg), "pgx")
	t.Cleanup(func() {
		db.Close()
		if _, err := admin.Exec("DROP SCHEMA " + schema + " CASCADE"); err != nil {
			t.Errorf("cleanup schema: %v", err)
		}
		admin.Close()
	})
	if migrate {
		paths, err := filepath.Glob("../../../db/migrations/*.sql")
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(paths)
		for _, path := range paths {
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			up := strings.Split(string(content), "-- +goose Down")[0]
			// Already prepared outside the per-test schema above.
			up = strings.ReplaceAll(up, `CREATE EXTENSION "uuid-ossp";`, "")
			if _, err := db.Exec(up); err != nil {
				t.Fatalf("migration %s: %v", filepath.Base(path), err)
			}
		}
	}
	return db, &logger.Logger{SugaredLogger: zap.NewNop().Sugar()}
}

func domainSeed(t *testing.T, db *sqlx.DB) {
	t.Helper()
	_, err := db.Exec(`
 INSERT INTO bus_statuses(id,name) VALUES(20,'Ready');
 INSERT INTO bus_categories(id,name) VALUES(30,'Coach');
 INSERT INTO driver_status(id,name) VALUES(40,'Available');
 INSERT INTO bus(id,license_plate,brand,capacity,category_id,status_id) VALUES(50,'A123BC','Bus A',45,30,20),(51,'0099','Bus B',30,30,20);
 INSERT INTO drivers(id,first_name,last_name,middle_name,phone_number,status) VALUES(60,'First','Last','','123',40),(61,'Other','Driver','','456',40);
 INSERT INTO stops(id,name,address,latitude,longitude) VALUES(70,'Start','Address A',56.1,60.2),(71,'End','Address B',56.2,60.3);
 INSERT INTO trips(id,bus_id,driver_id,start_time,end_time,status,base_price) VALUES
 (80,50,60,'2026-12-01 08:00','2026-12-01 12:00','scheduled',123.45),
 (81,51,61,'2026-12-02 08:00',NULL,'scheduled',432.10);
 INSERT INTO trip_stops(id,trip_id,stop_id,arrival_time,departure_time,stop_order,price_to_next) VALUES
 (91,80,71,'2026-12-01 11:30','2026-12-01 11:40',2,0),
 (90,80,70,'2026-12-01 08:00','2026-12-01 08:10',1,123.45);
 `)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDomainPostgresTripReadIDsAndOrderedStops(t *testing.T) {
	db, log := domainTestDB(t, true)
	domainSeed(t, db)
	ctx := context.Background()
	repo := NewTrip(log, dbx.NewQueryBuilder(db))
	all, err := repo.All(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].ID != 80 || all[1].ID != 81 {
		t.Fatalf("trips duplicated/lost: %+v", all)
	}
	first, err := repo.ByID(ctx, 80)
	if err != nil {
		t.Fatal(err)
	}
	if first.Bus.ID != 50 || first.Driver.ID != 60 || first.BasePrice != 123.45 || len(first.Stops) != 2 {
		t.Fatalf("bad response: %+v", first)
	}
	if first.Stops[0].ID != 90 || first.Stops[0].StopID != 70 || first.Stops[0].Stop.ID != 70 || first.Stops[1].ID != 91 {
		t.Fatalf("stops not ordered or IDs lost: %+v", first.Stops)
	}
	second, err := repo.ByID(ctx, 81)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != 81 || second.Bus.ID != 51 || second.Driver.ID != 61 || second.EndTime != nil || second.Stops == nil || len(second.Stops) != 0 {
		t.Fatalf("wrong second/no-stop trip: %+v", second)
	}
	if _, err := repo.ByID(ctx, 999999); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing trip returned wrong row/error: %v", err)
	}
}

func TestDomainPostgresPartialBusAndMissingMutations(t *testing.T) {
	db, log := domainTestDB(t, true)
	domainSeed(t, db)
	ctx := context.Background()
	qb := dbx.NewQueryBuilder(db)
	repo := NewBusRepository(log, qb)
	brand := "Changed"
	if err := repo.UpdateBus(ctx, &model.BusUpdate{ID: 50, Brand: &brand}); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetBus(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	if got.Brand != brand || got.Capacity != 45 || got.LicensePlate != "A123BC" || got.Category != "Coach" || got.Status != "Ready" {
		t.Fatalf("partial update destroyed fields: %+v", got)
	}
	stopName := "Renamed"
	status := "boarding"
	order := 3
	checks := []struct {
		name string
		err  error
	}{
		{"bus update", repo.UpdateBus(ctx, &model.BusUpdate{ID: 999999, Brand: &brand})},
		{"bus delete", repo.DeleteBus(ctx, 999999)},
		{"stop update", NewStop(log, qb).Update(ctx, &model.StopUpdate{ID: 999999, Name: &stopName})},
		{"stop delete", NewStop(log, qb).Delete(ctx, 999999)},
		{"trip update", NewTrip(log, qb).Update(ctx, &model.TripUpdate{ID: 999999, Status: &status})},
		{"trip delete", NewTrip(log, qb).Delete(ctx, 999999)},
		{"trip stop update", NewTripStop(log, qb).Update(ctx, &model.TripStopUpdate{ID: 999999, StopOrder: &order})},
		{"trip stop delete", NewTripStop(log, qb).Delete(ctx, 999999)},
		{"driver update", NewDriver(log, qb).Update(ctx, model.DriverInput{ID: 999999, FirstName: "F", LastName: "L", PhoneNumber: "1", Status: 40})},
		{"driver delete", NewDriver(log, qb).Delete(ctx, 999999)},
		{"driver status update", NewDriverStatus(log, qb).Update(ctx, &model.DriverStatus{ID: 999999, Name: "missing"})},
		{"driver status delete", NewDriverStatus(log, qb).Delete(ctx, 999999)},
		{"bus category delete", NewBusCategory(log, qb).Delete(ctx, 999999)},
		{"bus status delete", NewBusStatus(log, qb).Delete(ctx, 999999)},
	}
	for _, check := range checks {
		if !errors.Is(check.err, ErrNotFound) {
			t.Errorf("%s: want not found, got %v", check.name, check.err)
		}
	}
	if _, err := NewBusCategory(log, qb).Update(ctx, model.BusCategory{ID: 999999, Name: "missing"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("category update: %v", err)
	}
	if _, err := NewBusStatus(log, qb).Update(ctx, model.BusStatus{ID: 999999, Name: "missing"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("status update: %v", err)
	}
}

func TestDomainPostgresEntityIDsAndCreates(t *testing.T) {
	db, log := domainTestDB(t, true)
	domainSeed(t, db)
	ctx := context.Background()
	qb := dbx.NewQueryBuilder(db)
	stops, err := NewStop(log, qb).All(ctx)
	if err != nil || len(stops) != 2 || stops[0].ID == 0 {
		t.Fatalf("stop IDs: %+v %v", stops, err)
	}
	driver, err := NewDriver(log, qb).ByID(ctx, 60)
	if err != nil || driver.ID != 60 {
		t.Fatalf("driver ID: %+v %v", driver, err)
	}
	tripStop, err := NewTripStop(log, qb).ByID(ctx, 90)
	if err != nil || tripStop.ID != 90 {
		t.Fatalf("trip stop ID: %+v %v", tripStop, err)
	}
	stop := &model.Stop{Name: "New", Address: "New address", Latitude: 1, Longitude: 2}
	if err := NewStop(log, qb).Create(ctx, stop); err != nil || stop.ID <= 0 {
		t.Fatalf("created stop ID=%d err=%v", stop.ID, err)
	}
	start := time.Date(2026, 12, 3, 10, 0, 0, 0, time.UTC)
	trip := &model.Trip{BusID: 50, DriverID: 60, StartTime: start, EndTime: start.Add(time.Hour), Status: "scheduled", BasePrice: 12.34}
	if err := NewTrip(log, qb).Create(ctx, trip); err != nil || trip.ID <= 0 || trip.CreatedAt.IsZero() {
		t.Fatalf("created trip: %+v err=%v", trip, err)
	}
	if got, err := NewTrip(log, qb).ByID(ctx, trip.ID); err != nil || got.BasePrice != 12.34 {
		t.Fatalf("decimal price: %+v %v", got, err)
	}
}

func TestDomainPostgresBusIDMigrationRoundTrip(t *testing.T) {
	db, _ := domainTestDB(t, false)
	_, err := db.Exec(`CREATE TABLE bus(license_plate TEXT PRIMARY KEY,brand TEXT NOT NULL);
 CREATE TABLE trips(id SERIAL PRIMARY KEY,bus_id TEXT NOT NULL REFERENCES bus(license_plate));
 CREATE INDEX idx_trips_bus_id ON trips(bus_id);
 INSERT INTO bus VALUES ('A123BC','letter'),('0099','numeric'),('888','different');
 INSERT INTO trips(bus_id) VALUES ('0099'),('A123BC'),('888'),('A123BC');`)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile("../../../db/migrations/20251005230343_id_bus.sql")
	if err != nil {
		t.Fatal(err)
	}
	sections := strings.Split(string(content), "-- +goose Down")
	if _, err := db.Exec(sections[0]); err != nil {
		t.Fatalf("up: %v", err)
	}
	var plates []string
	if err := db.Select(&plates, "SELECT b.license_plate FROM trips t JOIN bus b ON b.id=t.bus_id ORDER BY t.id"); err != nil {
		t.Fatal(err)
	}
	want := []string{"0099", "A123BC", "888", "A123BC"}
	if strings.Join(plates, ",") != strings.Join(want, ",") {
		t.Fatalf("up lost references: %v", plates)
	}
	if _, err := db.Exec(sections[1]); err != nil {
		t.Fatalf("down: %v", err)
	}
	if err := db.Select(&plates, "SELECT bus_id FROM trips ORDER BY id"); err != nil {
		t.Fatal(err)
	}
	if strings.Join(plates, ",") != strings.Join(want, ",") {
		t.Fatalf("down lost plates: %v", plates)
	}
	if _, err := db.Exec("INSERT INTO trips(bus_id) VALUES ('missing')"); !IsPgError(err, ErrorCodeForeignKeyViolation) {
		t.Fatalf("rollback lost FK: %v", err)
	}
	// A second upgrade also verifies sequence/index/constraint names survived rollback.
	if _, err := db.Exec(sections[0]); err != nil {
		t.Fatalf("second up: %v", err)
	}
}
