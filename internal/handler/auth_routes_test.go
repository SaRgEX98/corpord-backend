package handler

import (
	"bytes"
	"context"
	"corpord-api/internal/config"
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/internal/sso"
	"corpord-api/internal/token"
	"corpord-api/model"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type routeAuthStub struct {
	service.Auth
	logoutUserIDs []int
	refreshCalls  int
	registerErr   error
	loginErr      error
}

func (s *routeAuthStub) Authenticate(_ context.Context, raw string) (*model.Claims, error) {
	switch raw {
	case "user":
		return &model.Claims{UserID: 42, Role: model.RoleUser}, nil
	case "admin":
		return &model.Claims{UserID: 7, Role: model.RoleAdmin}, nil
	default:
		return nil, service.ErrInvalidCredentials
	}
}
func (s *routeAuthStub) LogoutAll(_ context.Context, userID int) error {
	s.logoutUserIDs = append(s.logoutUserIDs, userID)
	return nil
}
func (s *routeAuthStub) Refresh(context.Context, string, string, string) (*model.TokenPair, error) {
	s.refreshCalls++
	return nil, errors.New("refresh should only be called explicitly")
}
func (s *routeAuthStub) Register(context.Context, *model.UserCreate, string, string) (*model.TokenPair, error) {
	return nil, s.registerErr
}
func (s *routeAuthStub) Login(context.Context, model.UserLogin, string, string) (*model.TokenPair, error) {
	return nil, s.loginErr
}

type routeUserStub struct {
	service.User
	calls int
}

func (s *routeUserStub) GetAll(context.Context) ([]*model.UserResponse, error) {
	s.calls++
	return []*model.UserResponse{{ID: 42}}, nil
}
func (s *routeUserStub) GetByID(_ context.Context, id int) (*model.UserResponse, error) {
	s.calls++
	return &model.UserResponse{ID: id}, nil
}
func (s *routeUserStub) Create(context.Context, *model.UserCreate) (*model.UserResponse, error) {
	s.calls++
	return &model.UserResponse{ID: 99}, nil
}
func (s *routeUserStub) Delete(context.Context, int) error {
	s.calls++
	return nil
}
func (s *routeUserStub) Update(context.Context, int, *model.UserUpdate) (*model.UserResponse, error) {
	s.calls++
	return &model.UserResponse{ID: 99}, nil
}

func authRoutesTestServer() (*gin.Engine, *routeAuthStub, *routeUserStub) {
	gin.SetMode(gin.TestMode)
	log := &logger.Logger{SugaredLogger: zap.NewNop().Sugar()}
	auth, users := &routeAuthStub{}, &routeUserStub{}
	tm := token.NewManager(&config.JWT{Secret: "http-tests-secret-at-least-32-bytes-long", SigningAlgorithm: "HS256", AccessTokenTTL: time.Minute, RefreshTokenTTL: time.Hour})
	router := New(log, &service.Service{Auth: auth, User: users}, &config.Config{}, tm, sso.NewRegistry()).InitRoutes()
	return router, auth, users
}

func routeRequest(router http.Handler, method, path, bearer, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	req.Header.Set("Content-Type", "application/json")
	// A refresh cookie must never cause protected routes to rotate automatically.
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "session"})
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res
}

func TestUserManagementRoutesRequireAdminIncludingLegacyAliases(t *testing.T) {
	router, auth, users := authRoutesTestServer()
	for _, prefix := range []string{"/api/v1/users", "/api/v1/admin/users"} {
		for _, route := range []struct {
			method string
			suffix string
			body   string
			status int
		}{
			{http.MethodGet, "", "", http.StatusOK},
			{http.MethodGet, "/99", "", http.StatusOK},
			{http.MethodPost, "", `{"email":"new@example.com","name":"New","password":"password"}`, http.StatusCreated},
			{http.MethodDelete, "/99", "", http.StatusNoContent},
		} {
			path := prefix + route.suffix
			for _, principal := range []struct {
				token  string
				status int
			}{{"", http.StatusUnauthorized}, {"invalid", http.StatusUnauthorized}, {"user", http.StatusForbidden}, {"admin", route.status}} {
				before := users.calls
				res := routeRequest(router, route.method, path, principal.token, route.body)
				if res.Code != principal.status {
					t.Errorf("%s %s token=%q: want %d got %d (%s)", route.method, path, principal.token, principal.status, res.Code, res.Body.String())
				}
				if principal.token != "admin" && users.calls != before {
					t.Errorf("unauthorized request reached user service: %s %s", route.method, path)
				}
			}
		}
	}
	if auth.refreshCalls != 0 {
		t.Fatal("a protected route automatically refreshed credentials")
	}
}

