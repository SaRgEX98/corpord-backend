package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"corpord-api/internal/apperrors"
	"corpord-api/internal/logger"
	"corpord-api/internal/token"
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
			c.AbortWithStatusJSON(apperrors.ErrUnauthorized.Status, apperrors.ErrorResponse{Error: "Необходима авторизация"})
			return
		}

		claims, err := tokenManager.Validate(tokenString)
		if err != nil {
			logger.Warnf("Invalid token: %v", err)
			c.AbortWithStatusJSON(apperrors.ErrUnauthorized.Status, apperrors.ErrorResponse{Error: "Неверный токен авторизации"})
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
			logger.Warnf("Role check failed: role not found in context")
			c.AbortWithStatusJSON(apperrors.ErrForbidden.Status, apperrors.ErrorResponse{Error: "Не удалось определить роль пользователя"})
			return
		}

		role, ok := roleVal.(string)
		if !ok {
			logger.Error("Role check failed: invalid role type in context")
			c.AbortWithStatusJSON(apperrors.ErrInternal.Status, apperrors.ErrorResponse{Error: "Внутренняя ошибка сервера"})
			return
		}

		if role != requiredRole {
			logger.Warnf("Insufficient permissions: required role %s, got %s", requiredRole, role)
			c.AbortWithStatusJSON(apperrors.ErrForbidden.Status, apperrors.ErrorResponse{
				Error: fmt.Sprintf("Недостаточно прав. Требуется роль: %s", requiredRole),
			})
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
