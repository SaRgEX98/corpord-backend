package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"corpord-api/internal/apperrors"
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/internal/token"
	"corpord-api/model"
)

const (
	AuthorizationHeader = "Authorization"
	ClaimsCtx           = "claims"
	UserIDCtx           = "userID"
)

// AuthMiddleware validates JWT and injects claims into context
func AuthMiddleware(log *logger.Logger, tm token.Manager, authServices ...service.Auth) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := extractToken(c)
		if err != nil {
			log.Warnf("Auth error: %v", err)
			abortUnauthorized(c)
			return
		}

		var claims *model.Claims
		if len(authServices) != 0 {
			claims, err = authServices[0].Authenticate(c.Request.Context(), tokenString)
		} else {
			claims, err = tm.Validate(tokenString)
		}
		if err != nil {
			log.Warnf("Invalid token: %v", err)
			if len(authServices) != 0 && !errors.Is(err, service.ErrInvalidCredentials) && !errors.Is(err, service.ErrUserNotFound) {
				c.AbortWithStatusJSON(http.StatusInternalServerError, apperrors.ErrorResponse{Error: apperrors.ErrInternal.Message})
				return
			}
			abortUnauthorized(c)
			return
		}
		if claims == nil || claims.UserID <= 0 {
			abortUnauthorized(c)
			return
		}

		c.Set(ClaimsCtx, claims)
		c.Set(UserIDCtx, claims.UserID)
		c.Next()
	}
}
