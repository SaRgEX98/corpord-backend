package handler

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type inboxStub struct {
	calls     int
	event, id string
	err       error
}

func (s *inboxStub) IngestWebhook(_ context.Context, event, id string) error {
	s.calls++
	s.event = event
	s.id = id
	return s.err
}
func TestPaymentNotificationDurableAckAndShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	valid := `{"type":"notification","event":"payment.succeeded","object":{"id":"fixture-payment-1234","amount":{"value":"999.00"},"card":{"number":"SENSITIVE_FAKE"}}}`
	for _, tc := range []struct {
		name, body    string
		enabled       bool
		err           error
		status, calls int
	}{
		{"durable", valid, true, nil, 200, 1},
		{"failed durable write", valid, true, errors.New("private SQL"), 503, 1},
		{"disabled", valid, false, nil, 503, 0},
		{"missingtype", `{"event":"payment.succeeded","object":{"id":"fixture-payment-1234"}}`, true, nil, 400, 0},
		{"trailing", valid + ` {}`, true, nil, 400, 0},
		{"badid", `{"type":"notification","event":"payment.succeeded","object":{"id":"../../payments"}}`, true, nil, 400, 0},
		{"large", strings.Repeat("x", 65537), true, nil, 400, 0},
		{"unknown event", strings.Replace(valid, "payment.succeeded", "payment.unknown", 1), true, nil, 200, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &inboxStub{err: tc.err}
			h := NewPaymentWebhook(stub, tc.enabled)
			r := gin.New()
			h.Register(r.Group(""))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("POST", "/payments/yookassa/webhook", strings.NewReader(tc.body)))
			if w.Code != tc.status || stub.calls != tc.calls {
				t.Fatalf("status/calls %d/%d", w.Code, stub.calls)
			}
			if strings.Contains(w.Body.String(), "SENSITIVE_FAKE") || strings.Contains(w.Body.String(), "private SQL") {
				t.Fatal("private payload exposed")
			}
		})
	}
}
func TestPaymentNotificationRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewPaymentWebhook(&inboxStub{}, true)
	r := gin.New()
	h.Register(r.Group(""))
	for n := 0; n < 121; n++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/payments/yookassa/webhook", strings.NewReader(`{}`)))
		if n == 120 && (w.Code != 429 || w.Header().Get("Retry-After") == "") {
			t.Fatal("unbounded intake")
		}
	}
}
