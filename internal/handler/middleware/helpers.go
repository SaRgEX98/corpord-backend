package middleware

import (
	"strings"

	"corpord-api/internal/apperrors"
	"corpord-api/internal/token"
	"github.com/gin-gonic/gin"
)

func extractToken(c *gin.Context) (string, error) {
	header := c.GetHeader(AuthorizationHeader)
	if header == "" {
		return "", token.ErrInvalidAuthHeader
	}

	parts := strings.Split(header, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", token.ErrInvalidAuthHeader
	}

	t := strings.TrimSpace(parts[1])
	if t == "" {
		return "", token.ErrInvalidToken
	}

	return t, nil
}

func abortUnauthorized(c *gin.Context) {
	c.AbortWithStatusJSON(
		apperrors.ErrUnauthorized.Status,
		apperrors.ErrorResponse{Error: "Необходима авторизация"},
	)
}
