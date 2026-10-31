package service

import (
	"context"
	"corpord-api/internal/repository/pg"
	"corpord-api/internal/sso"
	"corpord-api/model"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/oauth2"
)

type ssoAccountRepo struct {
	pg.AuthRepository
	existing      *model.UserDB
	byID          *model.UserDB
	emailLookups  int
	onEmailLookup func()
}

func (r *ssoAccountRepo) GetUserByEmail(context.Context, string) (*model.UserDB, error) {
	r.emailLookups++
	if r.onEmailLookup != nil {
		r.onEmailLookup()
	}
	return r.existing, nil
}

func TestSSOResolvesIdentityCommittedBetweenIdentityAndEmailLookup(t *testing.T) {
	info := &sso.UserInfo{Provider: "google", ProviderID: "stable-id", Email: "user@example.com"}
	user := &model.UserDB{ID: 7, Email: info.Email}
	identities := &ssoIdentityRepo{}
	accounts := &ssoAccountRepo{existing: user, byID: user}
	accounts.onEmailLookup = func() {
		identities.identity = &model.UserIdentity{UserID: user.ID, Provider: info.Provider, ProviderID: info.ProviderID}
	}
	s := &auth{authRepo: accounts, userIdentity: identities}
	u, err := s.findOrCreateUserFromSSO(context.Background(), info)
	if err != nil || u == nil || u.ID != user.ID || identities.createCalls != 0 {
		t.Fatalf("concurrent identical login failed: user=%+v err=%v creates=%d", u, err, identities.createCalls)
	}
}
func (r *ssoAccountRepo) GetUserByID(context.Context, int) (*model.UserDB, error) { return r.byID, nil }

type ssoIdentityRepo struct {
	pg.UserIdentitiesRepository
	identity      *model.UserIdentity
	createdUser   *model.UserDB
	createErr     error
	createCalls   int
	afterConflict *model.UserIdentity
}

func (r *ssoIdentityRepo) GetProviderByID(context.Context, string, string) (*model.UserIdentity, error) {
	if r.identity == nil {
		return nil, pg.ErrIdentityNotFound
	}
	return r.identity, nil
}
func (r *ssoIdentityRepo) CreateUserWithIdentity(context.Context, *model.UserCreate, *model.UserIdentity) (*model.UserDB, error) {
	r.createCalls++
	if r.afterConflict != nil {
		r.identity = r.afterConflict
	}
	return r.createdUser, r.createErr
}

func TestSSONeverLinksAnExistingAccountByEmail(t *testing.T) {
	for _, provider := range []string{"google", "yandex"} {
		accounts := &ssoAccountRepo{existing: &model.UserDB{ID: 1, Email: "existing@example.com"}}
		identities := &ssoIdentityRepo{}
		s := &auth{authRepo: accounts, userIdentity: identities}
		u, err := s.findOrCreateUserFromSSO(context.Background(), &sso.UserInfo{Provider: provider, ProviderID: "new-identity", Email: "existing@example.com", EmailVerified: true})
		if !errors.Is(err, ErrSSOAccountExists) || u != nil || identities.createCalls != 0 {
			t.Fatalf("existing account linked to %s: user=%+v error=%v creates=%d", provider, u, err, identities.createCalls)
		}
	}
}

func TestSSOUsesStableIdentityAndResolvesConcurrentCreate(t *testing.T) {
	info := &sso.UserInfo{Provider: "google", ProviderID: "stable-id", Email: "changed@example.com"}
	for _, concurrent := range []bool{false, true} {
		accounts := &ssoAccountRepo{byID: &model.UserDB{ID: 7, Email: "original@example.com"}}
		identities := &ssoIdentityRepo{}
		identity := &model.UserIdentity{UserID: 7, Provider: "google", ProviderID: "stable-id"}
		if concurrent {
			identities.afterConflict = identity
			identities.createErr = &pgconn.PgError{Code: pg.ErrorCodeUniqueViolation}
		} else {
			identities.identity = identity
		}
		s := &auth{authRepo: accounts, userIdentity: identities}
		u, err := s.findOrCreateUserFromSSO(context.Background(), info)
		if err != nil || u == nil || u.ID != 7 {
			t.Fatalf("stable identity login failed: user=%+v err=%v", u, err)
		}
		if !concurrent && accounts.emailLookups != 0 {
			t.Fatal("existing identity depended on a mutable email")
		}
	}
}

type ssoInvalidProvider struct {
	info     *sso.UserInfo
	verifier string
	deadline time.Time
}

func (p *ssoInvalidProvider) AuthURL(string, string) string { return "" }
func (p *ssoInvalidProvider) Exchange(ctx context.Context, code, verifier string) (*oauth2.Token, error) {
	p.verifier = verifier
	p.deadline, _ = ctx.Deadline()
	return &oauth2.Token{AccessToken: "access"}, nil
}
func (p *ssoInvalidProvider) GetUserInfo(context.Context, *oauth2.Token) (*sso.UserInfo, error) {
	return p.info, nil
}

func TestSSOLoginRejectsIncompleteAndMismatchedIdentityBeforeRepository(t *testing.T) {
	for _, info := range []*sso.UserInfo{
		nil,
		{Provider: "google", Email: "u@example.com", EmailVerified: true},
		{Provider: "google", ProviderID: "id", Email: "u@example.com", EmailVerified: false},
		{Provider: "google", ProviderID: "id", Email: "not-email", EmailVerified: true},
		{Provider: "yandex", ProviderID: "id", Email: "u@example.com"},
	} {
		p := &ssoInvalidProvider{info: info}
		reg := sso.NewRegistry()
		reg.Register("google", p)
		s := &auth{sso: reg}
		if _, err := s.SSOLogin(context.Background(), "google", "code", "browser-verifier", "agent", "ip"); err == nil {
			t.Fatalf("incomplete identity accepted: %+v", info)
		}
		if p.verifier != "browser-verifier" {
			t.Fatal("state-bound verifier was lost before exchange")
		}
		if p.deadline.IsZero() || time.Until(p.deadline) > sso.LoginTimeout {
			t.Fatal("SSO flow has no shared deadline")
		}
	}
}
