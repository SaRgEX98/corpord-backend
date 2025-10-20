package handler

import (
	"corpord-api/internal/apperrors"
	"corpord-api/internal/logger"
	"errors"
	"net/http"
	"time"

	"corpord-api/internal/service"
	"corpord-api/model"

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
// @Failure 400 {object} ErrorResponse "Некорректные данные"
// @Failure 409 {object} ErrorResponse "Пользователь уже существует"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	start := time.Now()
	h.logger.Info("handling user registration request")

	var req model.UserCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("invalid request body: %v", err)
		c.JSON(apperrors.ErrBadRequest.Status, apperrors.ErrorResponse{
			Error: apperrors.ErrBadRequest.Message,
		})
		return
	}

	user, err := h.service.Register(c.Request.Context(), &req)
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

	h.logger.Infof("user registered successfully in %v", time.Since(start))
	c.JSON(http.StatusCreated, user)
}

// Login handles user authentication
// @Summary Аутентификация пользователя
// @Description Вход пользователя в систему
// @Tags auth
// @Accept json
// @Produce json
// @Param input body model.UserLogin true "Данные для входа"
// @Success 200 {object} model.TokenResponse "Успешный вход"
// @Failure 400 {object} ErrorResponse "Некорректные данные"
// @Failure 401 {object} ErrorResponse "Неверные учетные данные"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
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

	token, err := h.service.Login(c.Request.Context(), req)
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

	h.logger.Infof("user %s logged in successfully in %v", req.Email, time.Since(start))
	c.JSON(http.StatusOK, model.TokenResponse{
		AccessToken: token,
	})
}
