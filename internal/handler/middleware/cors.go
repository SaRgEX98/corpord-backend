package middleware

import (
	"github.com/gin-gonic/gin"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

func normalizedOrigin(raw string) (string, string, bool) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", "", false
	}
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "", "", false
		}
	}
	if (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443") {
		port = ""
	}
	authority := host
	if strings.Contains(host, ":") {
		authority = "[" + host + "]"
	}
	if port != "" {
		authority = net.JoinHostPort(host, port)
	}
	return u.Scheme + "://" + authority, host, true
}

func CORSMiddleware() gin.HandlerFunc {
	allowed := make(map[string]bool)
	for _, raw := range strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",") {
		if origin, _, ok := normalizedOrigin(strings.TrimSpace(raw)); ok {
			allowed[origin] = true
		}
	}
	development := os.Getenv("APP_ENV") == "" || os.Getenv("APP_ENV") == "development"
	return func(c *gin.Context) {
		c.Writer.Header().Add("Vary", "Origin")
		raw := c.GetHeader("Origin")
		if raw != "" {
			origin, host, valid := normalizedOrigin(raw)
			loopback := host == "localhost"
			if ip := net.ParseIP(host); ip != nil {
				loopback = ip.IsLoopback()
			}
			if !valid || (!allowed[origin] && !(development && loopback)) {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Header("Access-Control-Allow-Origin", raw)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
