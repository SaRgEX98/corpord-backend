package handler

import (
	"corpord-api/internal/apperrors"
	"corpord-api/internal/handler/helper"
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/model"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	service service.Auth
	logger  *logger.Logger
}

func NewAuthHandler(s service.Auth, l *logger.Logger) *AuthHandler {
	return &AuthHandler{
		service: s,
		logger:  l,
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

	helper.SetRefreshCookie(c, tokens.RefreshToken, 7*24*time.Hour) // TTL берём как у сервиса

	h.logger.Infof("user registered successfully in %v", time.Since(start))
	c.JSON(http.StatusCreated, tokens)
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

	helper.SetRefreshCookie(c, tokens.RefreshToken, 7*24*time.Hour) // TTL берём как у сервиса

	h.logger.Infof("user %s logged in successfully in %v", req.Email, time.Since(start))
	c.JSON(http.StatusOK, tokens)
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

	helper.SetRefreshCookie(c, tokens.RefreshToken, 7*24*time.Hour) // TTL берём как у сервиса

	h.logger.Infof("user SSO login with provider.go %s successful in %v", req.Provider, time.Since(start))
	c.JSON(http.StatusOK, tokens)
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

	var req model.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("invalid refresh request body: %v", err)
		c.JSON(apperrors.ErrBadRequest.Status, apperrors.ErrorResponse{
			Error: apperrors.ErrBadRequest.Message,
		})
		return
	}

	userAgent := c.GetHeader("User-Agent")
	ip := c.ClientIP()

	tokens, err := h.service.Refresh(c.Request.Context(), req.RefreshToken, userAgent, ip)
	if err != nil {
		h.logger.Warnf("refresh token failed: %v", err)
		c.JSON(apperrors.ErrUnauthorized.Status, apperrors.ErrorResponse{
			Error: "Просроченный или недействительный токен",
		})
		return
	}

	h.logger.Infof("token refreshed successfully in %v", time.Since(start))

	helper.SetRefreshCookie(c, tokens.RefreshToken, 7*24*time.Hour)

	c.JSON(http.StatusOK, tokens)
}

func RegisterAuthRoutes(rg *gin.RouterGroup, authHandler *AuthHandler) {
	auth := rg.Group("/auth")

	auth.POST("/register", authHandler.Register)
	auth.POST("/login", authHandler.Login)
	auth.POST("/sso/login", authHandler.SSOLogin)
	auth.POST("/refresh", authHandler.Refresh)
}
