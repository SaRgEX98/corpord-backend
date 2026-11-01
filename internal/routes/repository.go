package routes

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/jmoiron/sqlx"
	"strings"
	"time"
)

type Repository struct{ DB *sqlx.DB }

func NewRepository(db *sqlx.DB) *Repository { return &Repository{db} }
func (r *Repository) Location(ctx context.Context, id string) (Location, error) {
	kind, n, err := ParseID(id)
	if err != nil {
		return Location{}, err
	}
	var loc Location
	err = r.DB.GetContext(ctx, &loc, locationSQL+` WHERE id=$1`, fmt.Sprintf("%s:%d", kind, n))
	if err == sql.ErrNoRows {
		return loc, missing()
	}
	return loc, err
}

const locationSQL = `SELECT * FROM (
 SELECT 'city:'||c.id AS id,'city' AS type,c.name,c.region,c.timezone,
 EXISTS(SELECT 1 FROM stops s JOIN trip_stops ts ON ts.stop_id=s.id JOIN trips t ON t.id=ts.trip_id WHERE s.city_id=c.id AND t.status IN ('scheduled','boarding','delayed') AND ts.departure_time>CURRENT_TIMESTAMP AT TIME ZONE 'UTC') AS served,
 'match' AS rank_reason,'' AS address,NULL::float8 AS latitude,NULL::float8 AS longitude FROM route_cities c
 UNION ALL
 SELECT 'stop:'||s.id,'stop',s.name,COALESCE(c.region,''),COALESCE(s.timezone,c.timezone),
 EXISTS(SELECT 1 FROM trip_stops ts JOIN trips t ON t.id=ts.trip_id WHERE ts.stop_id=s.id AND t.status IN ('scheduled','boarding','delayed') AND ts.departure_time>CURRENT_TIMESTAMP AT TIME ZONE 'UTC'),
 'match',COALESCE(s.address,''),s.latitude::float8,s.longitude::float8 FROM stops s LEFT JOIN route_cities c ON c.id=s.city_id
) locations`

