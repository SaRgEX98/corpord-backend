package middleware

import (
	"corpord-api/internal/logger"
	"corpord-api/internal/token"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	AuthorizationHeader = "Authorization"
	UserCtx             = "userId"
	RoleCtx             = "userRole"
)

// AuthMiddleware validates JWT token and sets user ID and role in context
func AuthMiddleware(logger *logger.Logger, tokenManager token.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := getTokenFromHeader(c)
		if err != nil {
			logger.Warnf("Auth error: %v", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		claims, err := tokenManager.Validate(tokenString)
		if err != nil {
			logger.Warnf("Invalid token: %v", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.Set(UserCtx, claims.UserID)
		c.Set(RoleCtx, claims.Role)
		c.Next()
	}
}

// RoleMiddleware checks if user has required role
func RoleMiddleware(requiredRole string, logger *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleVal, exists := c.Get(RoleCtx)
		if !exists {
			err := errors.New("role not found in context")
			logger.Warnf("Role check failed: %v", err)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}

		role, ok := roleVal.(string)
		if !ok {
			err := errors.New("invalid role type in context")
			logger.Errorf("Role check failed: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if role != requiredRole {
			err := fmt.Errorf("insufficient permissions: required role %s, got %s", requiredRole, role)
			logger.Warnf("Role check failed: %v", err)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			return
		}

		logger.Debugf("Role check passed: user has role %s (required: %s)", role, requiredRole)
		c.Next()
	}
}

func getTokenFromHeader(c *gin.Context) (string, error) {
	header := c.GetHeader(AuthorizationHeader)
	if header == "" {
		return "", token.ErrInvalidAuthHeader
	}

	headerParts := strings.Split(header, " ")
	if len(headerParts) != 2 || headerParts[0] != "Bearer" {
		return "", token.ErrInvalidAuthHeader
	}

	t := strings.TrimSpace(headerParts[1])
	if len(t) == 0 {
		return "", token.ErrInvalidToken
	}

	return t, nil
}
