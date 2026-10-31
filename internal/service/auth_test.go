package service

import (
	"context"
	"corpord-api/internal/config"
	"corpord-api/internal/logger"
	"corpord-api/internal/repository/pg"
	"corpord-api/internal/token"
	"corpord-api/model"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type authRepositoryStub struct {
	pg.AuthRepository
	user        *model.UserDB
	lookupError error
	created     *model.UserCreate
	createError error
}

func (r *authRepositoryStub) GetUserByEmail(context.Context, string) (*model.UserDB, error) {
	return r.user, r.lookupError
}
func (r *authRepositoryStub) GetUserByID(context.Context, int) (*model.UserDB, error) {
	return r.user, r.lookupError
}
func (r *authRepositoryStub) CreateUser(_ context.Context, u *model.UserCreate) (int, error) {
	if r.createError != nil {
		return 0, r.createError
	}
	r.created = u
	r.user = &model.UserDB{ID: 42, Email: u.Email, Name: u.Name, PasswordHash: u.Password, Role: model.RoleUser}
	return 42, nil
}

type refreshRepositoryStub struct {
	pg.RefreshTokenRepository
	oldSession *model.RefreshSession
	saved      []*model.RefreshSession
	rotated    []*model.RefreshSession
	rotateErr  error
	revoked    bool
}

func (r *refreshRepositoryStub) FindByHash(context.Context, string) (*model.RefreshSession, error) {
	if r.oldSession == nil || r.revoked {
		return nil, pg.ErrRefreshTokenNotFound
	}
	return r.oldSession, nil
}
func (r *refreshRepositoryStub) Save(_ context.Context, session *model.RefreshSession) error {
	r.saved = append(r.saved, session)
	return nil
}
func (r *refreshRepositoryStub) RefreshToken(_ context.Context, _ string, session *model.RefreshSession) error {
	if r.rotateErr != nil {
		return r.rotateErr
	}
	if r.revoked {
		return pg.ErrRefreshTokenNotFound
	}
	r.revoked = true
	r.rotated = append(r.rotated, session)
	return nil
}

func authTestManager() token.Manager {
	return token.NewManager(&config.JWT{
		Secret: "auth-tests-secret-at-least-32-bytes-long", SigningAlgorithm: "HS256",
		AccessTokenTTL: time.Minute, RefreshTokenTTL: time.Hour,
	})
}

func newAuthForTest(repo pg.AuthRepository, refresh pg.RefreshTokenRepository) *auth {
	return &auth{logger: &logger.Logger{SugaredLogger: zap.NewNop().Sugar()}, token: authTestManager(), authRepo: repo, refreshRepo: refresh}
}

func TestRegisterRejectsMissingOrInvalidPasswordBeforeRepository(t *testing.T) {
	empty, long := "", strings.Repeat("a", 73)
	for _, input := range []*model.UserCreate{nil, {}, {Password: &empty}, {Password: &long}} {
		_, err := newAuthForTest(nil, nil).Register(context.Background(), input, "", "")
		if !errors.Is(err, ErrInvalidPass) {
			t.Fatalf("Register(%+v): want invalid password, got %v", input, err)
		}
	}
}

func TestRegisterHashesCopyWithoutOverwritingPlaintextInput(t *testing.T) {
	password := "a-valid-password"
	input := &model.UserCreate{Email: "new@example.com", Name: "New", Password: &password}
	repo, refresh := &authRepositoryStub{}, &refreshRepositoryStub{}
	if _, err := newAuthForTest(repo, refresh).Register(context.Background(), input, "", ""); err != nil {
		t.Fatal(err)
	}
	if *input.Password != "a-valid-password" {
		t.Fatal("registration overwrote caller's password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*repo.created.Password), []byte(password)); err != nil {
		t.Fatalf("password persisted without valid bcrypt hash: %v", err)
	}
	if len(refresh.saved) != 1 {
		t.Fatalf("want exactly one refresh session, got %d", len(refresh.saved))
	}
}

func TestRegisterConcurrentEmailConflictReturnsEmailExists(t *testing.T) {
	password := "valid-password"
	repo := &authRepositoryStub{createError: &pgconn.PgError{Code: pg.ErrorCodeUniqueViolation}}
	_, err := newAuthForTest(repo, nil).Register(context.Background(), &model.UserCreate{Email: "existing@example.com", Name: "New", Password: &password}, "", "")
	if !errors.Is(err, ErrEmailExists) {
		t.Fatalf("concurrent duplicate registration was not a conflict: %v", err)
	}
}

func TestLoginRejectsSSOOrWrongPasswordWithoutPanic(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	passwordHash := string(hash)
	for _, u := range []*model.UserDB{nil, {ID: 1, PasswordHash: nil}, {ID: 1, PasswordHash: &passwordHash}} {
		_, err := newAuthForTest(&authRepositoryStub{user: u}, nil).Login(context.Background(), model.UserLogin{Email: "sso@example.com", Password: "wrong-password"}, "", "")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("Login(%+v): want invalid credentials, got %v", u, err)
		}
	}
}

func TestAuthenticateUsesCurrentUserRoleAndRejectsDeletedUsers(t *testing.T) {
	repo := &authRepositoryStub{user: &model.UserDB{ID: 42, Email: "current@example.com", Role: model.RoleUser}}
	s := newAuthForTest(repo, nil)
	raw, err := s.token.Generate(token.GenerateParams{UserID: 42, Email: "old@example.com", Role: model.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := s.Authenticate(context.Background(), raw)
	if err != nil || claims.Role != model.RoleUser || claims.Email != "current@example.com" {
		t.Fatalf("stale claims retained: %+v, %v", claims, err)
	}
	now := time.Now()
	repo.user.DeletedAt = &now
	if _, err := s.Authenticate(context.Background(), raw); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("deleted user accepted: %v", err)
	}
	repo.user = nil
	if _, err := s.Authenticate(context.Background(), raw); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("missing user accepted: %v", err)
	}
}

func TestRefreshPersistsOnlyThroughAtomicRotation(t *testing.T) {
	for _, rotateErr := range []error{nil, pg.ErrRefreshTokenNotFound, errors.New("insert failed")} {
		refresh := &refreshRepositoryStub{oldSession: &model.RefreshSession{UserID: 42, ExpiresAt: time.Now().Add(time.Hour)}, rotateErr: rotateErr}
		s := newAuthForTest(&authRepositoryStub{user: &model.UserDB{ID: 42, Role: model.RoleUser}}, refresh)
		tokens, err := s.Refresh(context.Background(), "old-refresh", "", "")
		if len(refresh.saved) != 0 {
			t.Fatal("refresh inserted a session outside the rotation transaction")
		}
		if rotateErr != nil {
			if err == nil || tokens != nil || len(refresh.rotated) != 0 {
				t.Fatalf("failed rotation returned credentials or persisted session: %+v %v", tokens, err)
			}
			continue
		}
		if err != nil || tokens == nil || len(refresh.rotated) != 1 {
			t.Fatalf("rotation: %+v %v, sessions=%d", tokens, err, len(refresh.rotated))
		}
		hash := sha256.Sum256([]byte(tokens.RefreshToken))
		if refresh.rotated[0].TokenHash != hex.EncodeToString(hash[:]) || refresh.rotated[0].UserAgent == "" || refresh.rotated[0].IP == "" {
			t.Fatal("rotation persisted incorrect hash or empty metadata")
		}
		if _, err := s.Refresh(context.Background(), "old-refresh", "", ""); !errors.Is(err, ErrInvalidRefreshToken) {
			t.Fatalf("old token replay accepted: %v", err)
		}
	}
}