func TestCurrentUserRouteAcceptsRegularUser(t *testing.T) {
	router, _, _ := authRoutesTestServer()
	res := routeRequest(router, http.MethodGet, "/api/v1/users/me", "user", "")
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"id":42`) {
		t.Fatalf("current user context was lost: %d %s", res.Code, res.Body.String())
	}
}

func TestLogoutAllRequiresAuthenticationAndUsesAuthenticatedUser(t *testing.T) {
	t.Setenv("COOKIE_DOMAIN", "")
	router, auth, _ := authRoutesTestServer()
	for _, raw := range []string{"", "invalid"} {
		res := routeRequest(router, http.MethodPost, "/api/v1/auth/logout/all", raw, "")
		if res.Code != http.StatusUnauthorized || len(auth.logoutUserIDs) != 0 {
			t.Fatalf("logout-all accepted invalid principal %q: %d %+v", raw, res.Code, auth.logoutUserIDs)
		}
	}
	res := routeRequest(router, http.MethodPost, "/api/v1/auth/logout/all", "user", "")
	if res.Code != http.StatusOK || len(auth.logoutUserIDs) != 1 || auth.logoutUserIDs[0] != 42 {
		t.Fatalf("logout-all used incorrect user: %d %+v", res.Code, auth.logoutUserIDs)
	}
	cookies := res.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "refresh_token" || cookies[0].MaxAge >= 0 {
		t.Fatalf("logout-all did not clear session cookie: %+v", cookies)
	}
}

func TestAuthHandlerClassifiesInvalidPasswordAndLoginFailures(t *testing.T) {
	router, auth, _ := authRoutesTestServer()
	auth.registerErr = service.ErrInvalidPass
	res := routeRequest(router, http.MethodPost, "/api/v1/auth/register", "", `{"email":"new@example.com","name":"New"}`)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("missing registration password returned %d: %s", res.Code, res.Body.String())
	}
	for _, loginErr := range []error{service.ErrInvalidCredentials, service.ErrInvalidPass, service.ErrUseSSOLogin} {
		auth.loginErr = loginErr
		res = routeRequest(router, http.MethodPost, "/api/v1/auth/login", "", `{"email":"new@example.com","password":"wrong"}`)
		if res.Code != http.StatusUnauthorized {
			t.Errorf("login failure %v returned %d: %s", loginErr, res.Code, res.Body.String())
		}
	}
	auth.loginErr = errors.New("database unavailable")
	res = routeRequest(router, http.MethodPost, "/api/v1/auth/login", "", `{"email":"new@example.com","password":"wrong"}`)
	if res.Code != http.StatusInternalServerError || strings.Contains(res.Body.String(), "database unavailable") {
		t.Fatalf("database failure was misclassified or leaked: %d %s", res.Code, res.Body.String())
	}
}

func TestRecoveryDoesNotLogOAuthQueryCookiesOrPanicValue(t *testing.T) {
	oldMode, oldWriter, oldErrorWriter := gin.Mode(), gin.DefaultWriter, gin.DefaultErrorWriter
	var output bytes.Buffer
	gin.SetMode(gin.DebugMode)
	gin.DefaultWriter, gin.DefaultErrorWriter = &output, &output
	t.Cleanup(func() {
		gin.SetMode(oldMode)
		gin.DefaultWriter, gin.DefaultErrorWriter = oldWriter, oldErrorWriter
	})
	core := zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(&output), zap.DebugLevel)
	log := &logger.Logger{SugaredLogger: zap.New(core).Sugar()}
	router := New(log, &service.Service{}, &config.Config{}, nil, sso.NewRegistry()).InitRoutes()
	router.GET("/panic", func(c *gin.Context) { panic("panic-secret") })
	req := httptest.NewRequest(http.MethodGet, "/panic?code=code-secret&state=state-secret", nil)
	req.Header.Set("Authorization", "Bearer bearer-secret")
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "cookie-secret"})
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("panic returned %d", res.Code)
	}
	if !strings.Contains(output.String(), "request panic recovered") {
		t.Fatal("panic was not reported")
	}
	for _, secret := range []string{"code-secret", "state-secret", "bearer-secret", "cookie-secret", "panic-secret"} {
		if strings.Contains(output.String(), secret) || strings.Contains(res.Body.String(), secret) {
			t.Errorf("recovery disclosed %s", secret)
		}
	}
}
