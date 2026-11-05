// Package booking implements expiring demo inventory holds. It never sells a
// ticket or infers a real carrier's availability from a bus capacity.
package booking

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const HoldSeconds = 600

type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }
func invalid() error {
	return &Error{400, "invalid_request", "Некорректные параметры запроса"}
}
func missing() error { return &Error{404, "not_found", "Запись не найдена"} }
func conflict(code string) error {
	messages := map[string]string{"sold_out": "Свободных мест на участке нет", "hold_limit": "Можно удерживать не более пяти мест", "idempotency_conflict": "Ключ уже использован для другого запроса", "trip_unavailable": "Рейс или участок недоступен", "inventory_unavailable": "Удержание места на этом рейсе недоступно", "ambiguous_segment": "Остановки повторяются: участок неоднозначен", "checkout_in_progress": "Удержание связано с оплатой. Откройте заказ", "paid_order": "Место закреплено за заказом"}
	return &Error{409, code, messages[code]}
}

type Booking struct {
	ID              int64      `json:"id" db:"id"`
	TripID          int        `json:"trip_id" db:"trip_id"`
	BoardingStopID  int        `json:"boarding_stop_id" db:"boarding_stop_id"`
	AlightingStopID int        `json:"alighting_stop_id" db:"alighting_stop_id"`
	BoardingName    string     `json:"boarding_name" db:"boarding_name"`
	AlightingName   string     `json:"alighting_name" db:"alighting_name"`
	DepartureTime   time.Time  `json:"departure_time" db:"departure_time"`
	ArrivalTime     time.Time  `json:"arrival_time" db:"arrival_time"`
	Timezone        string     `json:"timezone" db:"timezone"`
	ArrivalTimezone string     `json:"arrival_timezone" db:"arrival_timezone"`
	PassengerName   string     `json:"passenger_name" db:"passenger_name"`
	Status          string     `json:"status" db:"status"`
	SeatNumber      int        `json:"seat_number" db:"seat_number"`
	AmountMinor     int64      `json:"amount_minor" db:"amount_minor"`
	Currency        string     `json:"currency" db:"currency"`
	IsDemo          bool       `json:"is_demo" db:"is_demo"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	ExpiresAt       time.Time  `json:"expires_at" db:"expires_at"`
	CancelledAt     *time.Time `json:"cancelled_at" db:"cancelled_at"`
	CheckoutID      *int64     `json:"checkout_id" db:"checkout_id"`
	CheckoutEnabled bool       `json:"checkout_enabled" db:"-"`
}
type Passenger struct {
	ID        int64     `json:"id" db:"id"`
	FullName  string    `json:"full_name" db:"full_name"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}
type Availability struct {
	TripID          int    `json:"trip_id"`
	BoardingStopID  int    `json:"boarding_stop_id"`
	AlightingStopID int    `json:"alighting_stop_id"`
	IsDemo          bool   `json:"is_demo"`
	Bookable        bool   `json:"bookable"`
	Status          string `json:"status"`
	Seats           *int   `json:"seats"`
	HoldSeconds     int    `json:"hold_seconds"`
}
type CreateRequest struct {
	TripID          int     `json:"trip_id"`
	BoardingStopID  int     `json:"boarding_stop_id"`
	AlightingStopID int     `json:"alighting_stop_id"`
	PassengerID     *int64  `json:"passenger_id"`
	PassengerName   *string `json:"passenger_name"`
}
type Page[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

var keyPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{8,128}$`)

func NormalizeName(v string) (string, error) {
	v = strings.TrimSpace(v)
	if !utf8.ValidString(v) || utf8.RuneCountInString(v) < 1 || utf8.RuneCountInString(v) > 160 {
		return "", invalid()
	}
	for _, r := range v {
		if unicode.IsControl(r) {
			return "", invalid()
		}
	}
	return v, nil
}
func (r *CreateRequest) validate(key string) error {
	if !keyPattern.MatchString(key) || r.TripID < 1 || r.TripID > math.MaxInt32 || r.BoardingStopID < 1 || r.BoardingStopID > math.MaxInt32 || r.AlightingStopID < 1 || r.AlightingStopID > math.MaxInt32 || r.BoardingStopID == r.AlightingStopID || (r.PassengerID == nil) == (r.PassengerName == nil) {
		return invalid()
	}
	if r.PassengerID != nil && *r.PassengerID < 1 {
		return invalid()
	}
	if r.PassengerName != nil {
		v, err := NormalizeName(*r.PassengerName)
		if err != nil {
			return err
		}
		r.PassengerName = &v
	}
	return nil
}
func (r CreateRequest) hash() string {
	b, _ := json.Marshal(r)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

// Cursors are position hints, not credentials. SQL always applies the authenticated
// owner independently of their scope, including for tampered cursors.
func pagePosition(owner int, kind, encoded string, limit int) (int64, int, error) {
	if owner < 1 || limit < 0 || limit > 100 {
		return 0, 0, invalid()
	}
	if limit == 0 {
		limit = 30
	}
	if encoded == "" {
		return 0, limit, nil
	}
	if len(encoded) > 512 {
		return 0, 0, invalid()
	}
	b, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return 0, 0, invalid()
	}
	var c struct {
		ID    int64
		Scope string
	}
	if json.Unmarshal(b, &c) != nil || c.ID < 1 || c.Scope != pageScope(owner, kind) {
		return 0, 0, invalid()
	}
	return c.ID, limit, nil
}
func pageScope(owner int, kind string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d:%s", owner, kind))))
}
func pageCursor(owner int, kind string, id int64) *string {
	b, _ := json.Marshal(struct {
		ID    int64
		Scope string
	}{id, pageScope(owner, kind)})
	v := base64.RawURLEncoding.EncodeToString(b)
	return &v
}
