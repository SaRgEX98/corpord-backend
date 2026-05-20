package middleware

import (
	"github.com/gin-gonic/gin"

	"corpord-api/internal/logger"
	"corpord-api/internal/token"
)

const (
	AuthorizationHeader = "Authorization"
	ClaimsCtx           = "claims"
)

// AuthMiddleware validates JWT and injects claims into context
func AuthMiddleware(log *logger.Logger, tm token.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := extractToken(c)
		if err != nil {
			log.Warnf("Auth error: %v", err)
			abortUnauthorized(c)
			return
		}

		claims, err := tm.Validate(tokenString)
		if err != nil {
			log.Warnf("Invalid token: %v", err)
			abortUnauthorized(c)
			return
		}

		c.Set(ClaimsCtx, claims)
		c.Next()
	}
}
