package handler

import (
	"corpord-api/internal/handler/helper"
	"corpord-api/internal/handler/middleware"
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/internal/sso"
	"corpord-api/internal/token"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

type SSOHandler struct {
	log         *logger.Logger
	s           service.Auth
	registry    *sso.Registry
	t           token.Manager
	stateStore  sso.StateStore
	frontendURL string
}

type SSOOption func(*SSOHandler)

func WithSSOStateStore(store sso.StateStore) SSOOption {
	return func(h *SSOHandler) { h.stateStore = store }
}

func WithSSOFrontendURL(frontendURL string) SSOOption {
	return func(h *SSOHandler) { h.frontendURL = strings.TrimRight(frontendURL, "/") }
}

func NewSSOHandler(log *logger.Logger, s service.Auth, reg *sso.Registry, t token.Manager, opts ...SSOOption) *SSOHandler {
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}
	h := &SSOHandler{log: log, s: s, registry: reg, t: t, frontendURL: strings.TrimRight(frontendURL, "/")}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// @Summary Начать вход через SSO
// @Description Создает одноразовый state, браузерную cookie и PKCE verifier; перенаправляет браузер к включенному провайдеру.
// @Tags auth/sso
// @Produce html,json
// @Param provider path string true "SSO-провайдер" Enums(google,yandex)
// @Success 302 "Успешный ответ"
// @Header 302 {string} Location "URL авторизации провайдера"
// @Header 302 {string} Set-Cookie "HttpOnly cookie привязки OAuth-потока"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Failure 503 {object} apperrors.ErrorResponse "Хранилище SSO временно недоступно"
// @Router /auth/{provider} [get]
func (h *SSOHandler) redirectToProvider(c *gin.Context) {
	providerName := c.Param("provider")

	p, err := h.registry.Get(providerName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "provider not supported"})
		return
	}
	if h.stateStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "SSO state storage unavailable"})
		return
	}

	state, err := generateState()
	if err != nil {
		h.log.Errorf("state generation error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "state error"})
		return
	}

	binding, err := generateState()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "state error"})
		return
	}
	verifier, err := generateState()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "state error"})
		return
	}
	if err := h.stateStore.Save(c.Request.Context(), state, providerName, binding, verifier, sso.StateTTL); err != nil {
		h.log.Warn("failed to save OAuth state")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "SSO state storage unavailable"})
		return
	}
	helper.SetOAuthCookie(c, providerName, binding, sso.StateTTL)

	url := p.AuthURL(state, verifier)

	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.Redirect(http.StatusFound, url)
}

// @Summary Завершить вход через SSO
// @Description Проверяет и погашает state, привязанный к браузеру и провайдеру, обменивает code с PKCE, устанавливает refresh cookie и возвращает один redirect на FRONTEND_URL/auth/callback. Токены не включаются в URL. Frontend затем вызывает POST /auth/refresh и GET /users/me с cookies. Совпадение email с существующей учетной записью не привязывает SSO автоматически.
// @Tags auth/sso
// @Produce html,json
// @Param provider path string true "SSO-провайдер" Enums(google,yandex)
// @Param code query string false "Authorization code; обязателен при успешной авторизации"
// @Param state query string true "Одноразовый state из начала OAuth-потока"
// @Param error query string false "Ошибка или отказ пользователя, возвращенные провайдером"
// @Param Cookie header string true "Cookie привязки потока, установленная в начале SSO"
// @Success 302 "Успешный ответ"
// @Header 302 {string} Location "FRONTEND_URL/auth/callback"
// @Header 302 {string} Set-Cookie "HttpOnly cookie refresh_token; удаление OAuth cookie"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
// @Failure 503 {object} apperrors.ErrorResponse "Хранилище SSO временно недоступно"
// @Router /auth/{provider}/callback [get]
func (h *SSOHandler) callbackFromProvider(c *gin.Context) {
	provider := c.GetString("provider")
	code := c.GetString("code")

	resp, err := h.s.SSOLogin(
		c.Request.Context(),
		provider,
		code,
		c.GetString("code_verifier"),
		c.GetHeader("User-Agent"),
		c.ClientIP(),
	)
	if err != nil {
		h.log.Warnf("SSO login failed for %s: %v", provider, err)
		if errors.Is(err, service.ErrSSOAccountExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "account already exists; sign in using its existing login method"})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "SSO login failed"})
		return
	}

	helper.SetRefreshCookie(c, resp.RefreshToken, h.t.RefreshTTL())
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.Redirect(http.StatusFound, h.frontendURL+"/auth/callback")
}

func (h *SSOHandler) RegisterRoutes(r *gin.RouterGroup) {
	ssoGroup := r.Group("/auth/:provider")

	ssoGroup.GET("", h.redirectToProvider)
	ssoGroup.GET("/callback", middleware.SSOMiddleware(h.registry, h.stateStore), h.callbackFromProvider)
}
