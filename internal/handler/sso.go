package handler

import (
	"corpord-api/internal/handler/helper"
	"corpord-api/internal/handler/middleware"
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/internal/sso"
	"corpord-api/internal/token"
	"corpord-api/model"
	"crypto/rand"
	"encoding/base64"
	"net/http"

	"github.com/gin-gonic/gin"
)

type SSOHandler struct {
	log      *logger.Logger
	s        service.Auth
	registry *sso.Registry
	t        token.Manager
}

func NewSSOHandler(log *logger.Logger, s service.Auth, reg *sso.Registry, t token.Manager) *SSOHandler {
	return &SSOHandler{log: log, s: s, registry: reg, t: t}
}

func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (h *SSOHandler) redirectToProvider(c *gin.Context) {
	providerName := c.Param("provider")

	p, err := h.registry.Get(providerName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "provider not supported"})
		return
	}

	state, err := generateState()
	if err != nil {
		h.log.Errorf("state generation error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "state error"})
		return
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   300,
	})

	url := p.AuthURL(state)

	h.log.Infof("redirecting to provider %s: %s", providerName, url)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func (h *SSOHandler) callbackFromProvider(c *gin.Context) {
	provider := c.GetString("provider")
	code := c.GetString("code")

	resp, err := h.s.SSOLogin(
		c.Request.Context(),
		provider,
		code,
		"",
		"",
		c.GetHeader("User-Agent"),
		c.ClientIP(),
	)
	if err != nil {
		h.log.Warnf("SSO login failed for %s: %v", provider, err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	helper.SetRefreshCookie(c, resp.RefreshToken, h.t.RefreshTTL())
	c.JSON(http.StatusOK, model.TokenResponse{AccessToken: resp.AccessToken})
}

func (h *SSOHandler) RegisterRoutes(r *gin.RouterGroup) {
	ssoGroup := r.Group("/auth/:provider")

	ssoGroup.GET("", h.redirectToProvider)
	ssoGroup.GET("/callback", middleware.SSOMiddleware(h.registry), h.callbackFromProvider)
}
