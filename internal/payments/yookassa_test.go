package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testShop = "123456"
const testSecret = "test_local_fictional_secret"
const paymentID = "2f9714aa-000f-5000-9000-000000000001"
const refundID = "2f9714aa-000f-5000-9000-000000000002"

func authorization() AuthorizationRequest {
	return AuthorizationRequest{AmountMinor: 12345, Currency: "RUB", IntentID: "checkout_42", Description: "Тестовый заказ №42", ReturnURL: "http://localhost:3000/me/orders/42/payment-return"}
}
func fixturePayment(status string) map[string]any {
	return map[string]any{"id": paymentID, "status": status, "amount": map[string]any{"value": "123.45", "currency": "RUB"}, "test": true, "recipient": map[string]any{"account_id": testShop}, "payment_method": map[string]any{"type": "bank_card", "saved": false}, "metadata": map[string]any{intentMetadataKey: "checkout_42"}}
}
func fixtureRefund() map[string]any {
	return map[string]any{"id": refundID, "status": "succeeded", "payment_id": paymentID, "amount": map[string]any{"value": "123.45", "currency": "RUB"}, "metadata": map[string]any{intentMetadataKey: "checkout_42"}}
}
func localProvider(t *testing.T, h http.HandlerFunc) *YooKassa {
	t.Helper()
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	y, e := NewYooKassa(Config{ShopID: testShop, SecretKey: testSecret, Timeout: time.Second})
	if e != nil {
		t.Fatal(e)
	}
	// Package-private test substitution, deliberately absent from public Config.
	y.endpoint = server.URL + "/v3"
	return y
}
func kind(t *testing.T, e error, want string) {
	t.Helper()
	var p *Error
	if !errors.As(e, &p) || p.Kind != want {
		t.Fatalf("want safe kind %s got %v", want, e)
	}
	if strings.Contains(fmt.Sprint(e), testSecret) || strings.Contains(fmt.Sprint(e), "checkout/payments") {
		t.Fatal("error leaks sensitive content")
	}
}
func writeObject(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func TestConstructorTestOnlyAndRedaction(t *testing.T) {
	for _, c := range []Config{{}, {ShopID: testShop, SecretKey: "live_secret"}, {ShopID: "not_numeric", SecretKey: testSecret}, {ShopID: testShop, SecretKey: "test_bad\nsecret"}, {ShopID: testShop, SecretKey: testSecret, Timeout: -time.Second}, {ShopID: testShop, SecretKey: testSecret, Timeout: 31 * time.Second}} {
		_, e := NewYooKassa(c)
		kind(t, e, Configuration)
	}
	c := Config{ShopID: testShop, SecretKey: testSecret}
	y, e := NewYooKassa(c)
	if e != nil {
		t.Fatal(e)
	}
	if y.endpoint != yooKassaEndpoint || y.client.Timeout != 10*time.Second {
		t.Fatal("unexpected defaults")
	}
	for _, s := range []string{fmt.Sprintf("%v", c), fmt.Sprintf("%+v", c), fmt.Sprintf("%#v", c), fmt.Sprintf("%v", y), fmt.Sprintf("%#v", y), fmt.Sprintf("%+v", *y), fmt.Sprintf("%#v", *y)} {
		if strings.Contains(s, testSecret) || strings.Contains(s, testShop) {
			t.Fatal("credentials not redacted")
		}
	}
	b, _ := json.Marshal(c)
	if strings.Contains(string(b), testSecret) {
		t.Fatal("config JSON leaks credentials")
	}
}

func TestAuthorizationWireContractAndStableDuplicateEffects(t *testing.T) {
	var mu sync.Mutex
	keys := map[string]string{}
	effects := map[string]int{}
	accepted := false
	calls := 0
	status := "pending"
	y := localProvider(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		user, secret, ok := r.BasicAuth()
		if !ok || user != testShop || secret != testSecret {
			t.Error("missing Basic Auth")
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Error("missing accept")
		}
		if r.Method == "GET" {
			if r.Header.Get("Idempotence-Key") != "" {
				t.Error("GET key")
			}
			writeObject(w, fixturePayment(status))
			return
		}
		key := r.Header.Get("Idempotence-Key")
		if len(key) == 0 || len(key) > 64 || r.Header.Get("Idempotency-Key") != "" {
			t.Error("wrong idempotence header")
		}
		data, _ := io.ReadAll(r.Body)
		if before, exists := keys[key]; exists && before != string(data) {
			t.Error("same operation key got another payload")
		}
		if _, exists := keys[key]; !exists {
			effects[r.URL.Path]++
			keys[key] = string(data)
		}
		switch r.URL.Path {
		case "/v3/payments":
			var body map[string]any
			if e := json.Unmarshal(data, &body); e != nil {
				t.Error(e)
			}
			if body["capture"] != false || body["save_payment_method"] != false {
				t.Error("not two-stage/no-save")
			}
			if body["payment_method_data"].(map[string]any)["type"] != "bank_card" || body["confirmation"].(map[string]any)["type"] != "redirect" {
				t.Error("unsupported payment flow")
			}
			if body["amount"].(map[string]any)["value"] != "123.45" {
				t.Error("float monetary formatting")
			}
			if body["metadata"].(map[string]any)[intentMetadataKey] != "checkout_42" {
				t.Error("intent missing")
			}
			if !accepted {
				accepted = true
				w.WriteHeader(500)
				w.Write([]byte(`{"description":"sensitive raw failure"}`))
				return
			}
			p := fixturePayment(status)
			p["confirmation"] = map[string]any{"type": "redirect", "confirmation_url": "https://yoomoney.ru/checkout/payments/v2/contract?orderId=private"}
			writeObject(w, p)
		case "/v3/payments/" + paymentID + "/capture":
			status = "succeeded"
			writeObject(w, fixturePayment(status))
		case "/v3/refunds":
			writeObject(w, fixtureRefund())
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	})
	ctx := context.Background()
	_, e := y.CreateAuthorization(ctx, "create_stable_42", authorization())
	kind(t, e, Unknown)
	payment, e := y.CreateAuthorization(ctx, "create_stable_42", authorization())
	if e != nil || payment.ID != paymentID || !payment.Test || payment.AmountMinor != 12345 || payment.IntentID != "checkout_42" {
		t.Fatalf("retry payment %+v %v", payment, e)
	}
	mu.Lock()
	status = "waiting_for_capture"
	mu.Unlock()
	for i := 0; i < 2; i++ {
		p, e := y.Capture(ctx, "capture_stable_42", paymentID, 12345)
		if e != nil || p.Status != "succeeded" {
			t.Fatalf("capture %+v %v", p, e)
		}
	}
	for i := 0; i < 2; i++ {
		r, e := y.CreateRefund(ctx, "refund_stable_42", RefundRequest{PaymentID: paymentID, AmountMinor: 12345, Currency: "RUB", IntentID: "checkout_42"})
		if e != nil || r.ID != refundID {
			t.Fatalf("refund %+v %v", r, e)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	for _, path := range []string{"/v3/payments", "/v3/payments/" + paymentID + "/capture", "/v3/refunds"} {
		if effects[path] != 1 {
			t.Errorf("duplicated financial effect %s: %d", path, effects[path])
		}
	}
	if calls < 8 {
		t.Fatalf("expected preflight verification, got %d calls", calls)
	}
}

func TestPaymentFactsRejectForeignMerchantMethodAndMalformedMoney(t *testing.T) {
	variants := map[string]func(map[string]any){
		"live": func(p map[string]any) { p["test"] = false }, "missing_test": func(p map[string]any) { delete(p, "test") },
		"merchant":      func(p map[string]any) { p["recipient"] = map[string]any{"account_id": "654321"} },
		"method":        func(p map[string]any) { p["payment_method"] = map[string]any{"type": "sbp"} },
		"saved":         func(p map[string]any) { p["payment_method"] = map[string]any{"type": "bank_card", "saved": true} },
		"currency":      func(p map[string]any) { p["amount"] = map[string]any{"value": "123.45", "currency": "USD"} },
		"amount_number": func(p map[string]any) { p["amount"] = map[string]any{"value": 123.45, "currency": "RUB"} },
		"negative":      func(p map[string]any) { p["amount"] = map[string]any{"value": "-123.45", "currency": "RUB"} },
		"precision":     func(p map[string]any) { p["amount"] = map[string]any{"value": "123.450", "currency": "RUB"} },
		"exponent":      func(p map[string]any) { p["amount"] = map[string]any{"value": "1e2", "currency": "RUB"} },
		"overflow": func(p map[string]any) {
			p["amount"] = map[string]any{"value": "9223372036854775807.00", "currency": "RUB"}
		},
		"missing_intent": func(p map[string]any) { delete(p, "metadata") },
		"intent_type":    func(p map[string]any) { p["metadata"] = map[string]any{intentMetadataKey: 42} },
		"status":         func(p map[string]any) { p["status"] = "new_unrecognized_status" },
		"id":             func(p map[string]any) { p["id"] = "other_payment" },
		"confirmation_http": func(p map[string]any) {
			p["confirmation"] = map[string]any{"type": "redirect", "confirmation_url": "http://yoomoney.ru/pay"}
		},
		"confirmation_host": func(p map[string]any) {
			p["confirmation"] = map[string]any{"type": "redirect", "confirmation_url": "https://yoomoney.ru.evil.invalid/pay"}
		},
		"confirmation_userinfo": func(p map[string]any) {
			p["confirmation"] = map[string]any{"type": "redirect", "confirmation_url": "https://secret@yoomoney.ru/pay"}
		},
		"confirmation_type": func(p map[string]any) {
			p["confirmation"] = map[string]any{"type": "embedded", "confirmation_url": "https://yoomoney.ru/pay"}
		},
	}
	for name, change := range variants {
		t.Run(name, func(t *testing.T) {
			y := localProvider(t, func(w http.ResponseWriter, r *http.Request) {
				p := fixturePayment("pending")
				change(p)
				writeObject(w, p)
			})
			_, e := y.GetPayment(context.Background(), paymentID)
			if name == "amount_number" {
				kind(t, e, Unavailable)
			} else {
				kind(t, e, Security)
			}
		})
	}
	for _, name := range []string{"amount", "intent"} {
		t.Run("create_mismatch_"+name, func(t *testing.T) {
			y := localProvider(t, func(w http.ResponseWriter, r *http.Request) {
				p := fixturePayment("pending")
				if name == "amount" {
					p["amount"] = map[string]any{"value": "123.46", "currency": "RUB"}
				} else {
					p["metadata"] = map[string]any{intentMetadataKey: "other_intent"}
				}
				writeObject(w, p)
			})
			_, e := y.CreateAuthorization(context.Background(), "create_key", authorization())
			kind(t, e, Security)
		})
	}
}

func TestCaptureAndCancelPreflightNeverMutateUnverifiedOrPendingPayments(t *testing.T) {
	for _, operation := range []string{"capture", "cancel"} {
		for _, status := range []string{"pending", "canceled", "succeeded"} {
			t.Run(operation+"_"+status, func(t *testing.T) {
				var posts atomic.Int32
				y := localProvider(t, func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" {
						posts.Add(1)
					}
					writeObject(w, fixturePayment(status))
				})
				var p Payment
				var e error
				if operation == "capture" {
					p, e = y.Capture(context.Background(), "capture_key", paymentID, 12345)
				} else {
					p, e = y.CancelAuthorization(context.Background(), "cancel_key", paymentID)
				}
				if e != nil || p.Status != status || posts.Load() != 0 {
					t.Fatalf("invalid state mutated %+v %v posts%d", p, e, posts.Load())
				}
			})
		}
	}
	for _, operation := range []string{"capture", "cancel", "refund"} {
		t.Run(operation+"_live_parent", func(t *testing.T) {
			var posts atomic.Int32
			y := localProvider(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "POST" {
					posts.Add(1)
				}
				p := fixturePayment("waiting_for_capture")
				p["test"] = false
				writeObject(w, p)
			})
			var e error
			switch operation {
			case "capture":
				_, e = y.Capture(context.Background(), "capture_key", paymentID, 12345)
			case "cancel":
				_, e = y.CancelAuthorization(context.Background(), "cancel_key", paymentID)
			case "refund":
				_, e = y.CreateRefund(context.Background(), "refund_key", RefundRequest{PaymentID: paymentID, AmountMinor: 12345, Currency: "RUB", IntentID: "checkout_42"})
			}
			kind(t, e, Security)
			if posts.Load() != 0 {
				t.Fatal("mutated unverified parent")
			}
		})
	}
}

