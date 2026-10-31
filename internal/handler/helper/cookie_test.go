package helper

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRefreshCookieSetAndClearUseSameAttributes(t *testing.T) {
	for _, tc := range []struct {
		name, env, domain, secure, sameSite string
		wantSecure                          bool
		wantSameSite                        http.SameSite
	}{
		{"local_defaults", "development", "", "", "", false, http.SameSiteLaxMode},
		{"production", "production", "app.example", "", "strict", true, http.SameSiteStrictMode},
		{"cross_site", "development", "example.com", "true", "none", true, http.SameSiteNoneMode},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("APP_ENV", tc.env)
			t.Setenv("COOKIE_DOMAIN", tc.domain)
			t.Setenv("COOKIE_SECURE", tc.secure)
			t.Setenv("COOKIE_SAME_SITE", tc.sameSite)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			SetRefreshCookie(c, "refresh-token", time.Hour)
			ClearRefreshCookie(c)
			cookies := w.Result().Cookies()
			if len(cookies) != 2 {
				t.Fatalf("got %d cookies", len(cookies))
			}
			set, clear := cookies[0], cookies[1]
			if set.Domain != tc.domain || set.Secure != tc.wantSecure || set.SameSite != tc.wantSameSite || !set.HttpOnly {
				t.Fatalf("incorrect configured cookie %+v", set)
			}
			if set.Name != clear.Name || set.Domain != clear.Domain || set.Path != clear.Path || set.Secure != clear.Secure || set.SameSite != clear.SameSite || !clear.HttpOnly || clear.MaxAge != -1 || clear.Value != "" || clear.Expires.After(time.Now()) {
				t.Fatalf("cookie deletion attributes differ: set=%+v clear=%+v", set, clear)
			}
		})
	}
}
