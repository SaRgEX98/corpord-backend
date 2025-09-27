package middleware

import (
	"corpord-api/internal/logger"
	"github.com/gin-gonic/gin"
)

// CheckAuthorization is a middleware that checks if the user is authorized
func CheckAuthorization(logger *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement proper authorization logic
		// For now, we'll just log the request and continue
		logger.Debugf("Authorization check for path: %s", c.Request.URL.Path)
		c.Next()
	}
}
