package service

import (
	"context"
	"corpord-api/internal/logger"
	"corpord-api/internal/repository/pg"
	"corpord-api/model"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
)

type domainUserRepo struct {
	pg.UserRepository
	failure error
}

func (r *domainUserRepo) GetByID(context.Context, int) (*model.UserResponse, error) {
	return nil, r.failure
}

func TestUserLookupPreservesDatabaseFailure(t *testing.T) {
	log := &logger.Logger{SugaredLogger: zap.NewNop().Sugar()}
	unavailable := errors.New("database unavailable")
	for _, tc := range []struct{ repoErr, want error }{{unavailable, unavailable}, {pg.ErrNotFound, ErrUserNotFound}} {
		_, err := NewUser(log, &domainUserRepo{failure: tc.repoErr}).GetByID(context.Background(), 1)
		if !errors.Is(err, tc.want) {
			t.Fatalf("lookup error %v mapped to %v, want %v", tc.repoErr, err, tc.want)
		}
	}
}

type domainTripRepo struct {
	pg.Trip
	current *model.TripResponse
	updated bool
}

func (r *domainTripRepo) ByID(context.Context, int) (*model.TripResponse, error) {
	return r.current, nil
}
func (r *domainTripRepo) Update(context.Context, *model.TripUpdate) error {
	r.updated = true
	return nil
}

func TestTripPartialTimeUpdateChecksExistingInterval(t *testing.T) {
	start := time.Date(2026, 12, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	for _, tc := range []struct {
		name  string
		start time.Time
		valid bool
	}{{"within interval", start.Add(10 * time.Minute), true}, {"after existing end", end.Add(time.Minute), false}} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &domainTripRepo{current: &model.TripResponse{ID: 1, StartTime: start, EndTime: &end}}
			err := NewTrip(nil, repo).Update(context.Background(), &model.TripUpdate{ID: 1, StartTime: &tc.start})
			var validation *model.ValidationError
			if tc.valid && (err != nil || !repo.updated) {
				t.Fatalf("valid partial update rejected: %v", err)
			}
			if !tc.valid && (!errors.As(err, &validation) || repo.updated) {
				t.Fatalf("invalid interval reached mutation: err=%v updated=%v", err, repo.updated)
			}
		})
	}
}
