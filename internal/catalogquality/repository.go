package catalogquality

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jmoiron/sqlx"
)

type Error struct {
	Status        int
	Code, Message string
}

func (e *Error) Error() string                       { return e.Message }
func failure(status int, code, message string) error { return &Error{status, code, message} }

type Repository struct{ DB *sqlx.DB }

func New(db *sqlx.DB) *Repository { return &Repository{DB: db} }

type Stop struct {
	ID        int      `json:"id" db:"id"`
	Name      string   `json:"name" db:"name"`
	Address   *string  `json:"address" db:"address"`
	Latitude  *float64 `json:"latitude" db:"latitude"`
	Longitude *float64 `json:"longitude" db:"longitude"`
	CityID    *int     `json:"city_id" db:"city_id"`
	Timezone  *string  `json:"timezone" db:"timezone"`
	CityName  *string  `json:"city_name" db:"city_name"`
	Region    *string  `json:"region" db:"region"`
	Version   string   `json:"version" db:"version"`
	HasTrips  bool     `json:"has_trips" db:"has_trips"`
}
type Page struct {
	Items      []Stop  `json:"items"`
	NextCursor *string `json:"next_cursor"`
}
type Summary struct {
	Unmapped  int64 `json:"unmapped_stops" db:"unmapped_stops"`
	Protected int64 `json:"protected_unmapped_stops" db:"protected_unmapped_stops"`
}
type Mapping struct {
	CityID   *int    `json:"city_id"`
	Timezone *string `json:"timezone"`
}
type Audit struct {
	ID        int64           `json:"id" db:"id"`
	ActorID   int             `json:"actor_id" db:"actor_id"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
	Reason    string          `json:"reason" db:"reason"`
	Before    json.RawMessage `json:"before" db:"before_mapping"`
	After     json.RawMessage `json:"after" db:"after_mapping"`
}

const projection = `s.id,s.name,s.address,s.latitude::float8,s.longitude::float8,s.city_id,coalesce(s.timezone,c.timezone) timezone,c.name city_name,c.region,s.quality_version::text version,EXISTS(SELECT 1 FROM trip_stops ts WHERE ts.stop_id=s.id) has_trips`

func (r *Repository) Summary(ctx context.Context) (Summary, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var out Summary
	err := r.DB.GetContext(ctx, &out, `SELECT count(*) unmapped_stops,count(*) FILTER(WHERE EXISTS(SELECT 1 FROM trip_stops ts WHERE ts.stop_id=s.id)) protected_unmapped_stops FROM stops s WHERE city_id IS NULL`)
	return out, err
}
func (r *Repository) Queue(ctx context.Context, q, cursor string, limit int) (Page, error) {
	out := Page{Items: []Stop{}}
	q = strings.ReplaceAll(strings.ToLower(strings.Join(strings.Fields(q), " ")), "ё", "е")
	if limit < 1 || limit > 100 || len([]rune(q)) > 120 || len(cursor) > 512 {
		return out, failure(400, "invalid_request", "Некорректные параметры очереди")
	}
	scope := fmt.Sprintf("%x", sha256.Sum256([]byte(q)))
	after := 0
	if cursor != "" {
		b, e := base64.RawURLEncoding.DecodeString(cursor)
		var c struct {
			Scope string
			ID    int
		}
		if e != nil || json.Unmarshal(b, &c) != nil || c.Scope != scope || c.ID < 1 {
			return out, failure(400, "invalid_cursor", "Курсор не соответствует поиску")
		}
		after = c.ID
	}
	pattern := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q)
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	where := `s.city_id IS NULL AND s.id>$1`
	args := []any{after}
	if q != "" {
		args = append(args, "%"+pattern+"%")
		where += ` AND EXISTS(SELECT 1 FROM catalog_search cs WHERE cs.kind='stop' AND cs.entity_id=s.id AND cs.document LIKE $2)`
	}
	err := r.DB.SelectContext(ctx, &out.Items, `SELECT `+projection+` FROM stops s LEFT JOIN route_cities c ON c.id=s.city_id WHERE `+where+` ORDER BY s.id LIMIT `+strconv.Itoa(limit+1), args...)
	if err != nil {
		return out, err
	}
	if len(out.Items) > limit {
		out.Items = out.Items[:limit]
		b, _ := json.Marshal(struct {
			Scope string
			ID    int
		}{scope, out.Items[len(out.Items)-1].ID})
		s := base64.RawURLEncoding.EncodeToString(b)
		out.NextCursor = &s
	}
	return out, nil
}
func (r *Repository) Map(ctx context.Context, actor, id, city int, version, reason string) (Stop, error) {
	var out Stop
	reason = strings.TrimSpace(reason)
	parsed, e := strconv.ParseInt(version, 10, 64)
	if actor < 1 || id < 1 || city < 1 || e != nil || parsed < 1 || strconv.FormatInt(parsed, 10) != version || len([]rune(reason)) < 5 || len([]rune(reason)) > 500 || strings.IndexFunc(reason, unicode.IsControl) >= 0 {
		return out, failure(400, "invalid_request", "Укажите город, актуальную версию и причину от 5 до 500 символов")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := r.DB.BeginTxx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	// Import refreshes acquire city rows before stop rows. Follow the same
	// order, otherwise an import and manual mapping can deadlock each other.
	var tz *string
	if err = tx.GetContext(ctx, &tz, `SELECT timezone FROM route_cities WHERE id=$1 FOR SHARE`, city); err != nil {
		return out, notFound(err)
	}
	var old struct {
		CityID   *int    `db:"city_id"`
		Timezone *string `db:"timezone"`
		Version  string  `db:"version"`
	}
	if err = tx.GetContext(ctx, &old, `SELECT city_id,timezone,quality_version::text version FROM stops WHERE id=$1 FOR UPDATE`, id); err != nil {
		return out, notFound(err)
	}
	if old.Version != version {
		return out, failure(409, "stale_version", "Остановка изменена. Обновите очередь и повторите выбор")
	}
	var used bool
	if err = tx.GetContext(ctx, &used, `SELECT EXISTS(SELECT 1 FROM trip_stops WHERE stop_id=$1)`, id); err != nil {
		return out, err
	}
	if used {
		return out, failure(409, "stop_in_use", "Остановка используется в расписании. Привязку менять нельзя")
	}
	if old.Timezone != nil && tz != nil && *old.Timezone != *tz {
		return out, failure(409, "timezone_conflict", "Часовой пояс остановки не совпадает с выбранным городом")
	}
	if old.CityID != nil && *old.CityID == city {
		return out, failure(409, "stale_version", "Остановка уже связана с этим городом")
	}
	if _, err = tx.ExecContext(ctx, `UPDATE stops SET city_id=$1 WHERE id=$2`, city, id); err != nil {
		return out, err
	}
	before, _ := json.Marshal(Mapping{old.CityID, old.Timezone})
	after, _ := json.Marshal(Mapping{&city, old.Timezone})
	if _, err = tx.ExecContext(ctx, `INSERT INTO catalog_mapping_audit(stop_id,actor_id,reason,before_mapping,after_mapping) VALUES($1,$2,$3,$4::jsonb,$5::jsonb)`, id, actor, reason, string(before), string(after)); err != nil {
		return out, err
	}
	if err = tx.GetContext(ctx, &out, `SELECT `+projection+` FROM stops s LEFT JOIN route_cities c ON c.id=s.city_id WHERE s.id=$1`, id); err != nil {
		return out, err
	}
	return out, tx.Commit()
}
func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return failure(404, "not_found", "Остановка или город не найдены")
	}
	return err
}
func (r *Repository) History(ctx context.Context, id, limit int) ([]Audit, error) {
	out := []Audit{}
	if id < 1 || limit < 1 || limit > 100 {
		return out, failure(400, "invalid_request", "Некорректные параметры истории")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	// LEFT JOIN returns a sentinel for an existing stop with no audit events.
	rows, err := r.DB.QueryxContext(ctx, `SELECT a.id,a.actor_id,a.created_at,a.reason,a.before_mapping,a.after_mapping FROM stops s LEFT JOIN LATERAL (SELECT * FROM catalog_mapping_audit WHERE stop_id=s.id ORDER BY id DESC LIMIT $2) a ON true WHERE s.id=$1 ORDER BY a.id DESC`, id, limit)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	exists := false
	for rows.Next() {
		exists = true
		var v struct {
			ID        sql.NullInt64  `db:"id"`
			ActorID   sql.NullInt64  `db:"actor_id"`
			CreatedAt sql.NullTime   `db:"created_at"`
			Reason    sql.NullString `db:"reason"`
			Before    []byte         `db:"before_mapping"`
			After     []byte         `db:"after_mapping"`
		}
		if err = rows.StructScan(&v); err != nil {
			return out, err
		}
		if v.ID.Valid {
			out = append(out, Audit{v.ID.Int64, int(v.ActorID.Int64), v.CreatedAt.Time, v.Reason.String, v.Before, v.After})
		}
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	if !exists {
		return out, failure(404, "not_found", "Остановка не найдена")
	}
	return out, nil
}
