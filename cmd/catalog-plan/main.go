// catalog-plan records the real query plans on an explicitly selected test DB.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"corpord-api/internal/routes"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

type captured struct {
	SQL  string
	Args []any
}
type tracer struct {
	mu      sync.Mutex
	queries []captured
}

func (t *tracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(q.SQL, "WITH origin_legs") || strings.Contains(q.SQL, "WITH boarding AS MATERIALIZED") {
		t.mu.Lock()
		t.queries = append(t.queries, captured{q.SQL, append([]any(nil), q.Args...)})
		t.mu.Unlock()
	}
	return ctx
}
func (*tracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
func run() error {
	cfg, err := pgx.ParseConfig(os.Getenv("CATALOG_DATABASE_URL"))
	if err != nil {
		return err
	}
	if !strings.Contains(cfg.Database, "test") && !strings.Contains(cfg.Database, "demo") && !strings.Contains(cfg.Database, "catalog") {
		return fmt.Errorf("explicit test/demo database required")
	}
	cfg.RuntimeParams["default_transaction_read_only"] = "on"
	cfg.RuntimeParams["jit"] = "off"
	trace := &tracer{}
	cfg.Tracer = trace
	db := sqlx.NewDb(stdlib.OpenDB(*cfg), "pgx")
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var from, to int
	var date string
	err = db.QueryRowContext(ctx, `SELECT sa.city_id,sb.city_id,to_char(a.departure_time AT TIME ZONE 'UTC' AT TIME ZONE c.timezone,'YYYY-MM-DD')
 FROM trips t JOIN trip_stops a ON a.trip_id=t.id AND a.stop_order=1
 JOIN trip_stops b ON b.trip_id=t.id AND b.stop_order=(SELECT max(stop_order) FROM trip_stops WHERE trip_id=t.id)
 JOIN stops sa ON sa.id=a.stop_id JOIN stops sb ON sb.id=b.stop_id JOIN route_cities c ON c.id=sa.city_id
 WHERE t.is_demo AND a.departure_time>CURRENT_TIMESTAMP AT TIME ZONE 'UTC' ORDER BY t.id LIMIT 1`).Scan(&from, &to, &date)
	if err != nil {
		return err
	}
	repo := routes.NewRepository(db)
	service := routes.NewService(repo)
	if _, err = repo.Suggest(ctx, "Челябинск", fmt.Sprintf("city:%d", from), 8); err != nil {
		return err
	}
	q := routes.Search{From: fmt.Sprintf("city:%d", from), To: fmt.Sprintf("city:%d", to), Date: date, Month: date[:7], Limit: 20}
	if _, err = service.Search(ctx, q); err != nil {
		return err
	}
	if _, err = service.Calendar(ctx, q); err != nil {
		return err
	}
	trace.mu.Lock()
	queries := append([]captured(nil), trace.queries...)
	trace.mu.Unlock()
	type report struct {
		SQL  string `json:"sql"`
		Plan any    `json:"plan"`
	}
	out := []report{}
	for _, q := range queries {
		var raw []byte
		if err = db.QueryRowContext(ctx, "EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) "+q.SQL, q.Args...).Scan(&raw); err != nil {
			return err
		}
		var plan any
		if err = json.Unmarshal(raw, &plan); err != nil {
			return err
		}
		out = append(out, report{q.SQL, plan})
	}
	return json.NewEncoder(os.Stdout).Encode(out)
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Catalog plan failed; check the explicitly selected test dataset and connectivity.")
		os.Exit(1)
	}
}
