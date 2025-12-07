package helper

import (
	"github.com/gin-gonic/gin"
	"time"
)

func SetRefreshCookie(c *gin.Context, refreshToken string, ttl time.Duration) {
	c.SetCookie(
		"refresh_token",
		refreshToken,
		int(ttl.Seconds()),
		"/",
		"localhost", // потом меняем на домен сервера
		false,       // Secure=true на продакшене
		true,        // HttpOnly
	)
}