func TestCancellationAndRefundVerification(t *testing.T) {
	var posts atomic.Int32
	y := localProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/refunds/") {
			writeObject(w, fixtureRefund())
			return
		}
		if r.Method == "GET" {
			status := "succeeded"
			if posts.Load() == 0 {
				status = "waiting_for_capture"
			}
			writeObject(w, fixturePayment(status))
			return
		}
		posts.Add(1)
		writeObject(w, fixturePayment("canceled"))
	})
	p, e := y.CancelAuthorization(context.Background(), "cancel_key", paymentID)
	if e != nil || p.Status != "canceled" || posts.Load() != 1 {
		t.Fatalf("cancel %+v %v", p, e)
	}
	refund, e := y.GetRefund(context.Background(), refundID)
	if e != nil || refund.PaymentID != paymentID || refund.AmountMinor != 12345 {
		t.Fatalf("refund %+v %v", refund, e)
	}
	for _, change := range []string{"amount", "payment", "intent", "id", "currency", "status"} {
		t.Run("refund_mismatch_"+change, func(t *testing.T) {
			y := localProvider(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" {
					writeObject(w, fixturePayment("succeeded"))
					return
				}
				v := fixtureRefund()
				switch change {
				case "amount":
					v["amount"] = map[string]any{"value": "123.46", "currency": "RUB"}
				case "payment":
					v["payment_id"] = "other_payment"
				case "intent":
					v["metadata"] = map[string]any{intentMetadataKey: "other_intent"}
				case "id":
					v["id"] = "../unsafe"
				case "currency":
					v["amount"] = map[string]any{"value": "123.45", "currency": "USD"}
				case "status":
					v["status"] = "unknown"
				}
				writeObject(w, v)
			})
			_, e := y.CreateRefund(context.Background(), "refund_key", RefundRequest{PaymentID: paymentID, AmountMinor: 12345, Currency: "RUB", IntentID: "checkout_42"})
			kind(t, e, Security)
		})
	}
}

