package pg

import (
	"context"
	"corpord-api/internal/logger"
	"corpord-api/model"
	"corpord-api/pkg/dbx"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

// Integration tests require a disposable PostgreSQL database. Every test creates
// and drops its own schema; TEST_DATABASE_URL must never point to production.
func refreshTestDB(t *testing.T, withUniqueIndex bool) (*sqlx.DB, RefreshTokenRepository) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin := sqlx.NewDb(stdlib.OpenDB(*cfg), "pgx")
	schema := "auth_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	cfg.RuntimeParams["search_path"] = schema
	db := sqlx.NewDb(stdlib.OpenDB(*cfg), "pgx")
	db.SetMaxOpenConns(20)
	t.Cleanup(func() {
		db.Close()
		if _, err := admin.Exec("DROP SCHEMA " + schema + " CASCADE"); err != nil {
			t.Errorf("drop isolated schema: %v", err)
		}
		admin.Close()
	})
	_, err = db.Exec(`
		CREATE TABLE users (id integer PRIMARY KEY);
		INSERT INTO users (id) VALUES (1), (2);
		CREATE TABLE refresh_tokens (
			id uuid PRIMARY KEY,
			user_id integer NOT NULL REFERENCES users(id),
			token_hash text NOT NULL,
			expires_at timestamp NOT NULL,
			created_at timestamp NOT NULL DEFAULT now(),
			updated_at timestamp NOT NULL DEFAULT now(),
			revoked boolean NOT NULL DEFAULT false,
			ip inet NOT NULL,
			user_agent text NOT NULL
		)`)
	if err != nil {
		t.Fatal(err)
	}
	if withUniqueIndex {
		applyRefreshUniqueMigration(t, db)
	}
	log := &logger.Logger{SugaredLogger: zap.NewNop().Sugar()}
	return db, NewRefreshTokenRepo(log, dbx.NewQueryBuilder(db))
}

func applyRefreshUniqueMigration(t *testing.T, db *sqlx.DB) {
	t.Helper()
	content, err := os.ReadFile("../../../db/migrations/20261031000100_unique_refresh_token_hash.sql")
	if err != nil {
		t.Fatal(err)
	}
	up := strings.Split(string(content), "-- +goose Down")[0]
	if _, err := db.Exec(up); err != nil {
		t.Fatalf("apply refresh migration: %v", err)
	}
}

func testRefreshSession(userID int, hash string) *model.RefreshSession {
	return &model.RefreshSession{ID: uuid.New(), UserID: userID, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour), IP: "127.0.0.1", UserAgent: "integration-test"}
}

func TestRefreshRotationPostgresConsumesTokenOnce(t *testing.T) {
	db, repo := refreshTestDB(t, true)
	ctx := context.Background()
	old, next := testRefreshSession(1, "old"), testRefreshSession(1, "next")
	if err := repo.Save(ctx, old); err != nil {
		t.Fatal(err)
	}
	if err := repo.RefreshToken(ctx, "old", next); err != nil {
		t.Fatal(err)
	}
	if err := repo.RefreshToken(ctx, "old", testRefreshSession(1, "replayed")); !errors.Is(err, ErrRefreshTokenNotFound) {
		t.Fatalf("replay should fail, got %v", err)
	}
	var total, active int
	if err := db.QueryRow("SELECT count(*), count(*) FILTER (WHERE NOT revoked) FROM refresh_tokens").Scan(&total, &active); err != nil {
		t.Fatal(err)
	}
	if total != 2 || active != 1 {
		t.Fatalf("want old revoked and one replacement, total=%d active=%d", total, active)
	}
	if err := repo.Revoke(ctx, next.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.RefreshToken(ctx, "next", testRefreshSession(1, "after-logout")); !errors.Is(err, ErrRefreshTokenNotFound) {
		t.Fatalf("revoked token accepted: %v", err)
	}
}

func TestRefreshRotationPostgresConcurrentUseHasOneWinner(t *testing.T) {
	db, repo := refreshTestDB(t, true)
	ctx := context.Background()
	if err := repo.Save(ctx, testRefreshSession(1, "shared")); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 16)
	var wg sync.WaitGroup
	for i := 0; i < cap(results); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- repo.RefreshToken(ctx, "shared", testRefreshSession(1, uuid.NewString()))
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	winners := 0
	for err := range results {
		if err == nil {
			winners++
		} else if !errors.Is(err, ErrRefreshTokenNotFound) {
			t.Errorf("unexpected concurrent rotation error: %v", err)
		}
	}
	var total int
	if err := db.Get(&total, "SELECT count(*) FROM refresh_tokens"); err != nil {
		t.Fatal(err)
	}
	if winners != 1 || total != 2 {
		t.Fatalf("want one winner and exactly two rows, winners=%d total=%d", winners, total)
	}
}

func TestLogoutPostgresConcurrentRotationHasOnlyOneWinner(t *testing.T) {
	_, repo := refreshTestDB(t, true)
	ctx := context.Background()
	for i := 0; i < 12; i++ {
		old := testRefreshSession(1, uuid.NewString())
		if err := repo.Save(ctx, old); err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		results := make(chan error, 2)
		go func() {
			<-start
			results <- repo.RefreshToken(ctx, old.TokenHash, testRefreshSession(1, uuid.NewString()))
		}()
		go func() { <-start; results <- repo.Revoke(ctx, old.ID) }()
		close(start)
		winners := 0
		for n := 0; n < 2; n++ {
			err := <-results
			if err == nil {
				winners++
			} else if !errors.Is(err, ErrRefreshTokenNotFound) {
				t.Fatal(err)
			}
		}
		if winners != 1 {
			t.Fatalf("logout and rotation both accepted the same credential, winners=%d", winners)
		}
		if err := repo.Revoke(ctx, old.ID); !errors.Is(err, ErrRefreshTokenNotFound) {
			t.Fatalf("stale logout reported success: %v", err)
		}
	}
}

