package handler

import (
	"corpord-api/internal/apperrors"
	"corpord-api/internal/handler/helper"
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/internal/token"
	"corpord-api/model"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	service service.Auth
	logger  *logger.Logger
	t       token.Manager
}

func NewAuthHandler(s service.Auth, l *logger.Logger, t token.Manager) *AuthHandler {
	return &AuthHandler{
		service: s,
		logger:  l,
		t:       t,
	}
}

// Register handles user registration
// @Summary Регистрация нового пользователя
// @Description Создает нового пользователя в системе
// @Tags auth
// @Accept json
// @Produce json
// @Param input body model.UserCreate true "Данные пользователя"
// @Success 201 {object} model.UserResponse "Успешная регистрация"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректные данные"
// @Failure 409 {object} apperrors.ErrorResponse "Пользователь уже существует"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	start := time.Now()
	h.logger.Info("handling user registration request")

	userAgent := c.GetHeader("User-Agent")
	ip := c.ClientIP()

	var req model.UserCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("invalid request body: %v", err)
		c.JSON(apperrors.ErrBadRequest.Status, apperrors.ErrorResponse{
			Error: apperrors.ErrBadRequest.Message,
		})
		return
	}

	tokens, err := h.service.Register(c.Request.Context(), &req, userAgent, ip)
	if err != nil {
		h.logger.Warnf("registration failed: %v", err)
		if errors.Is(err, service.ErrEmailExists) {
			c.JSON(http.StatusConflict, apperrors.ErrorResponse{
				Error: "Пользователь с таким email уже существует",
			})
			return
		}
		c.JSON(apperrors.ErrInternal.Status, apperrors.ErrorResponse{
			Error: apperrors.ErrInternal.Message,
		})
		return
	}

	helper.SetRefreshCookie(c, tokens.RefreshToken, h.t.RefreshTTL()) // TTL берём как у сервиса

	h.logger.Infof("user registered successfully in %v", time.Since(start))
	c.JSON(http.StatusCreated, model.TokenResponse{AccessToken: tokens.AccessToken})
}

// Login handles user authentication
// @Summary Аутентификация пользователя
// @Description Вход пользователя в систему
// @Tags auth
// @Accept json
// @Produce json
// @Param input body model.UserLogin true "Данные для входа"
// @Success 200 {object} model.TokenResponse "Успешный вход"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректные данные"
// @Failure 401 {object} apperrors.ErrorResponse "Неверные учетные данные"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	start := time.Now()
	h.logger.Info("handling user login request")

	var req model.UserLogin
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("invalid request body: %v", err)
		c.JSON(apperrors.ErrBadRequest.Status, apperrors.ErrorResponse{
			Error: apperrors.ErrBadRequest.Message,
		})
		return
	}

	userAgent := c.GetHeader("User-Agent")
	ip := c.ClientIP()

	tokens, err := h.service.Login(c.Request.Context(), req, userAgent, ip)
	if err != nil {
		h.logger.Warnf("login failed for %s: %v", req.Email, err)
		if errors.Is(err, service.ErrInvalidCredentials) {
			c.JSON(apperrors.ErrUnauthorized.Status, apperrors.ErrorResponse{
				Error: "Неверный email или пароль",
			})
			return
		}
		c.JSON(apperrors.ErrInternal.Status, apperrors.ErrorResponse{
			Error: apperrors.ErrInternal.Message,
		})
		return
	}

	helper.SetRefreshCookie(c, tokens.RefreshToken, h.t.RefreshTTL())

	h.logger.Infof("user %s logged in successfully in %v", req.Email, time.Since(start))
	c.JSON(http.StatusOK, model.TokenResponse{AccessToken: tokens.AccessToken})
}

