package middleware

import (
	"corpord-api/internal/sso"
	"github.com/gin-gonic/gin"
	"net/http"
)

func SSOMiddleware(reg *sso.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		providerName := c.Param("provider")

		_, err := reg.Get(providerName)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "provider not supported"})
			c.Abort()
			return
		}

		code := c.Query("code")
		if code == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing code"})
			c.Abort()
			return
		}

		c.Set("provider", providerName)
		c.Set("code", code)

		c.Next()
	}
}
