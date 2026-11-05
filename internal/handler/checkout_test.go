package handler

import (
	"context"
	"corpord-api/internal/checkout"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestCheckoutStrictInputsDisabledAuthAndNoStore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		method, path, body, key string
		owner                   bool
		status                  int
	}{
		{"POST", "/me/checkouts", `{"hold_id":1,"amount_minor":0}`, "valid-key", true, 400},
		{"POST", "/me/checkouts", `{"hold_id":1,"return_url":"https://attacker.invalid"}`, "valid-key", true, 400},
		{"POST", "/me/checkouts", `{"hold_id":1} {"hold_id":2}`, "valid-key", true, 400},
		{"POST", "/me/checkouts", `{"hold_id":0}`, "valid-key", true, 400},
		{"POST", "/me/checkouts", `{"hold_id":1}`, "bad", true, 400},
		{"POST", "/me/checkouts", `{"hold_id":1}`, "valid-key", true, 503},
		{"POST", "/me/checkouts", `{"hold_id":1}`, "valid-key", false, 401},
		{"GET", "/me/checkouts?limit=101", "", "", true, 400},
		{"GET", "/me/checkouts?status=bogus", "", "", true, 400},
		{"GET", "/me/checkouts", "", "", false, 401},
		{"GET", "/me/checkouts/0", "", "", true, 400},
		{"POST", "/me/checkouts/1/reconcile", "", "", true, 503},
	} {
		h := NewCheckout(checkout.New(nil, nil, checkout.Options{}))
		r := gin.New()
		pub := r.Group("")
		auth := r.Group("")
		if tc.owner {
			auth.Use(func(c *gin.Context) { c.Set("userID", 1) })
		}
		h.Register(pub, auth)
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", tc.key)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tc.status || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s %s got=%d cache=%s body=%s", tc.method, tc.path, w.Code, w.Header().Get("Cache-Control"), w.Body.String())
		}
	}
}
func TestCheckoutSafeErrorsAndCooldown(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{errors.New("private provider URL secret passenger"), 500, "internal_error"},
		{context.DeadlineExceeded, 503, "provider_unavailable"},
		{&pgconn.PgError{Code: "40P01", Message: "private SQL"}, 503, "provider_unavailable"},
		{&checkout.Error{Status: 429, Code: "rate_limited", Message: "Попробуйте позже"}, 429, "rate_limited"},
	} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		checkoutError(c, tc.err)
		if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.code) || strings.Contains(w.Body.String(), "private") {
			t.Fatalf("unsafe error %d %s", w.Code, w.Body.String())
		}
		if tc.status == 429 && w.Header().Get("Retry-After") != "5" {
			t.Fatal("cooldown header absent")
		}
	}
}
