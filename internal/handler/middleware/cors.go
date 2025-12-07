package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware настраивает CORS для Gin
func CORSMiddleware() gin.HandlerFunc {
	// Берем ALLOWED_ORIGINS из env или оставляем пустым
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	origins := strings.Split(allowedOrigins, ",")

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		// Разрешаем localhost в dev
		if strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "http://127.0.0.1") {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			// Разрешаем только перечисленные origin из ALLOWED_ORIGINS
			allowed := false
			for _, o := range origins {
				if strings.TrimSpace(o) == origin {
					allowed = true
					break
				}
			}
			if allowed {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			}
		}

		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
