package handler

import (
	"context"
	"corpord-api/internal/handler/helper"
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/internal/sso"
	"corpord-api/internal/token"
	"corpord-api/model"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type ssoHandlerAuth struct {
	service.Auth
	called   int
	verifier string
	err      error
}

func (s *ssoHandlerAuth) SSOLogin(_ context.Context, provider, code, verifier, userAgent, ip string) (*model.TokenPair, error) {
	s.called++
	s.verifier = verifier
	return &model.TokenPair{AccessToken: "private-access", RefreshToken: "private-refresh"}, s.err
}

type ssoHandlerTokens struct{ token.Manager }

func (ssoHandlerTokens) RefreshTTL() time.Duration { return time.Hour }

type ssoTestRecord struct {
	provider, browser, verifier string
	expires                     time.Time
}
type ssoMemoryStore struct{ records map[string]ssoTestRecord }

func (s *ssoMemoryStore) Save(_ context.Context, state, provider, browser, verifier string, ttl time.Duration) error {
	s.records[state] = ssoTestRecord{provider, browser, verifier, time.Now().Add(ttl)}
	return nil
}
func (s *ssoMemoryStore) Consume(_ context.Context, state, provider, browser string) (string, error) {
	r, ok := s.records[state]
	if !ok || r.provider != provider || r.browser != browser || time.Now().After(r.expires) {
		return "", sso.ErrInvalidState
	}
	delete(s.records, state)
	return r.verifier, nil
}

func ssoTestRouter(t *testing.T, store sso.StateStore, auth *ssoHandlerAuth) *gin.Engine {
	t.Helper()
	t.Setenv("COOKIE_DOMAIN", "")
	t.Setenv("COOKIE_SECURE", "false")
	t.Setenv("COOKIE_SAME_SITE", "lax")
	gin.SetMode(gin.TestMode)
	reg := sso.NewRegistry()
	reg.Register("google", sso.NewGoogleProvider("client", "secret", "https://api.example/api/v1/auth/google/callback"))
	reg.Register("yandex", sso.NewYandexProvider("client", "secret", "https://api.example/api/v1/auth/yandex/callback"))
	h := NewSSOHandler(&logger.Logger{SugaredLogger: zap.NewNop().Sugar()}, auth, reg, ssoHandlerTokens{}, WithSSOStateStore(store), WithSSOFrontendURL("https://app.example"))
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	return r
}

func ssoStart(t *testing.T, r *gin.Engine) (state string, cookie *http.Cookie) {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/auth/google", nil))
	if w.Code != http.StatusFound {
		t.Fatalf("start: %d %s", w.Code, w.Body)
	}
	location, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	state = location.Query().Get("state")
	if len(state) < 43 || location.Query().Get("code_challenge_method") != "S256" {
		t.Fatal("missing random state or PKCE challenge")
	}
	for _, candidate := range w.Result().Cookies() {
		if candidate.Name == helper.OAuthCookieName("google") {
			cookie = candidate
		}
	}
	if cookie == nil || cookie.Value == state || !cookie.HttpOnly || cookie.MaxAge != int(sso.StateTTL.Seconds()) || cookie.Domain != "" || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("unsafe OAuth browser cookie: %+v", cookie)
	}
	return state, cookie
}

func ssoCallback(r *gin.Engine, provider, query string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/"+provider+"/callback?"+query, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestSSOCallbackValidatesStateBeforeLoginAndRejectsReplay(t *testing.T) {
	store := &ssoMemoryStore{records: make(map[string]ssoTestRecord)}
	auth := &ssoHandlerAuth{}
	r := ssoTestRouter(t, store, auth)
	state, cookie := ssoStart(t, r)
	for _, tc := range []struct {
		name, provider, query string
		cookie                *http.Cookie
	}{
		{"missing_state", "google", "code=code", cookie},
		{"missing_cookie", "google", "code=code&state=" + state, nil},
		{"invalid_state", "google", "code=code&state=bad", cookie},
		{"other_browser", "google", "code=code&state=" + state, &http.Cookie{Name: cookie.Name, Value: "other-browser"}},
		{"other_provider", "yandex", "code=code&state=" + state, &http.Cookie{Name: helper.OAuthCookieName("yandex"), Value: cookie.Value}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := ssoCallback(r, tc.provider, tc.query, tc.cookie)
			if w.Code != http.StatusBadRequest || auth.called != 0 {
				t.Fatalf("callback bypassed state validation: %d %s calls=%d", w.Code, w.Body, auth.called)
			}
		})
	}
	w := ssoCallback(r, "google", "code=code&state="+state, cookie)
	if w.Code != http.StatusFound || w.Header().Get("Location") != "https://app.example/auth/callback" {
		t.Fatalf("callback did not redirect: %d %s", w.Code, w.Body)
	}
	if auth.called != 1 || len(auth.verifier) < 43 {
		t.Fatal("validated verifier was not passed into login")
	}
	if strings.Contains(w.Body.String(), "private-") || strings.Contains(w.Header().Get("Location"), "private-") || strings.Contains(w.Header().Get("Content-Type"), "json") {
		t.Fatal("callback exposed tokens or wrote JSON before redirect")
	}
	var refresh, clearedState bool
	for _, setCookie := range w.Result().Cookies() {
		if setCookie.Name == "refresh_token" && setCookie.Value == "private-refresh" && setCookie.HttpOnly {
			refresh = true
		}
		if setCookie.Name == cookie.Name && setCookie.MaxAge < 0 {
			clearedState = true
		}
	}
	if !refresh || !clearedState {
		t.Fatal("callback did not set refresh and clear OAuth state cookie")
	}
	if replay := ssoCallback(r, "google", "code=code&state="+state, cookie); replay.Code != http.StatusBadRequest || auth.called != 1 {
		t.Fatal("consumed state was accepted twice")
	}
}

func TestSSOCallbackHandlesDeclineMissingCodeAndConflict(t *testing.T) {
	store := &ssoMemoryStore{records: make(map[string]ssoTestRecord)}
	auth := &ssoHandlerAuth{}
	r := ssoTestRouter(t, store, auth)
	for _, tc := range []struct {
		query  string
		status int
	}{{"error=access_denied", http.StatusUnauthorized}, {"", http.StatusBadRequest}} {
		state, cookie := ssoStart(t, r)
		w := ssoCallback(r, "google", tc.query+"&state="+state, cookie)
		if w.Code != tc.status || auth.called != 0 {
			t.Fatalf("invalid callback reached login: %d, calls=%d", w.Code, auth.called)
		}
		if replay := ssoCallback(r, "google", "code=code&state="+state, cookie); replay.Code != http.StatusBadRequest {
			t.Fatal("failed callback did not consume state")
		}
	}
	state, cookie := ssoStart(t, r)
	auth.err = service.ErrSSOAccountExists
	w := ssoCallback(r, "google", "code=code&state="+state, cookie)
	if w.Code != http.StatusConflict {
		t.Fatalf("existing account did not return conflict: %d", w.Code)
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == "refresh_token" && c.Value != "" {
			t.Fatal("failed SSO issued a refresh cookie")
		}
	}
}

func TestSSOStateStorageRequired(t *testing.T) {
	r := ssoTestRouter(t, nil, &ssoHandlerAuth{})
	for _, path := range []string{"/api/v1/auth/google", "/api/v1/auth/google/callback?state=x&code=y"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("state storage missing: got %d", w.Code)
		}
	}
}