func TestRefreshRotationPostgresRollsBackWhenReplacementInsertFails(t *testing.T) {
	db, repo := refreshTestDB(t, true)
	ctx := context.Background()
	for _, session := range []*model.RefreshSession{testRefreshSession(1, "old"), testRefreshSession(1, "collision")} {
		if err := repo.Save(ctx, session); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.RefreshToken(ctx, "old", testRefreshSession(1, "collision")); err == nil {
		t.Fatal("duplicate replacement hash should fail")
	}
	if _, err := repo.FindByHash(ctx, "old"); err != nil {
		t.Fatalf("failed transaction consumed old session: %v", err)
	}
	var total int
	if err := db.Get(&total, "SELECT count(*) FROM refresh_tokens"); err != nil || total != 2 {
		t.Fatalf("failed transaction left extra sessions: total=%d err=%v", total, err)
	}
	if err := repo.RefreshToken(ctx, "old", testRefreshSession(1, "retry")); err != nil {
		t.Fatalf("retry after rollback failed: %v", err)
	}
}

func TestRefreshRotationPostgresRejectsExpiredAndWrongUser(t *testing.T) {
	_, repo := refreshTestDB(t, true)
	ctx := context.Background()
	expired := testRefreshSession(1, "expired")
	expired.ExpiresAt = time.Now().Add(-time.Second)
	for _, session := range []*model.RefreshSession{expired, testRefreshSession(1, "other-user")} {
		if err := repo.Save(ctx, session); err != nil {
			t.Fatal(err)
		}
	}
	for _, hash := range []string{"expired", "other-user"} {
		replacement := testRefreshSession(2, uuid.NewString())
		if hash == "expired" {
			replacement.UserID = 1
		}
		if err := repo.RefreshToken(ctx, hash, replacement); !errors.Is(err, ErrRefreshTokenNotFound) {
			t.Fatalf("invalid rotation of %q accepted: %v", hash, err)
		}
	}
}

func TestLogoutAllPostgresConcurrentRotationLeavesNoActiveSession(t *testing.T) {
	db, repo := refreshTestDB(t, true)
	ctx := context.Background()
	for i := 0; i < 12; i++ {
		hash := uuid.NewString()
		if err := repo.Save(ctx, testRefreshSession(1, hash)); err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		rotation := make(chan error, 1)
		logout := make(chan error, 1)
		go func() { <-start; rotation <- repo.RefreshToken(ctx, hash, testRefreshSession(1, uuid.NewString())) }()
		go func() { <-start; logout <- repo.RevokeAllByUser(ctx, 1) }()
		close(start)
		if err := <-rotation; err != nil && !errors.Is(err, ErrRefreshTokenNotFound) {
			t.Fatal(err)
		}
		if err := <-logout; err != nil {
			t.Fatal(err)
		}
		var active int
		if err := db.Get(&active, "SELECT count(*) FROM refresh_tokens WHERE NOT revoked"); err != nil || active != 0 {
			t.Fatalf("refresh survived logout-all: active=%d err=%v", active, err)
		}
	}
}

func TestLogoutAllPostgresRevokesOnlyRequestedUser(t *testing.T) {
	_, repo := refreshTestDB(t, true)
	ctx := context.Background()
	for _, session := range []*model.RefreshSession{testRefreshSession(1, "first"), testRefreshSession(1, "second"), testRefreshSession(2, "other-user")} {
		if err := repo.Save(ctx, session); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.RevokeAllByUser(ctx, 1); err != nil {
		t.Fatal(err)
	}
	for _, hash := range []string{"first", "second"} {
		if _, err := repo.FindByHash(ctx, hash); !errors.Is(err, ErrRefreshTokenNotFound) {
			t.Fatalf("logout-all did not revoke %q: %v", hash, err)
		}
	}
	if _, err := repo.FindByHash(ctx, "other-user"); err != nil {
		t.Fatalf("logout-all affected another user: %v", err)
	}
}

func TestRefreshHashMigrationRevokesDuplicatesAndEnforcesUniqueness(t *testing.T) {
	db, repo := refreshTestDB(t, false)
	ctx := context.Background()
	for _, hash := range []string{"duplicate", "duplicate", "unique"} {
		if err := repo.Save(ctx, testRefreshSession(1, hash)); err != nil {
			t.Fatal(err)
		}
	}
	applyRefreshUniqueMigration(t, db)
	var duplicates, activeDuplicates, activeUnique int
	if err := db.QueryRow(`SELECT count(*) FILTER (WHERE token_hash='duplicate'),
		count(*) FILTER (WHERE token_hash='duplicate' AND NOT revoked),
		count(*) FILTER (WHERE token_hash='unique' AND NOT revoked) FROM refresh_tokens`).Scan(&duplicates, &activeDuplicates, &activeUnique); err != nil {
		t.Fatal(err)
	}
	if duplicates != 1 || activeDuplicates != 0 || activeUnique != 1 {
		t.Fatalf("migration corrupted sessions: duplicate=%d active_duplicate=%d active_unique=%d", duplicates, activeDuplicates, activeUnique)
	}
	if err := repo.Save(ctx, testRefreshSession(1, "duplicate")); err == nil {
		t.Fatal("unique hash index did not reject duplicate")
	}
}
