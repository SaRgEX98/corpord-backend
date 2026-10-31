package config

import (
	"net/url"
	"strings"
	"testing"
)

func loadTestConfig(t *testing.T) *Config {
	t.Helper()
	t.Chdir(t.TempDir())
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	t.Setenv("SSO_GOOGLE_ENABLED", "false")
	t.Setenv("SSO_YANDEX_ENABLED", "false")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestConfigRejectsInvalidSecurityAndRuntimeSettings(t *testing.T) {
	for _, test := range []struct{ key, value string }{
		{"JWT_SECRET", ""}, {"JWT_SECRET", "short"}, {"JWT_SIGNING_ALGORITHM", "none"},
		{"JWT_ACCESS_TTL", "bad"}, {"JWT_ACCESS_TTL", "-1s"}, {"JWT_REFRESH_TTL", "1s"},
		{"HTTP_PORT", "0"}, {"HTTP_PORT", "65536"}, {"HTTP_PORT", "abc"},
		{"HTTP_READ_HEADER_TIMEOUT", "0s"}, {"APP_DEBUG", "tru"},
		{"DB_POSTGRES_MAX_CONNS", "0"}, {"DB_POSTGRES_PORT", "bad"}, {"DB_REDIS_DB", "-1"},
		{"FRONTEND_URL", "//attacker.example"}, {"FRONTEND_URL", "https://user:pass@example.org"},
		{"SSO_GOOGLE_ENABLED", "true"}, {"SSO_YANDEX_ENABLED", "true"},
		{"COOKIE_DOMAIN", "http://example.org"}, {"COOKIE_SECURE", "bad"},
		{"COOKIE_SAME_SITE", "invalid"}, {"ALLOWED_ORIGINS", "https://example.org/path"},
	} {
		t.Run(test.key+"="+test.value, func(t *testing.T) {
			loadTestConfig(t)
			t.Setenv(test.key, test.value)
			if _, err := Load(); err == nil {
				t.Fatalf("accepted invalid %s", test.key)
			}
		})
	}
}

func TestConfigEnabledProviderAndCookie(t *testing.T) {
	c := loadTestConfig(t)
	if c.HTTP.ReadHeaderTimeout <= 0 {
		t.Fatal("no header timeout")
	}
	t.Setenv("SSO_GOOGLE_ENABLED", "true")
	t.Setenv("SSO_GOOGLE_CLIENT_ID", "client")
	t.Setenv("SSO_GOOGLE_CLIENT_SECRET", "secret")
	t.Setenv("SSO_GOOGLE_REDIRECT_URL", "https://api.example.org/api/v1/auth/google/callback")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("COOKIE_SAME_SITE", "none")
	t.Setenv("COOKIE_SECURE", "false")
	if _, err := Load(); err == nil {
		t.Fatal("accepted insecure SameSite=None")
	}
}

func TestPostgresDSNEscapesCredentials(t *testing.T) {
	pg := Postgres{Host: "::1", Port: "5432", User: "user with space", Password: "space ' slash/ @", DBName: "corpord", SSLMode: "disable"}
	u, err := url.Parse(pg.DSN())
	if err != nil {
		t.Fatal(err)
	}
	password, _ := u.User.Password()
	if password != pg.Password || u.User.Username() != pg.User || u.Host != "[::1]:5432" {
		t.Fatal("DSN corrupted credentials or IPv6 host")
	}
}
