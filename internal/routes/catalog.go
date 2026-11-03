package routes

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

type PageQuery struct {
	Kind, Q, Cursor, Region, Status, Category string
	CityID, Limit                             int
}
type Page struct {
	Items      []json.RawMessage `json:"items"`
	NextCursor *string           `json:"next_cursor"`
}
type pageCursor struct {
	Scope string
	ID    int
}
type catalogSpec struct{ From, Payload, Label, Where string }

var catalogSpecs = map[string]catalogSpec{
	"cities":  {`route_cities x`, `to_jsonb(x)-'source_snapshot'`, `x.name||' '||x.region`, `true`},
	"regions": {`route_regions x`, `to_jsonb(x)`, `x.name`, `EXISTS(SELECT 1 FROM route_cities c WHERE c.region=x.name)`},
	"stops":   {`stops x LEFT JOIN route_cities c ON c.id=x.city_id`, `to_jsonb(x)-'source_snapshot'||jsonb_build_object('effective_timezone',coalesce(x.timezone,c.timezone))`, `x.name||' '||coalesce(x.address,'')`, `true`},
	"buses":   {`bus x JOIN bus_categories c ON c.id=x.category_id JOIN bus_statuses s ON s.id=x.status_id`, `to_jsonb(x)||jsonb_build_object('category',c.name,'status',s.name)`, `x.brand||' '||x.license_plate`, `x.deleted_at IS NULL`},
	"drivers": {`drivers x JOIN driver_status s ON s.id=x.status`, `to_jsonb(x)||jsonb_build_object('status',s.name)`, `x.last_name||' '||x.first_name||' '||x.phone_number`, `true`},
	"trips": {`trips x JOIN bus b ON b.id=x.bus_id JOIN drivers d ON d.id=x.driver_id
 LEFT JOIN LATERAL (SELECT s.name FROM trip_stops ts JOIN stops s ON s.id=ts.stop_id WHERE ts.trip_id=x.id ORDER BY stop_order LIMIT 1) first_stop ON true
 LEFT JOIN LATERAL (SELECT s.name FROM trip_stops ts JOIN stops s ON s.id=ts.stop_id WHERE ts.trip_id=x.id ORDER BY stop_order DESC LIMIT 1) last_stop ON true`,
		`to_jsonb(x)||jsonb_build_object('start_time',to_char(x.start_time,'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),'end_time',to_char(x.end_time,'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),'bus',jsonb_build_object('id',b.id,'brand',b.brand,'license_plate',b.license_plate),'driver',jsonb_build_object('id',d.id,'first_name',d.first_name,'last_name',d.last_name),'route',coalesce(first_stop.name,'')||' → '||coalesce(last_stop.name,''),'stops','[]'::jsonb)`, `x.status`, `true`},
	"tripStops":      {`trip_stops x`, `to_jsonb(x)||jsonb_build_object('arrival_time',to_char(x.arrival_time,'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),'departure_time',to_char(x.departure_time,'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'))`, `x.id::text`, `true`},
	"users":          {`users x JOIN roles r ON r.id=x.role_id`, `jsonb_build_object('id',x.id,'name',x.name,'email',x.email,'role',r.name,'created_at',x.created_at,'updated_at',x.updated_at)`, `x.name||' '||x.email`, `x.deleted_at IS NULL`},
	"busCategories":  {`bus_categories x`, `to_jsonb(x)`, `x.name`, `true`},
	"busStatuses":    {`bus_statuses x`, `to_jsonb(x)`, `x.name`, `true`},
	"driverStatuses": {`driver_status x`, `to_jsonb(x)`, `x.name`, `true`},
}

// Catalog executes one SQL statement per page. Relations are joined and only
// endpoint names are read for trip summaries; complete stops belong to detail.
func (r *Repository) Catalog(ctx context.Context, q PageQuery) (Page, error) {
	out := Page{Items: []json.RawMessage{}}
	spec, ok := catalogSpecs[q.Kind]
	if !ok || q.Limit < 1 || q.Limit > 100 || len([]rune(q.Q)) > 120 || len(q.Cursor) > 1024 || q.CityID < 0 || len([]rune(q.Region)) > 160 || len([]rune(q.Status)) > 160 || len([]rune(q.Category)) > 160 {
		return out, invalid("Некорректные параметры каталога")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	q.Q = normal(q.Q)
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%d|%s|%s", q.Kind, q.Q, q.Region, q.CityID, q.Status, q.Category)))
	scope := fmt.Sprintf("%x", hash[:])
	after := 0
	if q.Cursor != "" {
		b, e := base64.RawURLEncoding.DecodeString(q.Cursor)
		if e != nil {
			return out, invalid("Некорректный cursor")
		}
		var c pageCursor
		if json.Unmarshal(b, &c) != nil || c.Scope != scope || c.ID < 1 {
			return out, invalid("Cursor не соответствует каталогу")
		}
		after = c.ID
	}
	where := spec.Where + ` AND x.id>$1`
	args := []any{after}
	add := func(value any) string { args = append(args, value); return "$" + strconv.Itoa(len(args)) }
	if q.Q != "" {
		if id, e := strconv.Atoi(q.Q); e == nil && id > 0 {
			where += ` AND x.id=` + add(id)
		} else if q.Kind == "cities" || q.Kind == "stops" || q.Kind == "regions" {
			kind := map[string]string{"cities": "city", "stops": "stop", "regions": "region"}[q.Kind]
			pattern := literalLike(q.Q) + "%"
			predicate := `cs.name_norm LIKE `
			if len([]rune(q.Q)) >= 3 {
				pattern = "%" + literalLike(q.Q) + "%"
				predicate = `cs.document LIKE `
			}
			where += ` AND x.id IN (SELECT entity_id FROM catalog_search cs WHERE kind=` + add(kind) + ` AND ` + predicate + add(pattern) + `)`
		} else {
			where += ` AND catalog_normalize(` + spec.Label + `) LIKE ` + add("%"+literalLike(q.Q)+"%")
		}
	}
	if q.Kind == "cities" && q.Region != "" {
		where += ` AND x.region=` + add(q.Region)
	}
	if q.Kind == "stops" && q.CityID > 0 {
		where += ` AND x.city_id=` + add(q.CityID)
	}
	if q.Status != "" && (q.Kind == "buses" || q.Kind == "drivers") {
		where += ` AND s.name=` + add(q.Status)
	}
	if q.Category != "" && q.Kind == "buses" {
		where += ` AND c.name=` + add(q.Category)
	}
	query := `SELECT x.id,` + spec.Payload + ` AS payload FROM ` + spec.From + ` WHERE ` + where + ` ORDER BY x.id LIMIT ` + strconv.Itoa(q.Limit+1)
	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	last := 0
	for rows.Next() {
		var id int
		var payload []byte
		if err = rows.Scan(&id, &payload); err != nil {
			return out, err
		}
		if len(out.Items) == q.Limit {
			b, _ := json.Marshal(pageCursor{scope, last})
			v := base64.RawURLEncoding.EncodeToString(b)
			out.NextCursor = &v
			break
		}
		out.Items = append(out.Items, json.RawMessage(payload))
		last = id
	}
	return out, rows.Err()
}