// SSOLogin handles SSO authentication
// @Summary Аутентификация через SSO
// @Description Вход пользователя через стороннего провайдера (Google, GitHub и др.)
// @Tags auth
// @Accept json
// @Produce json
// @Param input body model.SSOLoginRequest true "Данные для SSO входа"
// @Success 200 {object} model.TokenResponse "Успешный вход"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректные данные"
// @Failure 401 {object} apperrors.ErrorResponse "Ошибка авторизации через SSO"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /auth/sso/login [post]
func (h *AuthHandler) SSOLogin(c *gin.Context) {
	start := time.Now()
	h.logger.Info("handling SSO login request")

	var req model.SSOLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("invalid SSO request body: %v", err)
		c.JSON(apperrors.ErrBadRequest.Status, apperrors.ErrorResponse{
			Error: apperrors.ErrBadRequest.Message,
		})
		return
	}

	userAgent := c.GetHeader("User-Agent")
	ip := c.ClientIP()

	tokens, err := h.service.SSOLogin(c.Request.Context(), req.Provider, req.ProviderID, req.Email, req.Name, userAgent, ip)
	if err != nil {
		h.logger.Warnf("SSO login failed for provider.go %s: %v", req.Provider, err)
		c.JSON(apperrors.ErrUnauthorized.Status, apperrors.ErrorResponse{
			Error: "Ошибка авторизации через SSO",
		})
		return
	}

	helper.SetRefreshCookie(c, tokens.RefreshToken, h.t.RefreshTTL()) // TTL берём как у сервиса

	h.logger.Infof("user SSO login with provider.go %s successful in %v", req.Provider, time.Since(start))
	c.JSON(http.StatusOK, model.TokenResponse{AccessToken: tokens.AccessToken})
}

// Refresh handles token refresh
// @Summary Обновление токена
// @Description Получение нового access и refresh токенов
// @Tags auth
// @Accept json
// @Produce json
// @Param input body model.RefreshRequest true "Refresh токен"
// @Success 200 {object} model.TokenResponse "Новые токены"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректные данные"
// @Failure 401 {object} apperrors.ErrorResponse "Просроченный или недействительный токен"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	start := time.Now()
	h.logger.Info("handling token refresh request")

	// Получаем refresh token из cookie
	refreshCookie, err := c.Cookie("refresh_token")
	if err != nil || refreshCookie == "" {
		h.logger.Warn("refresh token cookie not found")
		c.JSON(apperrors.ErrUnauthorized.Status, apperrors.ErrorResponse{
			Error: "Просроченный или недействительный токен",
		})
		return
	}

	userAgent := c.GetHeader("User-Agent")
	ip := c.ClientIP()

	// Refresh токены через сервис
	tokens, err := h.service.Refresh(c.Request.Context(), refreshCookie, userAgent, ip)
	if err != nil {
		h.logger.Warnf("refresh token failed: %v", err)
		c.JSON(apperrors.ErrUnauthorized.Status, apperrors.ErrorResponse{
			Error: "Просроченный или недействительный токен",
		})
		return
	}

	// Сохраняем новый refresh token в cookie
	helper.SetRefreshCookie(c, tokens.RefreshToken, h.t.RefreshTTL())

	h.logger.Infof("token refreshed successfully in %v", time.Since(start))

	// Возвращаем новый access token
	c.JSON(http.StatusOK, model.TokenResponse{
		AccessToken: tokens.AccessToken,
	})
}

// LogoutHandler отзывает один токен
func (h *AuthHandler) Logout(c *gin.Context) {
	refreshCookie, err := c.Cookie("refresh_token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token not found"})
		return
	}

	userAgent := c.GetHeader("User-Agent")
	ip := c.ClientIP()

	if err := h.service.Logout(c.Request.Context(), refreshCookie); err != nil {
		h.logger.Warnf("logout failed: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}

	// Убираем cookie
	c.SetCookie("refresh_token", "", -1, "/", "", true, true)

	h.logger.Infof("user logged out from IP %s, User-Agent %s", ip, userAgent)
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

// LogoutAllHandler отзывает все токены пользователя
func (h *AuthHandler) LogoutAll(c *gin.Context) {
	userID := c.GetInt("user_id") // берём из middleware авторизации

	if err := h.service.LogoutAll(c.Request.Context(), userID); err != nil {
		h.logger.Warnf("logout all failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not logout all"})
		return
	}

	// Убираем cookie
	c.SetCookie("refresh_token", "", -1, "/", "", true, true)

	h.logger.Infof("user %d logged out from all sessions", userID)
	c.JSON(http.StatusOK, gin.H{"message": "logged out from all sessions"})
}

func RegisterAuthRoutes(rg *gin.RouterGroup, authHandler *AuthHandler) {
	auth := rg.Group("/auth")
	auth.POST("/register", authHandler.Register)
	auth.POST("/login", authHandler.Login)
	auth.POST("/refresh", authHandler.Refresh)
	auth.POST("/logout", authHandler.Logout)
	auth.POST("/logout/all", authHandler.LogoutAll)
}
