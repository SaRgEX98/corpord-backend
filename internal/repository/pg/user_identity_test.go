package pg

import (
	"context"
	"corpord-api/internal/logger"
	"corpord-api/model"
	"corpord-api/pkg/dbx"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

func ssoIdentityTestDB(t *testing.T) (*sqlx.DB, UserIdentitiesRepository) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin := sqlx.NewDb(stdlib.OpenDB(*cfg), "pgx")
	schema := "sso_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	cfg.RuntimeParams["search_path"] = schema
	db := sqlx.NewDb(stdlib.OpenDB(*cfg), "pgx")
	db.SetMaxOpenConns(12)
	t.Cleanup(func() {
		db.Close()
		if _, err := admin.Exec("DROP SCHEMA " + schema + " CASCADE"); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	// Apply the actual user and identity migrations, including nullable passwords
	// and the later identity metadata migration, in an isolated schema.
	for _, filename := range []string{
		"20250926205312_new_user_table.sql", "20250928135521_add_roles_table.sql", "20250928152124_add_deleted_at_to_users.sql",
		"20251201234459_users_edit_password.sql", "20251202001405_user_identity.sql", "20251227023001_user_identities_add_updated_at_column.sql",
	} {
		content, err := os.ReadFile("../../../db/migrations/" + filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(strings.Split(string(content), "-- +goose Down")[0]); err != nil {
			t.Fatalf("migration %s: %v", filename, err)
		}
	}
	return db, NewUserIdentitiesRepo(&logger.Logger{SugaredLogger: zap.NewNop().Sugar()}, dbx.NewQueryBuilder(db))
}

func TestSSOUserAndIdentityPostgresCommitAndRollbackTogether(t *testing.T) {
	db, repo := ssoIdentityTestDB(t)
	ctx := context.Background()
	identity := &model.UserIdentity{ID: uuid.New(), Provider: "google", ProviderID: "provider-user-123"}
	u, err := repo.CreateUserWithIdentity(ctx, &model.UserCreate{Email: "first@example.com", Name: "First"}, identity)
	if err != nil {
		t.Fatal(err)
	}
	if u.ID == 0 || u.PasswordHash != nil || u.Role != model.RoleUser || identity.UserID != u.ID {
		t.Fatalf("invalid created SSO account: %+v identity=%+v", u, identity)
	}
	found, err := repo.GetProviderByID(ctx, identity.Provider, identity.ProviderID)
	if err != nil || found.UserID != u.ID {
		t.Fatalf("identity lookup: %+v %v", found, err)
	}
	for _, tc := range []struct{ email, provider, providerID string }{
		{"orphan@example.com", "google", identity.ProviderID},
		{"first@example.com", "yandex", "another-provider-user"},
	} {
		_, err := repo.CreateUserWithIdentity(ctx, &model.UserCreate{Email: tc.email, Name: "Duplicate"}, &model.UserIdentity{ID: uuid.New(), Provider: tc.provider, ProviderID: tc.providerID})
		if !IsPgError(err, ErrorCodeUniqueViolation) {
			t.Fatalf("expected duplicate error, got %v", err)
		}
	}
	var users, identities int
	if err := db.Get(&users, "SELECT count(*) FROM users"); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&identities, "SELECT count(*) FROM user_identities"); err != nil {
		t.Fatal(err)
	}
	if users != 1 || identities != 1 {
		t.Fatalf("failed transaction left an orphan user/identity: users=%d identities=%d", users, identities)
	}
}

func TestSSOConcurrentIdentityCreationPostgresLeavesOneAccount(t *testing.T) {
	db, repo := ssoIdentityTestDB(t)
	const attempts = 8
	start, results := make(chan struct{}), make(chan error, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-start
			_, err := repo.CreateUserWithIdentity(context.Background(), &model.UserCreate{Email: fmt.Sprintf("user-%d@example.com", n), Name: "Concurrent"}, &model.UserIdentity{ID: uuid.New(), Provider: "yandex", ProviderID: "shared-provider-id"})
			results <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	winners := 0
	for err := range results {
		if err == nil {
			winners++
		} else if !IsPgError(err, ErrorCodeUniqueViolation) {
			t.Errorf("unexpected create error: %v", err)
		}
	}
	var users, identities int
	if err := db.Get(&users, "SELECT count(*) FROM users"); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&identities, "SELECT count(*) FROM user_identities"); err != nil {
		t.Fatal(err)
	}
	if winners != 1 || users != 1 || identities != 1 {
		t.Fatalf("concurrent callback left accounts: winners=%d users=%d identities=%d", winners, users, identities)
	}
}
