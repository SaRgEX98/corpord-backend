package payments

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	yooKassaEndpoint = "https://api.yookassa.ru/v3"
	maxResponseBytes = 64 << 10
	// Current bank-card limits, not a claim about every payment method/shop.
	minPaymentMinor   int64 = 100
	maxPaymentMinor   int64 = 35000000
	intentMetadataKey       = "corpord_intent_id"
)

type Config struct {
	ShopID    string `json:"-"`
	SecretKey string `json:"-"`
	Timeout   time.Duration
}

func (Config) String() string     { return "payments.Config(redacted,test-only)" }
func (c Config) GoString() string { return c.String() }

// YooKassa implements the frozen Provider port. It has no live mode, endpoint
// override, business persistence, retry loop or access to passenger/card data.
type YooKassa struct {
	shopID, secret, endpoint string
	client                   *http.Client
}

var _ Provider = (*YooKassa)(nil)

func (YooKassa) String() string     { return "YooKassa(test-only,redacted)" }
func (y YooKassa) GoString() string { return y.String() }

var shopIDPattern = regexp.MustCompile(`^[0-9]{1,32}$`)
var opaquePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
var operationPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
var amountPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,8})\.[0-9]{2}$`)

func NewYooKassa(c Config) (*YooKassa, error) {
	// Prefix validation is an application safety gate before any network call;
	// the authenticated response's test flag and recipient are checked as well.
	if !shopIDPattern.MatchString(c.ShopID) || !strings.HasPrefix(c.SecretKey, "test_") || len(c.SecretKey) < 10 || len(c.SecretKey) > 512 || strings.IndexFunc(c.SecretKey, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 || c.Timeout < 0 || c.Timeout > 30*time.Second {
		return nil, problem(Configuration, "test_credentials_required")
	}
	if c.Timeout == 0 {
		c.Timeout = 10 * time.Second
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConnsPerHost = 4
	transport.ResponseHeaderTimeout = c.Timeout
	return &YooKassa{shopID: c.ShopID, secret: c.SecretKey, endpoint: yooKassaEndpoint, client: &http.Client{Transport: transport, Timeout: c.Timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func problem(kind, code string) error { return &Error{Kind: kind, Code: code} }
func decimal(minor int64) string      { return fmt.Sprintf("%d.%02d", minor/100, minor%100) }
func validAmount(minor int64) bool    { return minor >= minPaymentMinor && minor <= maxPaymentMinor }
func parseAmount(v, currency string) (int64, error) {
	if currency != "RUB" || !amountPattern.MatchString(v) {
		return 0, problem(Security, "invalid_amount")
	}
	whole, fraction, _ := strings.Cut(v, ".")
	n, e := strconv.ParseInt(whole, 10, 64)
	if e != nil {
		return 0, problem(Security, "invalid_amount")
	}
	f, _ := strconv.ParseInt(fraction, 10, 64)
	n = n*100 + f
	if !validAmount(n) {
		return 0, problem(Security, "invalid_amount")
	}
	return n, nil
}
func validReturnURL(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil || len(raw) > 2048 || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	ip := net.ParseIP(u.Hostname())
	return u.Scheme == "http" && (u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback()))
}
func validConfirmationURL(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil || len(raw) > 2048 || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "yookassa.ru" || strings.HasSuffix(host, ".yookassa.ru") || host == "yoomoney.ru" || strings.HasSuffix(host, ".yoomoney.ru")
}

type money struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}
type rawPayment struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Amount    money  `json:"amount"`
	Test      *bool  `json:"test"`
	Recipient struct {
		AccountID string `json:"account_id"`
	} `json:"recipient"`
	Metadata map[string]json.RawMessage `json:"metadata"`
	Method   struct {
		Type  string `json:"type"`
		Saved bool   `json:"saved"`
	} `json:"payment_method"`
	Confirmation *struct {
		Type string `json:"type"`
		URL  string `json:"confirmation_url"`
	} `json:"confirmation"`
	ExpiresAt *time.Time `json:"expires_at"`
}

func (y *YooKassa) payment(raw rawPayment, expectedID string) (Payment, error) {
	p := Payment{ID: raw.ID, Status: raw.Status, Currency: raw.Amount.Currency, MerchantID: raw.Recipient.AccountID, ExpiresAt: raw.ExpiresAt}
	if raw.Test == nil || !*raw.Test || raw.Recipient.AccountID != y.shopID || !opaquePattern.MatchString(raw.ID) || (expectedID != "" && raw.ID != expectedID) {
		return Payment{}, problem(Security, "payment_identity_mismatch")
	}
	if raw.Status != "pending" && raw.Status != "waiting_for_capture" && raw.Status != "succeeded" && raw.Status != "canceled" {
		return Payment{}, problem(Security, "unknown_payment_status")
	}
	if raw.Method.Type != "bank_card" || raw.Method.Saved {
		return Payment{}, problem(Security, "unsupported_payment_method")
	}
	if e := json.Unmarshal(raw.Metadata[intentMetadataKey], &p.IntentID); e != nil || !opaquePattern.MatchString(p.IntentID) {
		return Payment{}, problem(Security, "invalid_intent")
	}
	n, e := parseAmount(raw.Amount.Value, raw.Amount.Currency)
	if e != nil {
		return Payment{}, e
	}
	p.AmountMinor = n
	p.Test = true
	if raw.Confirmation != nil {
		if raw.Confirmation.Type != "redirect" || !validConfirmationURL(raw.Confirmation.URL) {
			return Payment{}, problem(Security, "invalid_confirmation")
		}
		p.ConfirmationURL = raw.Confirmation.URL
	}
	return p, nil
}
func (y *YooKassa) request(ctx context.Context, method, path, key string, body any, out any) error {
	if method == http.MethodPost && !operationPattern.MatchString(key) {
		return problem(Rejected, "invalid_operation_key")
	}
	var data []byte
	if body != nil {
		var e error
		data, e = json.Marshal(body)
		if e != nil {
			return problem(Rejected, "invalid_request")
		}
	}
	req, e := http.NewRequestWithContext(ctx, method, y.endpoint+path, bytes.NewReader(data))
	if e != nil {
		return problem(Configuration, "invalid_request")
	}
	req.SetBasicAuth(y.shopID, y.secret)
	req.Header.Set("Accept", "application/json")
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotence-Key", key)
	}
	response, e := y.client.Do(req)
	if e != nil {
		if method == http.MethodPost {
			return problem(Unknown, "transport_error")
		}
		return problem(Unavailable, "transport_error")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		kind := Rejected
		switch {
		case response.StatusCode == 401 || response.StatusCode == 403:
			kind = Configuration
		case response.StatusCode == 429:
			delay := time.Second
			if n, e := strconv.Atoi(response.Header.Get("Retry-After")); e == nil && n > 0 {
				delay = time.Duration(min(n, 3600)) * time.Second
			} else if when, e := http.ParseTime(response.Header.Get("Retry-After")); e == nil {
				delay = min(max(time.Until(when), time.Second), time.Hour)
			}
			return &Error{Kind: RateLimited, Code: "rate_limited", RetryAfter: delay}
		case response.StatusCode >= 500 || response.StatusCode == 408 || response.StatusCode == 409:
			kind = Unavailable
			if method == http.MethodPost {
				kind = Unknown
			}
		case response.StatusCode >= 300 && response.StatusCode < 400:
			kind = Security
		}
		return problem(kind, "http_"+strconv.Itoa(response.StatusCode))
	}
	data, e = io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if e != nil || len(data) > maxResponseBytes {
		if method == http.MethodPost {
			return problem(Unknown, "invalid_response")
		}
		return problem(Unavailable, "invalid_response")
	}
	if e = json.Unmarshal(data, out); e != nil {
		if method == http.MethodPost {
			return problem(Unknown, "invalid_response")
		}
		return problem(Unavailable, "invalid_response")
	}
	return nil
}
func (y *YooKassa) CreateAuthorization(ctx context.Context, key string, r AuthorizationRequest) (Payment, error) {
	if !validAmount(r.AmountMinor) || r.Currency != "RUB" || !opaquePattern.MatchString(r.IntentID) || !validReturnURL(r.ReturnURL) || !utf8.ValidString(r.Description) || utf8.RuneCountInString(r.Description) > 128 || strings.IndexFunc(r.Description, unicode.IsControl) >= 0 {
		return Payment{}, problem(Rejected, "invalid_authorization")
	}
	var raw rawPayment
	body := struct {
		Amount  money `json:"amount"`
		Capture bool  `json:"capture"`
		Method  struct {
			Type string `json:"type"`
		} `json:"payment_method_data"`
		Confirmation struct {
			Type      string `json:"type"`
			ReturnURL string `json:"return_url"`
		} `json:"confirmation"`
		Save        bool              `json:"save_payment_method"`
		Description string            `json:"description,omitempty"`
		Metadata    map[string]string `json:"metadata"`
	}{Amount: money{decimal(r.AmountMinor), r.Currency}, Description: r.Description, Metadata: map[string]string{intentMetadataKey: r.IntentID}}
	body.Method.Type = "bank_card"
	body.Confirmation.Type = "redirect"
	body.Confirmation.ReturnURL = r.ReturnURL
	if e := y.request(ctx, http.MethodPost, "/payments", key, body, &raw); e != nil {
		return Payment{}, e
	}
	p, e := y.payment(raw, "")
	if e != nil {
		return Payment{}, e
	}
	if p.AmountMinor != r.AmountMinor || p.IntentID != r.IntentID {
		return Payment{}, problem(Security, "payment_intent_mismatch")
	}
	return p, nil
}
func (y *YooKassa) GetPayment(ctx context.Context, id string) (Payment, error) {
	if !opaquePattern.MatchString(id) {
		return Payment{}, problem(Rejected, "invalid_payment_id")
	}
	var raw rawPayment
	if e := y.request(ctx, http.MethodGet, "/payments/"+id, "", nil, &raw); e != nil {
		return Payment{}, e
	}
	return y.payment(raw, id)
}
func (y *YooKassa) Capture(ctx context.Context, key, id string, minor int64) (Payment, error) {
	if !operationPattern.MatchString(key) || !opaquePattern.MatchString(id) || !validAmount(minor) {
		return Payment{}, problem(Rejected, "invalid_capture")
	}
	current, err := y.GetPayment(ctx, id)
	if err != nil {
		return Payment{}, err
	}
	if current.AmountMinor != minor {
		return Payment{}, problem(Security, "capture_amount_mismatch")
	}
	// A GET may already show a terminal result of an earlier timed-out POST.
	// Return facts; do not repeat an operation that is invalid for that state.
	if current.Status != "waiting_for_capture" {
		return current, nil
	}
	var raw rawPayment
	if e := y.request(ctx, http.MethodPost, "/payments/"+id+"/capture", key, struct {
		Amount money `json:"amount"`
	}{money{decimal(minor), "RUB"}}, &raw); e != nil {
		return Payment{}, e
	}
	p, e := y.payment(raw, id)
	if e != nil {
		return Payment{}, e
	}
	if p.AmountMinor != minor {
		return Payment{}, problem(Security, "capture_amount_mismatch")
	}
	return p, nil
}
func (y *YooKassa) CancelAuthorization(ctx context.Context, key, id string) (Payment, error) {
	if !operationPattern.MatchString(key) || !opaquePattern.MatchString(id) {
		return Payment{}, problem(Rejected, "invalid_payment_id")
	}
	current, err := y.GetPayment(ctx, id)
	if err != nil {
		return Payment{}, err
	}
	// YooKassa cancellation applies to an authorized payment, never pending.
	if current.Status != "waiting_for_capture" {
		return current, nil
	}
	var raw rawPayment
	if e := y.request(ctx, http.MethodPost, "/payments/"+id+"/cancel", key, struct{}{}, &raw); e != nil {
		return Payment{}, e
	}
	return y.payment(raw, id)
}

type rawRefund struct {
	ID        string                     `json:"id"`
	PaymentID string                     `json:"payment_id"`
	Status    string                     `json:"status"`
	Amount    money                      `json:"amount"`
	Metadata  map[string]json.RawMessage `json:"metadata"`
}

func refundFact(raw rawRefund, expectedID string) (Refund, error) {
	if !opaquePattern.MatchString(raw.ID) || !opaquePattern.MatchString(raw.PaymentID) || (expectedID != "" && raw.ID != expectedID) || (raw.Status != "pending" && raw.Status != "succeeded" && raw.Status != "canceled") {
		return Refund{}, problem(Security, "invalid_refund")
	}
	n, e := parseAmount(raw.Amount.Value, raw.Amount.Currency)
	if e != nil {
		return Refund{}, e
	}
	return Refund{ID: raw.ID, PaymentID: raw.PaymentID, Status: raw.Status, AmountMinor: n, Currency: raw.Amount.Currency}, nil
}
func (y *YooKassa) CreateRefund(ctx context.Context, key string, r RefundRequest) (Refund, error) {
	if !operationPattern.MatchString(key) || !opaquePattern.MatchString(r.PaymentID) || !validAmount(r.AmountMinor) || r.Currency != "RUB" || !opaquePattern.MatchString(r.IntentID) {
		return Refund{}, problem(Rejected, "invalid_refund")
	}
	// Refund objects do not carry test/recipient. Verify the parent via Basic
	// Auth before issuing a refund, including on a replay of the persisted key.
	p, e := y.GetPayment(ctx, r.PaymentID)
	if e != nil {
		return Refund{}, e
	}
	if p.IntentID != r.IntentID || p.AmountMinor != r.AmountMinor || p.Currency != r.Currency {
		return Refund{}, problem(Security, "refund_intent_mismatch")
	}
	if p.Status != "succeeded" {
		return Refund{}, problem(Rejected, "payment_not_succeeded")
	}
	var raw rawRefund
	body := struct {
		PaymentID string            `json:"payment_id"`
		Amount    money             `json:"amount"`
		Metadata  map[string]string `json:"metadata"`
	}{r.PaymentID, money{decimal(r.AmountMinor), r.Currency}, map[string]string{intentMetadataKey: r.IntentID}}
	if e = y.request(ctx, http.MethodPost, "/refunds", key, body, &raw); e != nil {
		return Refund{}, e
	}
	fact, e := refundFact(raw, "")
	if e != nil {
		return Refund{}, e
	}
	var intent string
	if json.Unmarshal(raw.Metadata[intentMetadataKey], &intent) != nil || intent != r.IntentID || fact.PaymentID != r.PaymentID || fact.AmountMinor != r.AmountMinor {
		return Refund{}, problem(Security, "refund_intent_mismatch")
	}
	return fact, nil
}
func (y *YooKassa) GetRefund(ctx context.Context, id string) (Refund, error) {
	if !opaquePattern.MatchString(id) {
		return Refund{}, problem(Rejected, "invalid_refund_id")
	}
	var raw rawRefund
	if e := y.request(ctx, http.MethodGet, "/refunds/"+id, "", nil, &raw); e != nil {
		return Refund{}, e
	}
	fact, e := refundFact(raw, id)
	if e != nil {
		return Refund{}, e
	}
	p, e := y.GetPayment(ctx, fact.PaymentID)
	if e != nil {
		return Refund{}, e
	}
	if p.Status != "succeeded" || fact.AmountMinor != p.AmountMinor {
		return Refund{}, problem(Security, "refund_payment_mismatch")
	}
	return fact, nil
}
