package pg

import (
	"context"
	"corpord-api/internal/logger"
	"corpord-api/internal/routes"
	"corpord-api/model"
	"corpord-api/pkg/dbx"
	"errors"
	"go.uber.org/zap"
	"os"
	"strings"
	"testing"
	"time"
)

func routeFixture(t *testing.T) (*routes.Service, func(string)) {
	t.Helper()
	db, _ := domainTestDB(t, true)
	domainSeed(t, db)
	exec := func(q string) {
		t.Helper()
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	exec(`UPDATE stops SET city_id=2 WHERE id=70; UPDATE stops SET city_id=5 WHERE id=71;
 UPDATE trip_stops SET stop_order=3,arrival_time='2026-12-01 00:30',departure_time='2026-12-01 00:40' WHERE id=91;
 UPDATE trip_stops SET arrival_time='2026-11-30 22:00',departure_time='2026-11-30 22:10',price_to_next=0.10 WHERE id=90;
 INSERT INTO stops(id,name,address,latitude,longitude,city_id) VALUES(72,'Промежуточная','Address C',57,61,2);
 INSERT INTO trip_stops(id,trip_id,stop_id,arrival_time,departure_time,stop_order,price_to_next) VALUES(92,80,72,'2026-11-30 23:00','2026-11-30 23:10',2,0.20);
 UPDATE trips SET base_price=999.99 WHERE id=80;`)
	s := routes.NewService(routes.NewRepository(db))
	s.Now = func() time.Time { return time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC) }
	return s, exec
}
func TestRouteSearchSectionCalendarQuoteAndLegacyMoney(t *testing.T) {
	s, exec := routeFixture(t)
	ctx := context.Background()
	q := routes.Search{From: "city:2", To: "stop:71", Date: "2026-12-01", Passengers: 1, Limit: 20}
	found, err := s.Search(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(found.Items) != 2 {
		t.Fatalf("expected both origin city stops, got %+v", found)
	}
	var full routes.Segment
	for _, i := range found.Items {
		if i.BoardingStopID == 70 {
			full = i
		}
	}
	if full.Price == nil || full.Price.AmountMinor != 30 || full.DepartureTime.Format("2006-01-02 15:04") != "2026-12-01 03:10" || full.Availability.Status != "unknown" {
		t.Fatalf("wrong section %+v", full)
	}
	quote, err := s.Quote(ctx, 80, routes.QuoteRequest{BoardingStopID: 70, AlightingStopID: 71, Passengers: 1})
	if err != nil || quote.Price.AmountMinor != 30 || quote.ReservesSeat || quote.ExpiresAt != nil {
		t.Fatalf("quote %+v %v", quote, err)
	}
	q.Month = "2026-12"
	cal, err := s.Calendar(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if cal.Days[0].TripCount != 1 || len(cal.Days) != 31 || cal.Days[0].MinPriceMinor == nil || *cal.Days[0].MinPriceMinor != 20 || cal.Days[0].Status != "unknown" || cal.Days[1].Status != "no_trips" {
		t.Fatalf("calendar %+v", cal)
	}
	exec(`UPDATE trip_stops SET price_to_next=12.34 WHERE id=90`)
	quote, err = s.Quote(ctx, 80, routes.QuoteRequest{BoardingStopID: 70, AlightingStopID: 71})
	if err != nil || quote.Price.AmountMinor != 1254 {
		t.Fatalf("legacy update drift %+v %v", quote, err)
	}
	_, err = s.Quote(ctx, 80, routes.QuoteRequest{BoardingStopID: 71, AlightingStopID: 70})
	var api *routes.Error
	if !errors.As(err, &api) || api.Status != 404 {
		t.Fatalf("reverse quote %v", err)
	}
	exec(`UPDATE trips SET status='cancelled' WHERE id=80`)
	found, err = s.Search(ctx, q)
	if err != nil || len(found.Items) != 0 {
		t.Fatalf("cancelled %+v %v", found, err)
	}
	cal, err = s.Calendar(ctx, q)
	if err != nil || cal.Days[0].Status != "no_trips" || cal.Days[0].MinPriceMinor != nil {
		t.Fatalf("cancelled calendar %+v %v", cal, err)
	}
}
func TestRouteMissingPriceAndTimezonesRemainHonest(t *testing.T) {
	s, exec := routeFixture(t)
	ctx := context.Background()
	q := routes.Search{From: "stop:70", To: "stop:71", Date: "2026-12-01", Month: "2026-12"}
	for _, sql := range []string{`UPDATE trip_stops SET price_to_next=NULL WHERE id=90`, `UPDATE trip_stops SET price_to_next=-0.10 WHERE id=90`, `UPDATE trip_stops SET price_to_next=0.10 WHERE id=90;UPDATE stops SET city_id=NULL WHERE id=71`} {
		exec(sql)
		found, err := s.Search(ctx, q)
		if err != nil || len(found.Items) != 1 || found.Items[0].Price != nil {
			t.Fatalf("unconfirmed tariff %+v %v", found, err)
		}
		cal, err := s.Calendar(ctx, q)
		if err != nil || cal.Days[0].Status != "unknown" || cal.Days[0].MinPriceMinor != nil {
			t.Fatalf("unconfirmed calendar %+v %v", cal, err)
		}
	}
	exec(`UPDATE stops SET city_id=NULL WHERE id=70`)
	_, err := s.Search(ctx, q)
	var api *routes.Error
	if !errors.As(err, &api) || api.Code != "timezone_required" {
		t.Fatalf("timezone guessed %v", err)
	}
}
func TestRouteCursorRankMappingAndMigrationRollback(t *testing.T) {
	s, exec := routeFixture(t)
	ctx := context.Background()
	exec(`INSERT INTO trips(id,bus_id,driver_id,start_time,end_time,status,base_price) SELECT 82,bus_id,driver_id,start_time,end_time,status,base_price FROM trips WHERE id=80;
 INSERT INTO trip_stops(trip_id,stop_id,arrival_time,departure_time,stop_order,price_to_next) SELECT 82,stop_id,arrival_time,departure_time,stop_order,price_to_next FROM trip_stops WHERE trip_id=80;`)
	for _, sort := range []string{"departure", "price"} {
		q := routes.Search{From: "stop:70", To: "stop:71", Date: "2026-12-01", Limit: 1, Sort: sort}
		a, err := s.Search(ctx, q)
		if err != nil || len(a.Items) != 1 || a.NextCursor == nil {
			t.Fatalf("page 1 %+v %v", a, err)
		}
		q.Cursor = *a.NextCursor
		b, err := s.Search(ctx, q)
		if err != nil || len(b.Items) != 1 || b.Items[0].TripID == a.Items[0].TripID || b.NextCursor != nil {
			t.Fatalf("page 2 %+v %v", b, err)
		}
	}
	cities, err := s.Repo.Suggest(ctx, "Мос", "", 8)
	if err != nil || len(cities) != 1 || cities[0].Served || cities[0].ID != "city:1" {
		t.Fatalf("Moscow invented service %+v %v", cities, err)
	}
	cities, err = s.Repo.Suggest(ctx, "", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cities {
		if !c.Served {
			t.Fatalf("unserved default %+v", c)
		}
	}
	cities, err = s.Repo.Suggest(ctx, "%", "", 20)
	if err != nil || len(cities) != 0 {
		t.Fatalf("wildcard interpreted %+v %v", cities, err)
	}
	// Explicit mapping through the existing stop repository roundtrips new fields.
	id := 1
	zone := "Europe/Moscow"
	logDB, log := s.Repo.DB, loggerForRouteTest()
	stopRepo := NewStop(log, dbx.NewQueryBuilder(logDB))
	st := &model.Stop{Name: "Mapped", Address: "Test", CityID: &id, Timezone: &zone}
	if err := stopRepo.Create(ctx, st); err != nil {
		t.Fatal(err)
	}
	got, err := stopRepo.ByID(ctx, st.ID)
	if err != nil || got.CityID == nil || *got.CityID != id || got.Timezone == nil {
		t.Fatalf("mapping %+v %v", got, err)
	}
	clear := 0
	empty := ""
	if err := stopRepo.Update(ctx, &model.StopUpdate{ID: st.ID, CityID: &clear, Timezone: &empty}); err != nil {
		t.Fatal(err)
	}
	got, err = stopRepo.ByID(ctx, st.ID)
	if err != nil || got.CityID != nil || got.Timezone != nil {
		t.Fatalf("clear mapping %+v %v", got, err)
	}
	content, err := os.ReadFile("../../../db/migrations/20261101000100_route_locations_money.sql")
	if err != nil {
		t.Fatal(err)
	}
	sections := strings.Split(string(content), "-- +goose Down")
	exec(sections[1])
	var price string
	if err := s.Repo.DB.Get(&price, `SELECT base_price::text FROM trips WHERE id=80`); err != nil || price != "999.99" {
		t.Fatalf("rollback lost decimals %s %v", price, err)
	}
	exec(sections[0])
}

func loggerForRouteTest() *logger.Logger { return &logger.Logger{SugaredLogger: zap.NewNop().Sugar()} }

func TestRouteMixedUnpricedAndAmbiguousPairs(t *testing.T) {
	s, exec := routeFixture(t)
	ctx := context.Background()
	exec(`INSERT INTO trips(id,bus_id,driver_id,start_time,end_time,status,base_price) SELECT 82,bus_id,driver_id,start_time,end_time,status,base_price FROM trips WHERE id=80;
 INSERT INTO trip_stops(trip_id,stop_id,arrival_time,departure_time,stop_order,price_to_next) SELECT 82,stop_id,arrival_time,departure_time,stop_order,NULL FROM trip_stops WHERE trip_id=80;`)
	q := routes.Search{From: "stop:70", To: "stop:71", Date: "2026-12-01", Month: "2026-12", Sort: "price", Limit: 1}
	a, err := s.Search(ctx, q)
	if err != nil || len(a.Items) != 1 || a.Items[0].Price == nil || a.NextCursor == nil {
		t.Fatalf("mixed page1 %+v %v", a, err)
	}
	q.Cursor = *a.NextCursor
	b, err := s.Search(ctx, q)
	if err != nil || len(b.Items) != 1 || b.Items[0].Price != nil || b.Items[0].TripID != 82 {
		t.Fatalf("mixed page2 %+v %v", b, err)
	}
	cal, err := s.Calendar(ctx, q)
	if err != nil || cal.Days[0].TripCount != 2 || cal.Days[0].MinPriceMinor == nil || *cal.Days[0].MinPriceMinor != 30 {
		t.Fatalf("mixed calendar %+v %v", cal, err)
	}
	exec(`INSERT INTO trip_stops(trip_id,stop_id,arrival_time,departure_time,stop_order,price_to_next) VALUES(82,71,'2026-12-01 01:30','2026-12-01 01:40',4,0)`)
	q.Cursor = ""
	a, err = s.Search(ctx, q)
	if err != nil || len(a.Items) != 1 || a.NextCursor != nil || a.Items[0].TripID != 80 {
		t.Fatalf("ambiguous pair advertised %+v %v", a, err)
	}
	cal, err = s.Calendar(ctx, q)
	if err != nil || cal.Days[0].TripCount != 1 {
		t.Fatalf("ambiguous calendar %+v %v", cal, err)
	}
	_, err = s.Quote(ctx, 82, routes.QuoteRequest{BoardingStopID: 70, AlightingStopID: 71})
	var api *routes.Error
	if !errors.As(err, &api) || api.Code != "ambiguous_segment" {
		t.Fatalf("ambiguous quote %v", err)
	}
}
