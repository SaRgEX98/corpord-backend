package helper

import (
	"github.com/gin-gonic/gin"
	"os"
	"time"
)

func SetRefreshCookie(c *gin.Context, refreshToken string, ttl time.Duration) {
	secure := true
	env := os.Getenv("APP_ENV")
	if env != "production" {
		secure = false
	}
	c.SetCookie(
		"refresh_token",
		refreshToken,
		int(ttl.Seconds()),
		"/",
		"localhost", // потом меняем на домен сервера
		secure,      // Secure=true на продакшене
		true,        // HttpOnly
	)
}