func TestHTTPFailuresUnknownOutcomeAndNoCredentialRedirect(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404, 408, 409, 422, 429, 500, 502, 503} {
		for _, write := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d_post_%t", status, write), func(t *testing.T) {
				y := localProvider(t, func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Retry-After", "7")
					w.WriteHeader(status)
					w.Write([]byte(`{"description":"private provider body"}`))
				})
				var e error
				if write {
					_, e = y.CreateAuthorization(context.Background(), "stable_key", authorization())
				} else {
					_, e = y.GetPayment(context.Background(), paymentID)
				}
				want := Rejected
				switch {
				case status == 401 || status == 403:
					want = Configuration
				case status == 429:
					want = RateLimited
				case status >= 500 || status == 408 || status == 409:
					want = Unavailable
					if write {
						want = Unknown
					}
				}
				kind(t, e, want)
				if status == 429 {
					var p *Error
					errors.As(e, &p)
					if p.RetryAfter != 7*time.Second {
						t.Fatal("retry after lost")
					}
				}
			})
		}
	}
	var leaked atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked.Add(1) }))
	defer target.Close()
	y := localProvider(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL+"/secret", 307) })
	_, e := y.CreateAuthorization(context.Background(), "stable_key", authorization())
	kind(t, e, Security)
	if leaked.Load() != 0 {
		t.Fatal("redirect followed with credentials")
	}
}

