package handler

import (
	"context"
	"corpord-api/internal/booking"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestBookingStrictInputAndAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewBooking(nil)
	// Invalid requests are rejected before consulting the database. Auth identity
	// must come from trusted middleware, never a body field or arbitrary header.
	for _, tc := range []struct {
		method, path, body, key string
		owner                   bool
	}{
		{"POST", "/me/bookings", `{"trip_id":1,"boarding_stop_id":1,"alighting_stop_id":2,"passenger_name":"Test","user_id":99}`, "valid-key", true},
		{"POST", "/me/bookings", `{"trip_id":1,"boarding_stop_id":1,"alighting_stop_id":2,"passenger_name":"Test","amount_minor":0}`, "valid-key", true},
		{"POST", "/me/bookings", `{"trip_id":1,"boarding_stop_id":1,"alighting_stop_id":2,"passenger_name":"Test"}`, "bad", true},
		{"POST", "/me/bookings", `{"trip_id":1,"boarding_stop_id":1,"alighting_stop_id":2,"passenger_name":"Test","passenger_id":1}`, "valid-key", true},
		{"POST", "/me/bookings", `{"trip_id":2147483648,"boarding_stop_id":1,"alighting_stop_id":2,"passenger_name":"Test"}`, "valid-key", true},
		{"POST", "/me/bookings", `{"trip_id":1} {"trip_id":2}`, "valid-key", true},
		{"POST", "/me/bookings", strings.Repeat("x", 4097), "valid-key", true},
		{"GET", "/me/bookings?limit=0", "", "", true},
		{"GET", "/me/bookings?limit=101", "", "", true},
		{"GET", "/me/passengers?limit=NaN", "", "", true},
		{"PUT", "/me/passengers/0", `{"full_name":"Test"}`, "", true},
		{"POST", "/me/passengers", `{"full_name":""}`, "", true},
		{"POST", "/me/passengers", `{"full_name":"Test","passport":"test"}`, "", true},
		{"GET", "/trips/2147483648/availability?boarding_stop_id=1&alighting_stop_id=2", "", "", false},
		{"GET", "/trips/1/availability?boarding_stop_id=-1&alighting_stop_id=2", "", "", false},
		{"GET", "/me/bookings", "", "", false},
		{"GET", "/me/passengers", "", "", false},
		{"POST", "/me/bookings", `{}`, "valid-key", false},
	} {
		r := gin.New()
		public := r.Group("")
		authorized := r.Group("")
		if tc.owner {
			authorized.Use(func(c *gin.Context) { c.Set("userID", 1) })
		}
		h.Register(public, authorized)
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		if tc.key != "" {
			req.Header.Set("Idempotency-Key", tc.key)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		expected := 400
		if !tc.owner && strings.HasPrefix(tc.path, "/me/") {
			expected = 401
		}
		if w.Code != expected {
			t.Fatalf("%s %s got %d: %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}
func TestBookingErrorsDoNotExposeDatabaseOrPassengerDetails(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{errors.New("database-secret passenger-name"), 500, "internal_error"},
		{context.DeadlineExceeded, 503, "booking_timeout"},
		{&pgconn.PgError{Code: "57014", Message: "private SQL"}, 503, "booking_timeout"},
		{&booking.Error{Status: 409, Code: "sold_out", Message: "Свободных мест нет"}, 409, "sold_out"},
	} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		bookingError(c, tc.err)
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if w.Code != tc.status || body["code"] != tc.code || strings.Contains(w.Body.String(), "private") || strings.Contains(w.Body.String(), "database-secret") {
			t.Fatalf("unsafe error: %d %s", w.Code, w.Body.String())
		}
	}
}
