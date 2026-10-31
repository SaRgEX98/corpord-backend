package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestCORSOrigins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name, env, origin string
		status            int
	}{
		{"allowed", "production", "https://app.example.org", 204},
		{"default port", "production", "https://app.example.org:443", 204},
		{"wrong port", "production", "https://app.example.org:444", 403},
		{"localhost prefix", "production", "http://localhost.attacker.example", 403},
		{"ip prefix", "production", "http://127.0.0.1.attacker.example", 403},
		{"production loopback", "production", "http://localhost:3000", 403},
		{"default environment", "", "http://localhost:3000", 204},
		{"development loopback", "development", "http://localhost:3000", 204},
		{"development ipv6", "development", "http://[::1]:3000", 204},
		{"development attacker", "development", "http://localhost.attacker.example", 403},
		{"null origin", "production", "null", 403},
		{"path", "production", "https://app.example.org/other", 403},
		{"no origin", "production", "", 204},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("APP_ENV", test.env)
			t.Setenv("ALLOWED_ORIGINS", "https://app.example.org")
			r := gin.New()
			r.Use(CORSMiddleware())
			r.GET("/", func(c *gin.Context) { c.Status(204) })
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("Origin", test.origin)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != test.status {
				t.Fatalf("status %d, want %d", w.Code, test.status)
			}
			if test.status == 403 && w.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("rejected origin allowed")
			}
			if test.status == 204 && test.origin != "" && w.Header().Get("Access-Control-Allow-Origin") != test.origin {
				t.Fatal("allowed origin not reflected")
			}
			if w.Header().Get("Vary") != "Origin" {
				t.Fatal("missing Vary")
			}
		})
	}
}