func TestBoundedMalformedAndTimeoutResponses(t *testing.T) {
	for _, body := range []string{`{"id":`, strings.Repeat("x", maxResponseBytes+1), `{} {}`} {
		for _, write := range []bool{false, true} {
			t.Run(fmt.Sprintf("size%d_post%t", len(body), write), func(t *testing.T) {
				y := localProvider(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) })
				var e error
				if write {
					_, e = y.CreateAuthorization(context.Background(), "stable_key", authorization())
					kind(t, e, Unknown)
				} else {
					_, e = y.GetPayment(context.Background(), paymentID)
					kind(t, e, Unavailable)
				}
			})
		}
	}
	for _, write := range []bool{false, true} {
		t.Run(fmt.Sprintf("timeout_post%t", write), func(t *testing.T) {
			y := localProvider(t, func(w http.ResponseWriter, r *http.Request) {
				io.Copy(io.Discard, r.Body)
				select {
				case <-r.Context().Done():
				case <-time.After(250 * time.Millisecond):
				}
			})
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			var e error
			if write {
				_, e = y.CreateAuthorization(ctx, "stable_key", authorization())
				kind(t, e, Unknown)
			} else {
				_, e = y.GetPayment(ctx, paymentID)
				kind(t, e, Unavailable)
			}
		})
	}
}

