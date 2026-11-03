package pg

import (
	"context"
	"corpord-api/internal/routes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

type queryCounter struct{ calls atomic.Int64 }

func (q *queryCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	q.calls.Add(1)
	return ctx
}
func (q *queryCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestCatalogBoundedPagesIndexesAndConstantQueryCount(t *testing.T) {
	service, exec := routeFixture(t)
	exec(`SELECT setval('stops_id_seq',(SELECT max(id) FROM stops)); INSERT INTO stops(name,address,latitude,longitude,city_id)
 SELECT 'Ёлочная остановка '||g,'Улица тестовая',56.8,60.6,2 FROM generate_series(1,240) g`)
	ctx := context.Background()
	var schema string
	if err := service.Repo.DB.Get(&schema, `SELECT current_schema()`); err != nil {
		t.Fatal(err)
	}
	cfg, err := pgx.ParseConfig(os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.RuntimeParams["search_path"] = schema + ",public"
	counter := &queryCounter{}
	cfg.Tracer = counter
	db := sqlx.NewDb(stdlib.OpenDB(*cfg), "pgx")
	defer db.Close()
	repo := routes.NewRepository(db)
	for _, limit := range []int{1, 30, 100} {
		counter.calls.Store(0)
		page, err := repo.Catalog(ctx, routes.PageQuery{Kind: "stops", Q: "елочная", CityID: 2, Limit: limit})
		if err != nil || len(page.Items) != limit || page.NextCursor == nil {
			t.Fatalf("page limit %d: %+v %v", limit, page, err)
		}
		if got := counter.calls.Load(); got != 1 {
			t.Fatalf("N+1: %d queries for %d rows", got, limit)
		}
		seen := map[int]bool{}
		q := routes.PageQuery{Kind: "stops", Q: "елочная", CityID: 2, Limit: limit}
		for {
			p, err := repo.Catalog(ctx, q)
			if err != nil {
				t.Fatal(err)
			}
			for _, raw := range p.Items {
				var row struct{ ID int }
				if err = json.Unmarshal(raw, &row); err != nil {
					t.Fatal(err)
				}
				if seen[row.ID] {
					t.Fatal("duplicate page row")
				}
				seen[row.ID] = true
			}
			if p.NextCursor == nil {
				break
			}
			q.Cursor = *p.NextCursor
		}
		if len(seen) != 240 {
			t.Fatalf("lost rows: %d", len(seen))
		}
		_, err = repo.Catalog(ctx, routes.PageQuery{Kind: "stops", Q: "other", CityID: 2, Limit: limit, Cursor: *page.NextCursor})
		if err == nil {
			t.Fatal("cursor reused with another query")
		}
	}
	for _, q := range []string{"%", "_", `\`} {
		page, err := repo.Catalog(ctx, routes.PageQuery{Kind: "stops", Q: q, Limit: 30})
		if err != nil || len(page.Items) != 0 {
			t.Fatalf("wildcard %s: %+v %v", q, page, err)
		}
	}
	for _, q := range []routes.PageQuery{{Kind: "stops", Limit: 101}, {Kind: "passwords", Limit: 10}, {Kind: "cities", Limit: 0}, {Kind: "stops", Limit: 10, Cursor: "broken"}} {
		if _, err := repo.Catalog(ctx, q); err == nil {
			t.Fatalf("accepted invalid page %+v", q)
		}
	}
	for _, limit := range []int{1, 20} {
		counter.calls.Store(0)
		items, err := repo.Suggest(ctx, "елоч", "", limit)
		if err != nil || len(items) != limit {
			t.Fatalf("suggest %+v %v", items, err)
		}
		if counter.calls.Load() != 1 {
			t.Fatal("suggest query count grows with page")
		}
	}
	exec(`UPDATE route_cities SET region='Проверенный регион' WHERE id=2`)
	loc, err := repo.Location(ctx, "stop:70")
	if err != nil || loc.Region != "Проверенный регион" {
		t.Fatalf("stale region %+v %v", loc, err)
	}
	regional, err := repo.Suggest(ctx, "", "", 20, "Проверенный регион")
	if err != nil || len(regional) == 0 {
		t.Fatalf("region search %+v %v", regional, err)
	}
	for _, item := range regional {
		if item.Region != "Проверенный регион" {
			t.Fatal("region leaked")
		}
	}
	exec(`DELETE FROM stops WHERE name='Ёлочная остановка 240'`)
	var ghosts int
	if err := db.Get(&ghosts, `SELECT count(*) FROM catalog_search WHERE name='Ёлочная остановка 240'`); err != nil || ghosts != 0 {
		t.Fatalf("index delete: %d %v", ghosts, err)
	}
	// A larger result page still uses exactly two location checks and one
	// set-based segment query. Fare calculation cannot issue per-card requests.
	for _, limit := range []int{1, 20} {
		counter.calls.Store(0)
		s := routes.NewService(repo)
		s.Now = func() time.Time { return time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC) }
		_, err := s.Search(ctx, routes.Search{From: "stop:70", To: "stop:71", Date: "2026-12-01", Limit: limit})
		if err != nil {
			t.Fatal(err)
		}
		if counter.calls.Load() != 3 {
			t.Fatalf("search executed %d queries at limit %d", counter.calls.Load(), limit)
		}
	}
	// TIMESTAMP columns retain the legacy UTC semantics in JSON. Without an
	// explicit offset, browser Date would reinterpret summaries in device time.
	for _, check := range []struct{ kind, field string }{{"trips", "start_time"}, {"tripStops", "departure_time"}} {
		page, err := repo.Catalog(ctx, routes.PageQuery{Kind: check.kind, Limit: 1})
		if err != nil || len(page.Items) != 1 {
			t.Fatalf("timestamp catalog %s: %v", check.kind, err)
		}
		var row map[string]any
		if err := json.Unmarshal(page.Items[0], &row); err != nil {
			t.Fatal(err)
		}
		stamp, ok := row[check.field].(string)
		if !ok || !strings.HasSuffix(stamp, "Z") {
			t.Fatalf("ambiguous catalog timestamp %s: %v", check.field, row[check.field])
		}
		if _, err := time.Parse(time.RFC3339Nano, stamp); err != nil {
			t.Fatal(err)
		}
	}
	// The underlying prefix and trigram indexes must actually exist in this schema.
	var indexes int
	if err := db.Get(&indexes, `SELECT count(*) FROM pg_indexes WHERE schemaname=$1 AND indexname IN ('catalog_prefix','catalog_name_fuzzy','catalog_document','trip_stops_stop_departure')`, schema); err != nil || indexes != 4 {
		t.Fatal(fmt.Sprintf("missing indexes %d: %v", indexes, err))
	}
}

func TestCatalogMigrationRollbackRefusesToInventTimezone(t *testing.T) {
	service, exec := routeFixture(t)
	content, err := os.ReadFile("../../../db/migrations/20261103000100_catalog_search.sql")
	if err != nil {
		t.Fatal(err)
	}
	sections := strings.Split(string(content), "-- +goose Down")
	exec(`UPDATE route_cities SET timezone=NULL WHERE id=1`)
	tx, err := service.Repo.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(sections[1])
	tx.Rollback()
	if err == nil {
		t.Fatal("rollback must refuse unknown timezone")
	}
	var exists bool
	if err := service.Repo.DB.Get(&exists, `SELECT EXISTS(SELECT 1 FROM catalog_search WHERE kind='city' AND entity_id=1)`); err != nil || !exists {
		t.Fatalf("failed rollback lost index: %v", err)
	}
	city := routes.City{ID: 1, Name: "Москва", Region: "Москва", Timezone: "Europe/Moscow", Popularity: 100}
	if err := service.Repo.SaveCity(context.Background(), &city); err != nil {
		t.Fatalf("explicit missing timezone repair: %v", err)
	}
	exec(sections[1])
	exec(sections[0])
	exec(`UPDATE stops SET name='После rollback' WHERE id=70`)
	location, err := service.Repo.Location(context.Background(), "stop:70")
	if err != nil || location.Name != "После rollback" {
		t.Fatalf("restored triggers: %+v %v", location, err)
	}
}
