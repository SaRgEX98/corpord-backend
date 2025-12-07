package handler

import (
	"corpord-api/internal/handler/helper"
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/internal/sso"
	"crypto/rand"
	"encoding/base64"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

type GoogleHandler interface {
	RegisterGoogleRoutes(rg *gin.RouterGroup, google sso.Provider)
}

type googleHandler struct {
	logger *logger.Logger
	s      service.Auth
}

func NewGoogleHandler(logger *logger.Logger, s service.Auth) GoogleHandler {
	return &googleHandler{
		logger: logger,
		s:      s,
	}
}

// --- helpers ---

// generateState creates a cryptographically secure random state string
func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (gh *googleHandler) RegisterGoogleRoutes(rg *gin.RouterGroup, google sso.Provider) {
	g := rg.Group("/auth/google")

	// STEP 1 — redirect to Google OAuth2
	g.GET("", func(c *gin.Context) {
		// 1. generate secure state
		state, err := generateState()
		if err != nil {
			gh.logger.Errorf("failed to generate state: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "state generation failed"})
			return
		}

		// 2. store state in httponly, samesite=lax cookie
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     "oauth_state",
			Value:    state,
			Path:     "/",
			HttpOnly: true,
			Secure:   false, // true in prod with HTTPS
			SameSite: http.SameSiteLaxMode,
			MaxAge:   300,
		})

		// 3. create Google auth URL
		url := google.AuthURL(state)

		gh.logger.Infof("redirecting to Google OAuth2: %s", url)

		c.Redirect(http.StatusTemporaryRedirect, url)
	})

	// STEP 2 — user returned from Google
	g.GET("/callback", func(c *gin.Context) {
		// Check if Google returned an error
		if errMsg := c.Query("error"); errMsg != "" {
			gh.logger.Warnf("Google OAuth error: %s", errMsg)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":             errMsg,
				"error_description": c.Query("error_description"),
			})
			return
		}

		// 1. get params from callback
		code := c.Query("code")
		state := c.Query("state")

		if code == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing code"})
			return
		}

		// 2. read state cookie
		cookie, err := c.Cookie("oauth_state")
		if err != nil {
			gh.logger.Warn("no oauth_state cookie")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid oauth state"})
			return
		}

		// 3. validate state
		if cookie != state {
			gh.logger.Warnf("invalid state: received=%s expected=%s", state, cookie)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid oauth state"})
			return
		}

		// state verified — remove cookie
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     "oauth_state",
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			MaxAge:   -1,
		})

		// 4. complete login through service layer
		resp, err := gh.s.SSOLogin(
			c.Request.Context(),
			"google",
			code,
			"", // fallbackEmail
			"", // fallbackName
			c.GetHeader("User-Agent"),
			c.ClientIP(),
		)
		if err != nil {
			gh.logger.Warnf("SSO login failed: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		// 5. optionally set refresh cookie
		// (если у тебя есть setRefreshCookie — вставь сюда)
		// setRefreshCookie(c, resp.RefreshToken)

		helper.SetRefreshCookie(c, resp.RefreshToken, 7*24*time.Hour)

		gh.logger.Infof("SSO login successful via Google for user: %s", resp.AccessToken)

		// 6. return tokens as JSON
		// или редиректнуть на UI? зависит от flow
		c.JSON(http.StatusOK, resp)
	})
}
