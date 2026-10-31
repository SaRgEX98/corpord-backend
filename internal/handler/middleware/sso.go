package middleware

import (
	"corpord-api/internal/handler/helper"
	"corpord-api/internal/sso"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

func SSOMiddleware(reg *sso.Registry, stores ...sso.StateStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Header("Referrer-Policy", "no-referrer")
		providerName := c.Param("provider")

		_, err := reg.Get(providerName)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "provider not supported"})
			c.Abort()
			return
		}

		if len(stores) == 0 || stores[0] == nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "SSO state storage unavailable"})
			return
		}
		binding, _ := c.Cookie(helper.OAuthCookieName(providerName))
		verifier, err := stores[0].Consume(c.Request.Context(), c.Query("state"), providerName, binding)
		if err != nil {
			status := http.StatusServiceUnavailable
			message := "SSO state storage unavailable"
			if errors.Is(err, sso.ErrInvalidState) {
				status = http.StatusBadRequest
				message = "invalid or expired OAuth state"
			}
			c.AbortWithStatusJSON(status, gin.H{"error": message})
			return
		}
		helper.ClearOAuthCookie(c, providerName)
		if c.Query("error") != "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization declined by provider"})
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
		c.Set("code_verifier", verifier)

		c.Next()
	}
}
