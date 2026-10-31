package pg

import (
	"context"
	"corpord-api/model"
	"corpord-api/pkg/dbx"
	"errors"
	"testing"
)

// Exercise the actual migrations and PostgreSQL role relation. GetByID backs
// /users/me, while Create/Update/GetAll back the admin user-management responses.
func TestUserRepositoryPostgresReturnsCurrentRoles(t *testing.T) {
	db, log := domainTestDB(t, true)
	repo := NewUserRepository(log, dbx.NewQueryBuilder(db))
	ctx := context.Background()
	storedHash := "test-password-hash"
	first, err := repo.Create(ctx, &model.UserCreate{Email: "first@example.invalid", Name: "First", Password: &storedHash})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID <= 0 || first.Role != model.RoleUser {
		t.Fatalf("create must return database-default role user: %+v", first)
	}
	second, err := repo.Create(ctx, &model.UserCreate{Email: "second@example.invalid", Name: "Second", Password: &storedHash})
	if err != nil {
		t.Fatal(err)
	}
	if second.Role != model.RoleUser {
		t.Fatalf("second create returned missing/wrong role: %+v", second)
	}

	// Include moderator because it exists in the role migration even though
	// only exact admin may access the administrative API.
	for _, role := range []string{model.RoleUser, model.RoleAdmin, "moderator", model.RoleUser} {
		t.Run(role, func(t *testing.T) {
			if _, err := db.ExecContext(ctx, "UPDATE users SET role_id=(SELECT id FROM roles WHERE name=$1) WHERE id=$2", role, first.ID); err != nil {
				t.Fatal(err)
			}
			me, err := repo.GetByID(ctx, first.ID)
			if err != nil || me.Role != role || me.ID != first.ID {
				t.Fatalf("profile must return current persisted role %q: %+v err=%v", role, me, err)
			}
			byEmail, err := repo.GetByEmail(ctx, first.Email)
			if err != nil || byEmail.Role != role || byEmail.ID != first.ID || byEmail.PasswordHash == nil || *byEmail.PasswordHash != storedHash {
				t.Fatalf("email lookup must preserve role and password hash: %+v err=%v", byEmail, err)
			}
			all, err := repo.GetAll(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(all) != 2 || all[0].ID != first.ID || all[0].Role != role || all[1].ID != second.ID || all[1].Role != model.RoleUser {
				t.Fatalf("list must keep each user's own role and stable ordering: %+v", all)
			}
			name := "Updated " + role
			updated, err := repo.Update(ctx, first.ID, &model.UserUpdate{Name: &name})
			if err != nil || updated.Role != role || updated.Name != name || updated.Email != first.Email {
				t.Fatalf("partial update must return role and preserve omitted fields: %+v err=%v", updated, err)
			}
		})
	}
	if _, err := repo.GetByID(ctx, -1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing id contract changed: %v", err)
	}
	if _, err := repo.GetByEmail(ctx, "missing@example.invalid"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing email contract changed: %v", err)
	}
}
