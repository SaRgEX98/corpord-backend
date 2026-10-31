// Package routes implements the informational route search. It never reserves seats.
package routes

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

const RuleVersion = "segment-v1"

type Error struct {
	Status        int
	Code, Message string
}

func (e *Error) Error() string     { return e.Message }
func invalid(message string) error { return &Error{400, "invalid_request", message} }
func missing() error {
	return &Error{404, "not_found", "Точка или рейс не найдены"}
}
func conflict(code, message string) error { return &Error{409, code, message} }

type Location struct {
	ID         string   `json:"id" db:"id"`
	Type       string   `json:"type" db:"type"`
	Name       string   `json:"name" db:"name"`
	Region     string   `json:"region" db:"region"`
	Timezone   *string  `json:"timezone" db:"timezone"`
	Served     bool     `json:"served" db:"served"`
	RankReason string   `json:"rank_reason" db:"rank_reason"`
	Address    string   `json:"address" db:"address"`
	Latitude   *float64 `json:"latitude" db:"latitude"`
	Longitude  *float64 `json:"longitude" db:"longitude"`
}
type City struct {
	ID         int    `json:"id" db:"id"`
	Name       string `json:"name" db:"name"`
	Region     string `json:"region" db:"region"`
	Timezone   string `json:"timezone" db:"timezone"`
	Popularity int    `json:"popularity" db:"popularity"`
}

func (c City) Validate() error {
	if strings.TrimSpace(c.Name) == "" || strings.TrimSpace(c.Region) == "" || len([]rune(c.Name)) > 160 || len([]rune(c.Region)) > 160 || c.Popularity < 0 {
		return invalid("Нужны название, регион и неотрицательный приоритет")
	}
	if !ValidTimezone(c.Timezone) {
		return invalid("Нужен часовой пояс IANA")
	}
	return nil
}
func ValidTimezone(tz string) bool {
	if tz == "" || tz == "Local" {
		return false
	}
	_, err := time.LoadLocation(tz)
	return err == nil
}
func ParseID(value string) (string, int, error) {
	p := strings.Split(value, ":")
	if len(p) != 2 || (p[0] != "city" && p[0] != "stop") {
		return "", 0, invalid("Нужен ID city:N или stop:N")
	}
	n, err := strconv.Atoi(p[1])
	if err != nil || n < 1 {
		return "", 0, invalid("Некорректный ID точки")
	}
	return p[0], n, nil
}

type Money struct {
	AmountMinor  int64     `json:"amount_minor"`
	Currency     string    `json:"currency"`
	RuleVersion  string    `json:"rule_version"`
	CalculatedAt time.Time `json:"calculated_at"`
}
type Availability struct {
	Status string `json:"status"`
	Seats  *int   `json:"seats"`
}
type Segment struct {
	TripID            int          `json:"trip_id" db:"trip_id"`
	BoardingRelation  int          `json:"-" db:"boarding_relation"`
	AlightingRelation int          `json:"-" db:"alighting_relation"`
	BoardingStopID    int          `json:"boarding_stop_id" db:"boarding_stop_id"`
	AlightingStopID   int          `json:"alighting_stop_id" db:"alighting_stop_id"`
	BoardingName      string       `json:"boarding_name" db:"boarding_name"`
	AlightingName     string       `json:"alighting_name" db:"alighting_name"`
	DepartureTime     time.Time    `json:"departure_time" db:"departure_time"`
	ArrivalTime       time.Time    `json:"arrival_time" db:"arrival_time"`
	Timezone          string       `json:"timezone" db:"timezone"`
	ArrivalTimezone   string       `json:"arrival_timezone" db:"arrival_timezone"`
	Status            string       `json:"status" db:"status"`
	Price             *Money       `json:"price" db:"-"`
	Availability      Availability `json:"availability" db:"-"`
	AmountMinor       *int64       `json:"-" db:"amount_minor"`
}

func (s *Segment) decorate(now time.Time) error {
	s.Availability = Availability{Status: "unknown"}
	zone, err := time.LoadLocation(s.Timezone)
	if err != nil || s.Timezone == "" {
		return conflict("timezone_required", "Укажите часовой пояс остановки")
	}
	s.DepartureTime = s.DepartureTime.In(zone)
	arrival, err := time.LoadLocation(s.ArrivalTimezone)
	if err == nil && s.ArrivalTimezone != "" {
		s.ArrivalTime = s.ArrivalTime.In(arrival)
	}
	if s.AmountMinor != nil && *s.AmountMinor >= 0 && err == nil && s.ArrivalTimezone != "" {
		s.Price = &Money{*s.AmountMinor, "RUB", RuleVersion, now}
	}
	return nil
}

type Search struct {
	From, To, Date, Month, Sort, Cursor string
	Passengers, Limit                   int
}

func (q *Search) Validate(calendar bool, now time.Time) error {
	if _, _, err := ParseID(q.From); err != nil {
		return err
	}
	if _, _, err := ParseID(q.To); err != nil {
		return err
	}
	if q.From == q.To {
		return invalid("Отправление и прибытие должны различаться")
	}
	if q.Passengers == 0 {
		q.Passengers = 1
	}
	if q.Passengers != 1 {
		return invalid("Пока поддерживается один пассажир")
	}
	if q.Sort == "" {
		q.Sort = "departure"
	}
	if q.Sort != "departure" && q.Sort != "price" {
		return invalid("Неизвестная сортировка")
	}
	if q.Limit == 0 {
		q.Limit = 20
	}
	if q.Limit < 1 || q.Limit > 50 {
		return invalid("limit: от 1 до 50")
	}
	layout, value := "2006-01-02", q.Date
	if calendar {
		layout, value = "2006-01", q.Month
	}
	d, err := time.Parse(layout, value)
	if err != nil {
		return invalid("Некорректная дата")
	}
	// Bounded calendar horizon. Historical days in the current month remain explicit no_trips.
	floor := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
	if d.Before(floor) || !d.Before(floor.AddDate(0, 14, 0)) {
		return invalid("Дата вне доступного горизонта")
	}
	return nil
}

type SearchResult struct {
	Items      []Segment `json:"items"`
	NextCursor *string   `json:"next_cursor"`
}
type CalendarDay struct {
	Date          string `json:"date"`
	Status        string `json:"status"`
	MinPriceMinor *int64 `json:"min_price_minor"`
	TripCount     int    `json:"trip_count"`
}
type Calendar struct {
	Timezone     string        `json:"timezone"`
	Month        string        `json:"month"`
	CalculatedAt time.Time     `json:"calculated_at"`
	RuleVersion  string        `json:"rule_version"`
	Days         []CalendarDay `json:"days"`
}
type QuoteRequest struct {
	BoardingStopID  int `json:"boarding_stop_id"`
	AlightingStopID int `json:"alighting_stop_id"`
	Passengers      int `json:"passengers"`
}
type Quote struct {
	Segment
	ExpiresAt    *time.Time `json:"expires_at"`
	ReservesSeat bool       `json:"reserves_seat"`
}

func stopID(n int) string { return fmt.Sprintf("stop:%d", n) }
