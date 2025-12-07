package middleware

import (
	"corpord-api/internal/handler/helper"
	"corpord-api/internal/service"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	AccessTokenHeader  = "Authorization"
	RefreshTokenCookie = "refresh_token"
)

// RefreshMiddleware автоматически обновляет access token при необходимости
func RefreshMiddleware(auth service.Auth) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Получаем access token из заголовка Authorization
		authHeader := c.GetHeader(AccessTokenHeader)
		tokenStr := ""
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
		}

		if tokenStr != "" {
			// Проверяем access token
			userID, err := auth.ValidateToken(tokenStr)
			if err == nil {
				// Токен валиден, сохраняем userID в контекст
				c.Set("userID", userID)
				c.Next()
				return
			}
		}

		// Access token недействителен, пробуем обновить через refresh token
		rtCookie, err := c.Cookie(RefreshTokenCookie)
		if err != nil {
			// Нет refresh token → Unauthorized
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token expired, please login"})
			return
		}

		userAgent := c.GetHeader("User-Agent")
		ip := c.ClientIP()

		tokens, err := auth.Refresh(c.Request.Context(), rtCookie, userAgent, ip)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token, please login"})
			return
		}

		// Устанавливаем новый refresh token в HttpOnly cookie
		c.SetCookie(
			RefreshTokenCookie,
			tokens.RefreshToken,
			3600*24*30, // 30 дней, можно менять на refreshTTL
			"/",
			"",
			true,
			true,
		)

		// Добавляем новый access token в header
		c.Header(AccessTokenHeader, "Bearer "+tokens.AccessToken)

		// Получаем userID из нового access token и сохраняем в контекст
		userID, _ := auth.ValidateToken(tokens.AccessToken)

		helper.SetRefreshCookie(c, tokens.RefreshToken, 7*24*time.Hour) // TTL берём как у сервиса

		c.Set("userID", userID)

		c.Next()
	}
}