func TestValidationPreventsNetworkRequests(t *testing.T) {
	var calls atomic.Int32
	y := localProvider(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(500) })
	for _, amount := range []int64{-1, 0, 1, 99, 35000001, 9007199254740991} {
		r := authorization()
		r.AmountMinor = amount
		_, e := y.CreateAuthorization(context.Background(), "valid_key", r)
		kind(t, e, Rejected)
	}
	for _, key := range []string{"", strings.Repeat("x", 65), "bad\r\nkey", "with space"} {
		_, e := y.CreateAuthorization(context.Background(), key, authorization())
		kind(t, e, Rejected)
	}
	for _, id := range []string{"", "../payments", "id?query=1", "id#fragment", "id/other", "https://evil.invalid"} {
		_, e := y.GetPayment(context.Background(), id)
		kind(t, e, Rejected)
	}
	for _, url := range []string{"javascript:alert(1)", "http://public.invalid/return", "https://user:secret@public.invalid/return", "https://public.invalid/#fragment"} {
		r := authorization()
		r.ReturnURL = url
		_, e := y.CreateAuthorization(context.Background(), "valid_key", r)
		kind(t, e, Rejected)
	}
	if calls.Load() != 0 {
		t.Fatal("validation sent network calls", calls.Load())
	}
	for _, n := range []int64{100, 101, 12345, 35000000} {
		actual, e := parseAmount(decimal(n), "RUB")
		if e != nil || actual != n {
			t.Fatalf("integer roundtrip %d %d %v", n, actual, e)
		}
	}
}

func TestAcceptedButConnectionLostReplaysTheSameFinancialEffect(t *testing.T) {
	for _, operation := range []string{"create", "capture", "cancel", "refund"} {
		t.Run(operation, func(t *testing.T) {
			var mu sync.Mutex
			accepted := false
			effects := 0
			status := "waiting_for_capture"
			if operation == "refund" {
				status = "succeeded"
			}
			y := localProvider(t, func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				if r.Method == "GET" {
					writeObject(w, fixturePayment(status))
					return
				}
				io.Copy(io.Discard, r.Body)
				if r.Header.Get("Idempotence-Key") != operation+"_persisted_key" {
					t.Error("changed retry operation key")
				}
				if !accepted {
					accepted = true
					effects++
					if operation == "capture" {
						status = "succeeded"
					}
					if operation == "cancel" {
						status = "canceled"
					}
					connection, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					connection.Close() // Financial effect accepted, no HTTP response.
					return
				}
				if operation == "refund" {
					writeObject(w, fixtureRefund())
				} else {
					writeObject(w, fixturePayment(status))
				}
			})
			call := func() error {
				ctx := context.Background()
				key := operation + "_persisted_key"
				switch operation {
				case "create":
					_, err := y.CreateAuthorization(ctx, key, authorization())
					return err
				case "capture":
					_, err := y.Capture(ctx, key, paymentID, 12345)
					return err
				case "cancel":
					_, err := y.CancelAuthorization(ctx, key, paymentID)
					return err
				default:
					_, err := y.CreateRefund(ctx, key, RefundRequest{PaymentID: paymentID, AmountMinor: 12345, Currency: "RUB", IntentID: "checkout_42"})
					return err
				}
			}
			kind(t, call(), Unknown)
			if err := call(); err != nil {
				t.Fatalf("recovery failed: %v", err)
			}
			mu.Lock()
			defer mu.Unlock()
			if effects != 1 {
				t.Fatalf("duplicate effects: %d", effects)
			}
		})
	}
}