func normal(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), "ё", "е")
}
func (r *Repository) Suggest(ctx context.Context, q, origin string, limit int) ([]Location, error) {
	if len([]rune(q)) > 120 || limit < 1 || limit > 20 {
		return nil, invalid("Некорректный запрос подсказок")
	}
	ot, on := "", 0
	if origin != "" {
		var err error
		ot, on, err = ParseID(origin)
		if err != nil {
			return nil, err
		}
		if _, err = r.Location(ctx, origin); err != nil {
			return nil, err
		}
	}
	// strpos treats % and _ literally; all user input remains SQL parameters.
	query := `WITH loc AS (` + locationSQL + `), ranked AS (
 SELECT loc.*,CASE WHEN $1='' THEN 0 WHEN replace(lower(name),'ё','е')=$1 THEN 3 WHEN strpos(replace(lower(name),'ё','е'),$1)=1 THEN 2 ELSE 1 END AS relevance,
 CASE WHEN $2='' THEN false ELSE EXISTS(
 SELECT 1 FROM trip_stops a JOIN stops sa ON sa.id=a.stop_id JOIN trip_stops b ON b.trip_id=a.trip_id AND b.stop_order>a.stop_order JOIN stops sb ON sb.id=b.stop_id JOIN trips t ON t.id=a.trip_id
 WHERE t.status IN ('scheduled','boarding','delayed') AND a.departure_time>CURRENT_TIMESTAMP AT TIME ZONE 'UTC'
 AND (($2='stop' AND sa.id=$3) OR ($2='city' AND sa.city_id=$3))
 AND ((loc.type='stop' AND loc.id='stop:'||sb.id) OR (loc.type='city' AND loc.id='city:'||sb.city_id))) END AS connected,
 COALESCE((SELECT popularity FROM route_cities c WHERE loc.id='city:'||c.id),0) AS popularity
 FROM loc WHERE ($1='' AND served) OR ($1<>'' AND (strpos(replace(lower(name),'ё','е'),$1)>0 OR strpos(replace(lower(address),'ё','е'),$1)>0)))
 SELECT id,type,name,region,timezone,served,CASE WHEN relevance>0 THEN 'match' WHEN connected THEN 'connected' ELSE 'popular' END AS rank_reason,address,latitude,longitude
 FROM ranked ORDER BY relevance DESC,connected DESC,served DESC,popularity DESC,name,id LIMIT $4`
	out := []Location{}
	err := r.DB.SelectContext(ctx, &out, query, normal(q), ot, on, limit)
	return out, err
}
func (r *Repository) Stops(ctx context.Context, id string) ([]Location, error) {
	if _, err := r.Location(ctx, id); err != nil {
		return nil, err
	}
	kind, n, _ := ParseID(id)
	out := []Location{}
	err := r.DB.SelectContext(ctx, &out, locationSQL+` WHERE type='stop' AND (($1='stop' AND id='stop:'||($2::integer)::text) OR ($1='city' AND id IN (SELECT 'stop:'||id FROM stops WHERE city_id=$2::integer))) ORDER BY name,id`, kind, n)
	return out, err
}
func (r *Repository) Cities(ctx context.Context) ([]City, error) {
	out := []City{}
	err := r.DB.SelectContext(ctx, &out, `SELECT id,name,region,timezone,popularity FROM route_cities ORDER BY name,id`)
	return out, err
}
func (r *Repository) SaveCity(ctx context.Context, c *City) error {
	c.Name = strings.TrimSpace(c.Name)
	c.Region = strings.TrimSpace(c.Region)
	if err := c.Validate(); err != nil {
		return err
	}
	if c.ID == 0 {
		return r.DB.QueryRowxContext(ctx, `INSERT INTO route_cities(name,region,timezone,popularity) VALUES($1,$2,$3,$4) RETURNING id`, c.Name, c.Region, c.Timezone, c.Popularity).Scan(&c.ID)
	}
	// Existing mapped city timezone is deliberately immutable: changing it would re-date live schedules.
	var timezone string
	if err := r.DB.GetContext(ctx, &timezone, `SELECT timezone FROM route_cities WHERE id=$1`, c.ID); err == sql.ErrNoRows {
		return missing()
	} else if err != nil {
		return err
	}
	if timezone != c.Timezone {
		return conflict("timezone_immutable", "Создайте новый город для другого часового пояса")
	}
	_, err := r.DB.ExecContext(ctx, `UPDATE route_cities SET name=$1,region=$2,popularity=$3 WHERE id=$4`, c.Name, c.Region, c.Popularity, c.ID)
	return err
}

const segmentSQL = `SELECT t.id AS trip_id,a.id AS boarding_relation,b.id AS alighting_relation,a.stop_id AS boarding_stop_id,b.stop_id AS alighting_stop_id,
 sa.name AS boarding_name,sb.name AS alighting_name,
 a.departure_time AT TIME ZONE 'UTC' AS departure_time,b.arrival_time AT TIME ZONE 'UTC' AS arrival_time,
 COALESCE(sa.timezone,ca.timezone,'') AS timezone,COALESCE(sb.timezone,cb.timezone,'') AS arrival_timezone,t.status,
 CASE WHEN COALESCE(sa.timezone,ca.timezone,'')<>'' AND COALESCE(sb.timezone,cb.timezone,'')<>'' THEN fare.amount_minor END AS amount_minor
 FROM trips t JOIN trip_stops a ON a.trip_id=t.id JOIN trip_stops b ON b.trip_id=t.id AND b.stop_order>a.stop_order
 JOIN stops sa ON sa.id=a.stop_id JOIN stops sb ON sb.id=b.stop_id
 LEFT JOIN route_cities ca ON ca.id=sa.city_id LEFT JOIN route_cities cb ON cb.id=sb.city_id
 CROSS JOIN LATERAL (SELECT CASE WHEN count(*)=count(price_to_next_minor) AND min(price_to_next_minor)>=0 THEN sum(price_to_next_minor)::bigint END AS amount_minor FROM trip_stops leg WHERE leg.trip_id=t.id AND leg.stop_order>=a.stop_order AND leg.stop_order<b.stop_order) fare
 WHERE (($1='stop' AND sa.id=$2) OR ($1='city' AND sa.city_id=$2))
 AND (($3='stop' AND sb.id=$4) OR ($3='city' AND sb.city_id=$4))
 AND b.arrival_time>a.departure_time`

