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
	err = r.DB.GetContext(ctx, &loc, locationSQL+` WHERE kind=$1 AND entity_id=$2`, kind, n)
	if err == sql.ErrNoRows {
		return loc, missing()
	}
	return loc, err
}

// OFFSET 0 keeps each stop's selective departure-index probe ahead of the
// trip join. PostgreSQL otherwise misestimates city->served-stop correlation
// and can scan the entire national timetable even for one city.
const locationServiceSQL = `CASE WHEN cs.kind='city' THEN EXISTS(
 SELECT 1 FROM (SELECT id FROM stops WHERE city_id=cs.entity_id ORDER BY id OFFSET 0) s
 CROSS JOIN LATERAL (SELECT ts.trip_id FROM trip_stops ts WHERE ts.stop_id=s.id
 AND ts.departure_time>CURRENT_TIMESTAMP AT TIME ZONE 'UTC' OFFSET 0) leg
 CROSS JOIN LATERAL (SELECT 1 FROM trips t WHERE t.id=leg.trip_id AND t.status IN ('scheduled','boarding','delayed') OFFSET 0) active
 )
 ELSE EXISTS(SELECT 1 FROM trip_stops ts
 CROSS JOIN LATERAL (SELECT 1 FROM trips t WHERE t.id=ts.trip_id AND t.status IN ('scheduled','boarding','delayed') OFFSET 0) active
 WHERE ts.stop_id=cs.entity_id AND ts.departure_time>CURRENT_TIMESTAMP AT TIME ZONE 'UTC') END`
const locationSQL = `SELECT kind||':'||entity_id AS id,kind AS type,name,region,timezone,
 ` + locationServiceSQL + ` AS served,
 'match' AS rank_reason,address,city_name,latitude,longitude FROM catalog_search cs`

func normal(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), "ё", "е")
}

func (r *Repository) Stops(ctx context.Context, id string) ([]Location, error) {
	if _, err := r.Location(ctx, id); err != nil {
		return nil, err
	}
	kind, n, _ := ParseID(id)
	out := []Location{}
	err := r.DB.SelectContext(ctx, &out, locationSQL+` WHERE kind='stop' AND (($1='stop' AND entity_id=$2) OR ($1='city' AND city_id=$2)) ORDER BY name_norm,entity_id LIMIT 50`, kind, n)
	return out, err
}
func (r *Repository) Cities(ctx context.Context) ([]City, error) {
	out := []City{}
	err := r.DB.SelectContext(ctx, &out, `SELECT id,name,region,coalesce(timezone,'') AS timezone,popularity FROM route_cities ORDER BY name,id LIMIT 100`)
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
	var timezone sql.NullString
	if err := r.DB.GetContext(ctx, &timezone, `SELECT timezone FROM route_cities WHERE id=$1`, c.ID); err == sql.ErrNoRows {
		return missing()
	} else if err != nil {
		return err
	}
	if timezone.Valid && timezone.String != c.Timezone {
		return conflict("timezone_immutable", "Создайте новый город для другого часового пояса")
	}
	result, err := r.DB.ExecContext(ctx, `UPDATE route_cities SET name=$1,region=$2,popularity=$3,timezone=coalesce(timezone,$5) WHERE id=$4 AND (timezone IS NULL OR timezone=$5)`, c.Name, c.Region, c.Popularity, c.ID, c.Timezone)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return conflict("timezone_immutable", "Часовой пояс уже изменён. Обновите карточку")
	}
	return nil
}

const segmentSQL = `SELECT t.is_demo,t.id AS trip_id,a.id AS boarding_relation,b.id AS alighting_relation,a.stop_id AS boarding_stop_id,b.stop_id AS alighting_stop_id,
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

// The country-wide distribution is highly skewed: most physical stops have no
// timetable. Keep stop membership and time bounds before route expansion rather
// than relying on cross-table correlation estimates to infer this order.
func datedSegmentSQL() string {
	const boarding = `WITH boarding AS MATERIALIZED (
 SELECT a.* FROM stops origin
 CROSS JOIN LATERAL (SELECT ts.* FROM trip_stops ts WHERE ts.stop_id=origin.id
 AND ts.departure_time>=($5::timestamptz AT TIME ZONE 'UTC')
 AND ts.departure_time<($6::timestamptz AT TIME ZONE 'UTC')
 AND ts.departure_time>($7::timestamptz AT TIME ZONE 'UTC') OFFSET 0) a
 WHERE ($1='stop' AND origin.id=$2) OR ($1='city' AND origin.city_id=$2)
 ) `
	query := strings.Replace(segmentSQL,
		`FROM trips t JOIN trip_stops a ON a.trip_id=t.id JOIN trip_stops b ON b.trip_id=t.id AND b.stop_order>a.stop_order`,
		`FROM boarding a
 CROSS JOIN LATERAL (SELECT trip.* FROM trips trip WHERE trip.id=a.trip_id OFFSET 0) t
 CROSS JOIN LATERAL (SELECT rel.* FROM trip_stops rel WHERE rel.trip_id=a.trip_id AND rel.stop_order>a.stop_order OFFSET 0) b`, 1)
	query = strings.Replace(query, `JOIN stops sa ON sa.id=a.stop_id JOIN stops sb ON sb.id=b.stop_id`,
		`CROSS JOIN LATERAL (SELECT s.* FROM stops s WHERE s.id=a.stop_id OFFSET 0) sa
 CROSS JOIN LATERAL (SELECT s.* FROM stops s WHERE s.id=b.stop_id OFFSET 0) sb`, 1)
	return boarding + query
}

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
	query := datedSegmentSQL() + unambiguousSQL + ` AND t.status IN ('scheduled','boarding','delayed') AND a.departure_time>=($5::timestamptz AT TIME ZONE 'UTC') AND a.departure_time<($6::timestamptz AT TIME ZONE 'UTC') AND a.departure_time>($7::timestamptz AT TIME ZONE 'UTC')`
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
