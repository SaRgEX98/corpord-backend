package routes

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

type Service struct {
	Repo *Repository
	Now  func() time.Time
}

func NewService(repo *Repository) *Service { return &Service{repo, time.Now} }

type cursor struct {
	Scope                                     string
	Departure                                 time.Time
	Trip, BoardingRelation, AlightingRelation int
	Amount                                    int64
}

func scope(q Search) string {
	s := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s|%d", q.From, q.To, q.Date, q.Sort, q.Passengers)))
	return fmt.Sprintf("%x", s[:])
}
func decodeCursor(q Search) (*cursor, error) {
	if q.Cursor == "" {
		return nil, nil
	}
	if len(q.Cursor) > 2048 {
		return nil, invalid("Некорректный cursor")
	}
	b, err := base64.RawURLEncoding.DecodeString(q.Cursor)
	if err != nil {
		return nil, invalid("Некорректный cursor")
	}
	var c cursor
	if json.Unmarshal(b, &c) != nil || c.Scope != scope(q) || c.Departure.IsZero() || c.Trip < 1 || c.BoardingRelation < 1 || c.AlightingRelation < 1 {
		return nil, invalid("Cursor не соответствует поиску")
	}
	return &c, nil
}
func encodeCursor(q Search, s Segment) *string {
	amount := int64(9223372036854775807)
	if s.AmountMinor != nil {
		amount = *s.AmountMinor
	}
	b, _ := json.Marshal(cursor{scope(q), s.DepartureTime, s.TripID, s.BoardingRelation, s.AlightingRelation, amount})
	v := base64.RawURLEncoding.EncodeToString(b)
	return &v
}
func (s *Service) bounds(ctx context.Context, q Search, calendar bool) (string, time.Time, time.Time, error) {
	tz, err := s.Repo.OriginTimezone(ctx, q.From)
	if err != nil {
		return "", time.Time{}, time.Time{}, err
	}
	if _, err = s.Repo.Location(ctx, q.To); err != nil {
		return "", time.Time{}, time.Time{}, err
	}
	zone, _ := time.LoadLocation(tz)
	layout, val := "2006-01-02", q.Date
	if calendar {
		layout, val = "2006-01", q.Month
	}
	start, err := time.ParseInLocation(layout, val, zone)
	if err != nil {
		return "", start, start, invalid("Некорректная дата")
	}
	end := start.AddDate(0, 0, 1)
	if calendar {
		end = start.AddDate(0, 1, 0)
	}
	return tz, start, end, nil
}
func (s *Service) Search(ctx context.Context, q Search) (SearchResult, error) {
	out := SearchResult{Items: []Segment{}}
	now := s.Now().UTC()
	if err := q.Validate(false, now); err != nil {
		return out, err
	}
	after, err := decodeCursor(q)
	if err != nil {
		return out, err
	}
	_, start, end, err := s.bounds(ctx, q, false)
	if err != nil {
		return out, err
	}
	items, err := s.Repo.segments(ctx, q, start, end, now, after)
	if err != nil {
		return out, err
	}
	if len(items) > q.Limit {
		items = items[:q.Limit]
		out.NextCursor = encodeCursor(q, items[len(items)-1])
	}
	for i := range items {
		if err := items[i].decorate(now); err != nil {
			return out, err
		}
	}
	out.Items = items
	return out, nil
}
func (s *Service) Calendar(ctx context.Context, q Search) (Calendar, error) {
	now := s.Now().UTC()
	out := Calendar{Month: q.Month, CalculatedAt: now, RuleVersion: RuleVersion, Days: []CalendarDay{}}
	if err := q.Validate(true, now); err != nil {
		return out, err
	}
	tz, start, end, err := s.bounds(ctx, q, true)
	if err != nil {
		return out, err
	}
	out.Timezone = tz
	ft, fn, _ := ParseID(q.From)
	tt, tn, _ := ParseID(q.To)
	type daily struct {
		Date  string `db:"date"`
		Count int    `db:"count"`
		Min   *int64 `db:"min"`
	}
	rows := []daily{}
	query := `SELECT to_char(departure_time AT TIME ZONE $8,'YYYY-MM-DD') AS date,count(DISTINCT trip_id) AS count,min(amount_minor) AS min FROM (` + segmentSQL + unambiguousSQL + ` AND t.status IN ('scheduled','boarding','delayed') AND a.departure_time>=($5::timestamptz AT TIME ZONE 'UTC') AND a.departure_time<($6::timestamptz AT TIME ZONE 'UTC') AND a.departure_time>($7::timestamptz AT TIME ZONE 'UTC')) segments GROUP BY 1`
	if err = s.Repo.DB.SelectContext(ctx, &rows, query, ft, fn, tt, tn, start, end, now, tz); err != nil {
		return out, err
	}
	byDate := map[string]daily{}
	for _, r := range rows {
		byDate[r.Date] = r
	}
	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		date := d.Format("2006-01-02")
		r := byDate[date]
		status := "no_trips"
		if r.Count > 0 {
			status = "unknown"
		}
		out.Days = append(out.Days, CalendarDay{date, status, r.Min, r.Count})
	}
	return out, nil
}
func (s *Service) Quote(ctx context.Context, id int, q QuoteRequest) (Quote, error) {
	var out Quote
	if id < 1 || q.BoardingStopID < 1 || q.AlightingStopID < 1 || q.BoardingStopID == q.AlightingStopID || (q.Passengers != 0 && q.Passengers != 1) {
		return out, invalid("Некорректный участок или число пассажиров")
	}
	seg, err := s.Repo.quote(ctx, id, q.BoardingStopID, q.AlightingStopID)
	if err != nil {
		return out, err
	}
	now := s.Now().UTC()
	if (seg.Status != "scheduled" && seg.Status != "boarding" && seg.Status != "delayed") || !seg.DepartureTime.After(now) {
		return out, conflict("trip_unavailable", "Рейс недоступен для поиска")
	}
	if err = seg.decorate(now); err != nil {
		return out, err
	}
	out.Segment = seg
	return out, nil
}
