// Package checkout owns durable sandbox checkout and the shared seat ledger.
// All provider calls run outside database transactions and row locks.
package checkout

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

type Options struct {
	Enabled      bool
	ReturnOrigin string
}
type Error struct {
	Status        int
	Code, Message string
}

func (e *Error) Error() string { return e.Message }
func failure(status int, code string) error {
	messages := map[string]string{"invalid_request": "Некорректные параметры запроса", "not_found": "Заказ или удержание не найдено", "hold_expired": "Срок удержания истёк", "checkout_exists": "Удержание уже связано с заказом", "idempotency_conflict": "Ключ использован для другого запроса", "checkout_in_progress": "Оплата уже обрабатывается", "paid_order": "Заказ оплачен", "not_payable": "Удержание недоступно для оплаты", "rate_limited": "Повторите сверку немного позже", "payments_disabled": "Тестовая оплата отключена", "provider_unavailable": "Оплата временно недоступна"}
	return &Error{status, code, messages[code]}
}

type Actions struct {
	CanPay              bool `json:"can_pay"`
	CanCancel           bool `json:"can_cancel"`
	CanReconcile        bool `json:"can_reconcile"`
	CanViewConfirmation bool `json:"can_view_confirmation"`
}
type Summary struct {
	ID              int64     `json:"id" db:"id"`
	HoldID          int64     `json:"hold_id" db:"hold_id"`
	TripID          int       `json:"trip_id" db:"trip_id"`
	Status          string    `json:"status" db:"status"`
	PaymentStatus   string    `json:"payment_status" db:"payment_status"`
	AmountMinor     int64     `json:"amount_minor" db:"amount_minor"`
	Currency        string    `json:"currency" db:"currency"`
	IsDemo          bool      `json:"is_demo" db:"is_demo"`
	Test            bool      `json:"test" db:"test"`
	PassengerName   string    `json:"passenger_name" db:"passenger_name"`
	BoardingName    string    `json:"boarding_name" db:"boarding_name"`
	AlightingName   string    `json:"alighting_name" db:"alighting_name"`
	DepartureTime   time.Time `json:"departure_time" db:"departure_time"`
	ArrivalTime     time.Time `json:"arrival_time" db:"arrival_time"`
	Timezone        string    `json:"timezone" db:"timezone"`
	ArrivalTimezone string    `json:"arrival_timezone" db:"arrival_timezone"`
	SeatNumber      int       `json:"seat_number" db:"seat_number"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
	HoldExpiresAt   time.Time `json:"hold_expires_at" db:"hold_expires_at"`
	Actions         Actions   `json:"actions" db:"-"`
}
type Detail struct {
	Summary
	Payment struct {
		ConfirmationURL *string `json:"confirmation_url"`
	} `json:"payment"`
	NextPollAfterMS int `json:"next_poll_after_ms"`
}
type Page struct {
	Items      []Summary `json:"items"`
	NextCursor *string   `json:"next_cursor"`
}

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)
var statuses = map[string]bool{"awaiting_payment": true, "committing": true, "paid": true, "cancel_pending": true, "cancelled": true, "expired": true, "review_required": true, "refund_pending": true, "refunded": true}

func terminal(status string) bool {
	return status == "paid" || status == "cancelled" || status == "expired" || status == "refunded"
}
func scope(owner int, status string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d:%s", owner, status))))
}
func decodeCursor(owner int, status, cursor string) (int64, error) {
	if cursor == "" {
		return 0, nil
	}
	if len(cursor) > 512 {
		return 0, failure(400, "invalid_request")
	}
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, failure(400, "invalid_request")
	}
	var c struct {
		ID    int64
		Scope string
	}
	if json.Unmarshal(b, &c) != nil || c.ID < 1 || c.Scope != scope(owner, status) {
		return 0, failure(400, "invalid_request")
	}
	return c.ID, nil
}
func encodeCursor(owner int, status string, id int64) *string {
	b, _ := json.Marshal(struct {
		ID    int64
		Scope string
	}{id, scope(owner, status)})
	v := base64.RawURLEncoding.EncodeToString(b)
	return &v
}
