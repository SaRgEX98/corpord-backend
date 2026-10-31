package helper

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func configuredCookie(name, value string, ttl time.Duration) *http.Cookie {
	secure := os.Getenv("APP_ENV") == "production"
	if configured, err := strconv.ParseBool(os.Getenv("COOKIE_SECURE")); err == nil {
		secure = configured
	}
	sameSite := http.SameSiteLaxMode
	switch strings.ToLower(os.Getenv("COOKIE_SAME_SITE")) {
	case "strict":
		sameSite = http.SameSiteStrictMode
	case "none":
		sameSite = http.SameSiteNoneMode
		secure = true
	}
	maxAge := int(ttl.Seconds())
	expires := time.Now().Add(ttl)
	if ttl < 0 {
		maxAge = -1
		expires = time.Unix(1, 0)
	}
	return &http.Cookie{
		Name: name, Value: value, Path: "/", Domain: os.Getenv("COOKIE_DOMAIN"),
		Secure: secure, HttpOnly: true, SameSite: sameSite,
		MaxAge: maxAge, Expires: expires,
	}
}

func SetRefreshCookie(c *gin.Context, refreshToken string, ttl time.Duration) {
	http.SetCookie(c.Writer, configuredCookie("refresh_token", refreshToken, ttl))
}

func ClearRefreshCookie(c *gin.Context) {
	http.SetCookie(c.Writer, configuredCookie("refresh_token", "", -time.Second))
}

func OAuthCookieName(provider string) string {
	return "oauth_state_" + provider
}

// OAuth callbacks arrive via a top-level navigation from another site, which
// requires Lax even when the refresh cookie policy is Strict.
func SetOAuthCookie(c *gin.Context, provider, binding string, ttl time.Duration) {
	cookie := configuredCookie(OAuthCookieName(provider), binding, ttl)
	cookie.SameSite = http.SameSiteLaxMode
	http.SetCookie(c.Writer, cookie)
}

func ClearOAuthCookie(c *gin.Context, provider string) {
	SetOAuthCookie(c, provider, "", -time.Second)
}
