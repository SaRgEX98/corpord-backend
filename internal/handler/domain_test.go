package handler

import (
	"context"
	"corpord-api/internal/logger"
	"corpord-api/internal/repository/pg"
	"corpord-api/internal/service"
	"corpord-api/model"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
)

type domainTripStub struct {
	service.Trip
	byID   func(context.Context, int) (*model.TripResponse, error)
	update func(context.Context, *model.TripUpdate) error
}

func (s *domainTripStub) ById(ctx context.Context, id int) (*model.TripResponse, error) {
	return s.byID(ctx, id)
}
func (s *domainTripStub) Update(ctx context.Context, input *model.TripUpdate) error {
	return s.update(ctx, input)
}

func domainLogger() *logger.Logger { return &logger.Logger{SugaredLogger: zap.NewNop().Sugar()} }

func TestTripHandlerUsesOneJSONAndCorrectHTTPStatus(t *testing.T) {
	cases := []struct {
		name, path, body string
		failure          error
		want             int
		calls            int
	}{
		{"invalid ID", "bad", `{}`, nil, 400, 0},
		{"negative ID", "-1", `{}`, nil, 400, 0},
		{"invalid body", "1", `{`, nil, 400, 0},
		{"validation failure", "1", `{}`, &model.ValidationError{Message: "no fields to update"}, 400, 1},
		{"missing record", "1", `{}`, pg.ErrNotFound, 404, 1},
		{"database failure", "1", `{}`, errors.New("SQL includes private database details"), 500, 1},
		{"successful start time", "1", `{"id":999,"start_time":"2026-12-01T10:00:00Z"}`, nil, 200, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			stub := &domainTripStub{update: func(_ context.Context, input *model.TripUpdate) error {
				calls++
				if input.ID != 1 {
					t.Errorf("path ID replaced: %d", input.ID)
				}
				return tc.failure
			}}
			r := gin.New()
			r.PUT("/trips/:id", NewTrip(domainLogger(), stub).Update)
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPut, "/trips/"+tc.path, strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(response, request)
			if response.Code != tc.want || calls != tc.calls {
				t.Fatalf("status=%d calls=%d body=%s", response.Code, calls, response.Body)
			}
			if !json.Valid(response.Body.Bytes()) {
				t.Fatalf("response must contain exactly one JSON: %s", response.Body)
			}
			if strings.Contains(response.Body.String(), "private database") {
				t.Fatalf("internal error leaked: %s", response.Body)
			}
		})
	}
}

func TestTripByIDReturns404OnlyForMissingRecord(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want int
	}{{sql.ErrNoRows, 404}, {errors.New("connection refused"), 500}} {
		stub := &domainTripStub{byID: func(context.Context, int) (*model.TripResponse, error) { return nil, tc.err }}
		r := gin.New()
		r.GET("/trips/:id", NewTrip(domainLogger(), stub).ByID)
		response := httptest.NewRecorder()
		r.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/trips/42", nil))
		if response.Code != tc.want {
			t.Fatalf("error %v: got %d body=%s", tc.err, response.Code, response.Body)
		}
	}
}

func TestDomainErrorClassifiesConstraintsWithoutLeakingSQL(t *testing.T) {
	for _, tc := range []struct {
		code string
		want int
	}{{"23505", 409}, {"23503", 409}, {"23514", 400}, {"23502", 400}, {"22003", 400}, {"42P01", 500}} {
		r := gin.New()
		r.GET("/", func(c *gin.Context) {
			writeDomainError(c, &pgconn.PgError{Code: tc.code, Message: "private_table sensitive value"})
		})
		response := httptest.NewRecorder()
		r.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
		if response.Code != tc.want || strings.Contains(response.Body.String(), "private_table") {
			t.Fatalf("code %s: status=%d body=%s", tc.code, response.Code, response.Body)
		}
	}
}

type domainDriverStatusStub struct {
	service.DriverStatus
	createCalls int
}

func (s *domainDriverStatusStub) ById(context.Context, int) (model.DriverStatus, error) {
	return model.DriverStatus{}, sql.ErrNoRows
}
func (s *domainDriverStatusStub) Create(context.Context, *model.DriverStatus) error {
	s.createCalls++
	return nil
}
func (s *domainDriverStatusStub) All(context.Context) ([]model.DriverStatus, error) {
	return nil, errors.New("DB unavailable")
}

func TestDriverStatusHandlerStopsAfterErrors(t *testing.T) {
	stub := &domainDriverStatusStub{}
	handler := NewDriverStatus(domainLogger(), stub)
	r := gin.New()
	r.GET("/status/:id", handler.ById)
	r.GET("/status", handler.All)
	r.POST("/status", handler.Create)
	for _, tc := range []struct {
		method, path, body string
		want               int
	}{{"GET", "/status/1", "", 404}, {"GET", "/status", "", 500}, {"POST", "/status", "{", 400}} {
		response := httptest.NewRecorder()
		r.ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
		if response.Code != tc.want || !json.Valid(response.Body.Bytes()) {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, response.Code, response.Body)
		}
	}
	if stub.createCalls != 0 {
		t.Fatalf("service called after bad JSON: %d", stub.createCalls)
	}
}
