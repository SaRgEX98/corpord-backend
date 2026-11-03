package routes

import (
	"context"
	"strings"
	"time"
)

func literalLike(q string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q)
}

// Suggest bounds candidates BEFORE checking timetable connectivity. The two
// indexed branches never expand all locations into all possible route pairs.
func (r *Repository) Suggest(ctx context.Context, q, origin string, limit int, regions ...string) ([]Location, error) {
	if len([]rune(q)) > 120 || limit < 1 || limit > 20 {
		return nil, invalid("Некорректный запрос подсказок")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	kind, n := "", 0
	if origin != "" {
		var err error
		kind, n, err = ParseID(origin)
		if err != nil {
			return nil, err
		}
		if _, err = r.Location(ctx, origin); err != nil {
			return nil, err
		}
	}
	region := ""
	if len(regions) > 0 {
		region = regions[0]
	}
	if len([]rune(region)) > 160 {
		return nil, invalid("Некорректный регион")
	}
	q = normal(q)
	// Explicit per-kind prefix probes also work efficiently for one Cyrillic
	// character, for which a trigram-only index is ineffective.
	candidates := `WITH origin_legs AS MATERIALIZED (
 SELECT a.trip_id,a.stop_order FROM stops sa
 CROSS JOIN LATERAL (SELECT ts.trip_id,ts.stop_order FROM trip_stops ts
 WHERE ts.stop_id=sa.id AND ts.departure_time>CURRENT_TIMESTAMP AT TIME ZONE 'UTC' OFFSET 0) a
 CROSS JOIN LATERAL (SELECT 1 FROM trips t WHERE t.id=a.trip_id AND t.status IN ('scheduled','boarding','delayed') OFFSET 0) active
 WHERE $3<>'' AND (($3='stop' AND sa.id=$4) OR ($3='city' AND sa.city_id=$4))
 ), reachable AS MATERIALIZED (
 SELECT DISTINCT b.stop_id,sb.city_id FROM origin_legs a
 CROSS JOIN LATERAL (SELECT ts.stop_id FROM trip_stops ts WHERE ts.trip_id=a.trip_id AND ts.stop_order>a.stop_order OFFSET 0) b
 CROSS JOIN LATERAL (SELECT s.city_id FROM stops s WHERE s.id=b.stop_id OFFSET 0) sb
 ), prefix AS MATERIALIZED (
 (SELECT * FROM catalog_search WHERE kind='city' AND ($7='' OR region=$7) AND name_norm LIKE $1 ORDER BY name_norm,entity_id LIMIT 64)
 UNION ALL
 (SELECT * FROM catalog_search WHERE kind='stop' AND ($7='' OR region=$7) AND name_norm LIKE $1 ORDER BY name_norm,entity_id LIMIT 64)
 ), fuzzy AS MATERIALIZED (
 SELECT * FROM catalog_search WHERE kind IN ('city','stop') AND ($7='' OR region=$7) AND length($2)>=3
 AND (SELECT count(*) FROM prefix)<$5 ORDER BY name_norm <-> $2 LIMIT 64
 ), address_matches AS MATERIALIZED (
 SELECT * FROM catalog_search WHERE kind IN ('city','stop') AND ($7='' OR region=$7) AND length($2)>=3 AND document LIKE $6
 AND (SELECT count(*) FROM prefix)<$5 LIMIT 64
 ), candidates AS MATERIALIZED (
 SELECT DISTINCT ON(kind,entity_id) * FROM (SELECT * FROM prefix UNION ALL
 SELECT * FROM fuzzy WHERE similarity(name_norm,$2)>=0.25 UNION ALL SELECT * FROM address_matches) c
 ), enriched AS (
 SELECT cs.*,` + locationServiceSQL + ` AS served,
 CASE WHEN $3='' THEN false ELSE EXISTS(SELECT 1 FROM reachable rr
 WHERE (cs.kind='stop' AND rr.stop_id=cs.entity_id) OR (cs.kind='city' AND rr.city_id=cs.entity_id)) END AS connected
 FROM candidates cs)
 SELECT kind||':'||entity_id AS id,kind AS type,name,region,timezone,served,
 CASE WHEN $2<>'' THEN 'match' WHEN connected THEN 'connected' ELSE 'popular' END AS rank_reason,address,city_name,latitude,longitude
 FROM enriched WHERE $2<>'' OR served OR $7<>''
 ORDER BY (name_norm=$2) DESC,(name_norm LIKE $1) DESC,connected DESC,served DESC,popularity DESC,kind,name_norm,entity_id LIMIT $5`
	if q == "" {
		// Empty input uses a bounded popularity index, then verifies current service.
		candidates = strings.Replace(candidates, `name_norm LIKE $1 ORDER BY name_norm,entity_id LIMIT 64`, `true ORDER BY popularity DESC,entity_id LIMIT 64`, 2)
	}
	out := []Location{}
	err := r.DB.SelectContext(ctx, &out, candidates, literalLike(q)+"%", q, kind, n, limit, "%"+literalLike(q)+"%", region)
	return out, err
}