func (r *Repository) OriginTimezone(ctx context.Context, id string) (string, error) {
	loc, err := r.Location(ctx, id)
	if err != nil {
		return "", err
	}
	if loc.Timezone == nil || !ValidTimezone(*loc.Timezone) {
		return "", conflict("timezone_required", "Укажите город или часовой пояс остановки в справочнике")
	}
	return *loc.Timezone, nil
}

const unambiguousSQL = ` AND 1=(SELECT count(*) FROM trip_stops la JOIN trip_stops lb ON lb.trip_id=la.trip_id AND lb.stop_order>la.stop_order WHERE la.trip_id=t.id AND la.stop_id=a.stop_id AND lb.stop_id=b.stop_id)`

func (r *Repository) segments(ctx context.Context, q Search, start, end, now time.Time, after *cursor) ([]Segment, error) {
	ft, fn, _ := ParseID(q.From)
	tt, tn, _ := ParseID(q.To)
	query := segmentSQL + unambiguousSQL + ` AND t.status IN ('scheduled','boarding','delayed') AND a.departure_time>=($5::timestamptz AT TIME ZONE 'UTC') AND a.departure_time<($6::timestamptz AT TIME ZONE 'UTC') AND a.departure_time>($7::timestamptz AT TIME ZONE 'UTC')`
	args := []any{ft, fn, tt, tn, start, end, now}
	if after != nil {
		if q.Sort == "price" {
			query += ` AND (COALESCE(CASE WHEN COALESCE(sa.timezone,ca.timezone,'')<>'' AND COALESCE(sb.timezone,cb.timezone,'')<>'' THEN fare.amount_minor END,9223372036854775807),a.departure_time,t.id,a.id,b.id)>($8,($9::timestamptz AT TIME ZONE 'UTC'),$10,$11,$12)`
			args = append(args, after.Amount, after.Departure, after.Trip, after.BoardingRelation, after.AlightingRelation)
		} else {
			query += ` AND (a.departure_time,t.id,a.id,b.id)>(($8::timestamptz AT TIME ZONE 'UTC'),$9,$10,$11)`
			args = append(args, after.Departure, after.Trip, after.BoardingRelation, after.AlightingRelation)
		}
	}
	if q.Sort == "price" {
		query += ` ORDER BY amount_minor NULLS LAST,a.departure_time,t.id,a.id,b.id`
	} else {
		query += ` ORDER BY a.departure_time,t.id,a.id,b.id`
	}
	if q.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", q.Limit+1)
	}
	out := []Segment{}
	err := r.DB.SelectContext(ctx, &out, query, args...)
	return out, err
}
func (r *Repository) quote(ctx context.Context, trip int, b, a int) (Segment, error) {
	var out Segment
	// Stop IDs may occur twice on a loop. Require an unambiguous relation pair.
	items := []Segment{}
	err := r.DB.SelectContext(ctx, &items, segmentSQL+` AND t.id=$5 ORDER BY a.stop_order,b.stop_order LIMIT 2`, "stop", b, "stop", a, trip)
	if err != nil {
		return out, err
	}
	if len(items) == 0 {
		return out, missing()
	}
	if len(items) > 1 {
		return out, conflict("ambiguous_segment", "Участок неоднозначен")
	}
	return items[0], nil
}
