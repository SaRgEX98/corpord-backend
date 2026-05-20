package middleware

import (
	"corpord-api/internal/apperrors"
	"corpord-api/internal/logger"
	"corpord-api/model"
	"fmt"
	"github.com/gin-gonic/gin"
)

// RoleMiddleware requires user to have one of the required roles
func RoleMiddleware(log *logger.Logger, requiredRoles ...string) gin.HandlerFunc {
	roleSet := make(map[string]struct{}, len(requiredRoles))
	for _, r := range requiredRoles {
		roleSet[r] = struct{}{}
	}

	return func(c *gin.Context) {
		claimsRaw, ok := c.Get(ClaimsCtx)
		if !ok {
			log.Warn("Role check failed: claims not found in context")
			c.AbortWithStatusJSON(apperrors.ErrForbidden.Status, apperrors.ErrorResponse{
				Error: "Не удалось определить данные пользователя",
			})
			return
		}

		claims, ok := claimsRaw.(*model.Claims)
		if !ok {
			log.Error("Role check failed: invalid claims type in context")
			c.AbortWithStatusJSON(apperrors.ErrInternal.Status, apperrors.ErrorResponse{
				Error: "Внутренняя ошибка сервера",
			})
			return
		}

		if _, exists := roleSet[claims.Role]; !exists {
			log.Warnf("Insufficient permissions: required %v, got %s", requiredRoles, claims.Role)
			c.AbortWithStatusJSON(apperrors.ErrForbidden.Status, apperrors.ErrorResponse{
				Error: fmt.Sprintf("Недостаточно прав. Требуется одна из ролей: %v", requiredRoles),
			})
			return
		}

		c.Next()
	}
}
