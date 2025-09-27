package middleware

import (
	"corpord-api/internal/logger"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLogger logs incoming HTTP requests and their responses
func RequestLogger(logger *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		// Log request
		logger.Infof("Started %s %s", method, path)

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Log response
		statusCode := c.Writer.Status()
		statusText := http.StatusText(statusCode)

		logger.Infof("Completed %s %s | %d %s | %v",
			method,
			path,
			statusCode,
			statusText,
			latency,
		)
	}
}
