package handler

import (
	"corpord-api/internal/apperrors"
	"corpord-api/internal/handler/helper"
	"corpord-api/internal/handler/middleware"
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

// @Summary Регистрация пользователя
// @Description Создает локальную учетную запись и возвращает access_token. Пароль обязателен, длина 1–72 байта. Refresh token устанавливается в HttpOnly cookie refresh_token.
// @Tags auth
// @Accept json
// @Produce json
// @Param input body model.UserCreate true "Данные запроса"
// @Success 201 {object} model.TokenResponse "Успешный ответ"
// @Header 201 {string} Set-Cookie "HttpOnly cookie refresh_token"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
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
		if errors.Is(err, service.ErrInvalidPass) || errors.Is(err, service.ErrNoFields) {
			c.JSON(http.StatusBadRequest, apperrors.ErrorResponse{Error: "Некорректный пароль"})
			return
		}
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

// @Summary Вход по email и паролю
// @Description Возвращает access_token и устанавливает HttpOnly cookie refresh_token. Пользователь без локального пароля должен входить через своего SSO-провайдера.
// @Tags auth
// @Accept json
// @Produce json
// @Param input body model.UserLogin true "Данные запроса"
// @Success 200 {object} model.TokenResponse "Успешный ответ"
// @Header 200 {string} Set-Cookie "HttpOnly cookie refresh_token"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
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
		if errors.Is(err, service.ErrInvalidCredentials) || errors.Is(err, service.ErrInvalidPass) || errors.Is(err, service.ErrUseSSOLogin) {
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

// @Summary Обновить access token
// @Description Читает refresh_token из cookie; JSON-тело не требуется и не используется. Старый refresh token погашается один раз, новый устанавливается через Set-Cookie. В JSON возвращается только access_token. Для браузерного запроса отправляйте cookies (credentials: include).
// @Tags auth
// @Produce json
// @Param Cookie header string true "refresh_token=<token>; браузер отправляет cookie автоматически"
// @Success 200 {object} model.TokenResponse "Успешный ответ"
// @Header 200 {string} Set-Cookie "Новая HttpOnly cookie refresh_token"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
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
		if errors.Is(err, service.ErrInvalidRefreshToken) || errors.Is(err, service.ErrRefreshTokenExpired) {
			c.JSON(apperrors.ErrUnauthorized.Status, apperrors.ErrorResponse{
				Error: "Просроченный или недействительный токен",
			})
			return
		}
		c.JSON(apperrors.ErrInternal.Status, apperrors.ErrorResponse{Error: apperrors.ErrInternal.Message})
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

// @Summary Завершить текущую refresh-сессию
// @Description Отзывает refresh token из cookie и удаляет cookie. JSON-тело не требуется. Уже выданный access token действует до истечения его TTL.
// @Tags auth
// @Produce json
// @Param Cookie header string true "refresh_token=<token>; браузер отправляет cookie автоматически"
// @Success 200 {object} apperrors.SuccessResponse "Успешный ответ"
// @Header 200 {string} Set-Cookie "Удаление cookie refresh_token"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /auth/logout [post]
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
		if errors.Is(err, service.ErrInvalidRefreshToken) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
			return
		}
		c.JSON(http.StatusInternalServerError, apperrors.ErrorResponse{Error: apperrors.ErrInternal.Message})
		return
	}

	// Убираем cookie
	helper.ClearRefreshCookie(c)

	h.logger.Infof("user logged out from IP %s, User-Agent %s", ip, userAgent)
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

// @Summary Завершить все refresh-сессии пользователя
// @Description Отзывает все refresh tokens пользователя из Bearer access token и удаляет refresh cookie. JSON-тело не требуется. Уже выданные access tokens действуют до истечения TTL.
// @Tags auth
// @Security Bearer
// @Produce json
// @Success 200 {object} apperrors.SuccessResponse "Успешный ответ"
// @Header 200 {string} Set-Cookie "Удаление cookie refresh_token"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /auth/logout/all [post]
func (h *AuthHandler) LogoutAll(c *gin.Context) {
	userID := c.GetInt(middleware.UserIDCtx)
	if userID <= 0 {
		c.JSON(http.StatusUnauthorized, apperrors.ErrorResponse{Error: "Необходима авторизация"})
		return
	}

	if err := h.service.LogoutAll(c.Request.Context(), userID); err != nil {
		h.logger.Warnf("logout all failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not logout all"})
		return
	}

	// Убираем cookie
	helper.ClearRefreshCookie(c)

	h.logger.Infof("user %d logged out from all sessions", userID)
	c.JSON(http.StatusOK, gin.H{"message": "logged out from all sessions"})
}

func RegisterAuthRoutes(rg *gin.RouterGroup, authHandler *AuthHandler) {
	auth := rg.Group("/auth")
	auth.POST("/register", authHandler.Register)
	auth.POST("/login", authHandler.Login)
	auth.POST("/refresh", authHandler.Refresh)
	auth.POST("/logout", authHandler.Logout)
	auth.POST("/logout/all", middleware.AuthMiddleware(authHandler.logger, authHandler.t, authHandler.service), authHandler.LogoutAll)
}
