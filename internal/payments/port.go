// Package payments defines a test-shop-only provider boundary. Business seat
// transitions belong to checkout, never the HTTP adapter.
package payments

import (
	"context"
	"time"
)

type Payment struct {
	ID              string
	Status          string
	AmountMinor     int64
	Currency        string
	Test            bool
	MerchantID      string
	IntentID        string
	ConfirmationURL string
	ExpiresAt       *time.Time
}
type Refund struct {
	ID          string
	PaymentID   string
	Status      string
	AmountMinor int64
	Currency    string
}
type AuthorizationRequest struct {
	AmountMinor int64
	Currency    string
	Description string
	IntentID    string
	ReturnURL   string
}
type RefundRequest struct {
	PaymentID   string
	AmountMinor int64
	Currency    string
	IntentID    string
}
type Provider interface {
	CreateAuthorization(ctx context.Context, key string, request AuthorizationRequest) (Payment, error)
	GetPayment(ctx context.Context, paymentID string) (Payment, error)
	Capture(ctx context.Context, key string, paymentID string, amountMinor int64) (Payment, error)
	CancelAuthorization(ctx context.Context, key string, paymentID string) (Payment, error)
	CreateRefund(ctx context.Context, key string, request RefundRequest) (Refund, error)
	GetRefund(ctx context.Context, refundID string) (Refund, error)
}

// Unknown outcome must be reconciled; it is never proof that money was not moved.
const (
	Unknown       = "unknown"
	Rejected      = "rejected"
	RateLimited   = "rate_limited"
	Unavailable   = "unavailable"
	Security      = "security"
	Configuration = "configuration"
)

type Error struct {
	Kind       string
	Code       string
	RetryAfter time.Duration
}

func (e *Error) Error() string { return "payment provider: " + e.Kind }
