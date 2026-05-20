package handler

import (
	"corpord-api/internal/apperrors"
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/model"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	logger *logger.Logger
	s      service.User
}

func NewUser(logger *logger.Logger, s service.User) *UserHandler {
	return &UserHandler{
		logger: logger,
		s:      s,
	}
}

// All возвращает список всех пользователей
// @Summary Получить всех пользователей
// @Description Возвращает список всех зарегистрированных пользователей (только для администраторов)
// @Tags users
// @Produce json
// @Security Bearer
// @Success 200 {array} model.UserResponse "Список пользователей"
// @Failure 401 {object} apperrors.ErrorResponse "Не авторизован"
// @Failure 403 {object} apperrors.ErrorResponse "Доступ запрещен"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /users [get]
func (h *UserHandler) All(c *gin.Context) {
	start := time.Now()
	h.logger.Info("handling get all users request")

	users, err := h.s.GetAll(c.Request.Context())
	if err != nil {
		h.logger.Errorf("failed to get users: %v", err)
		c.JSON(apperrors.ErrInternal.Status, apperrors.ErrorResponse{
			Error: "Не удалось получить список пользователей",
		})
		return
	}

	h.logger.Infof("retrieved %d users in %v", len(users), time.Since(start))
	c.JSON(http.StatusOK, users)
}

// Get возвращает пользователя по ID
// @Summary Получить пользователя по ID
// @Description Возвращает информацию о пользователе по его идентификатору
// @Tags users
// @Produce json
// @Security Bearer
// @Param id path int true "ID пользователя"
// @Success 200 {object} model.UserResponse "Данные пользователя"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный ID"
// @Failure 401 {object} apperrors.ErrorResponse "Не авторизован"
// @Failure 403 {object} apperrors.ErrorResponse "Доступ запрещен"
// @Failure 404 {object} apperrors.ErrorResponse "Пользователь не найден"
// @Router /users/{id} [get]
func (h *UserHandler) Get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(apperrors.ErrBadRequest.Status, apperrors.ErrorResponse{
			Error: "Некорректный ID пользователя",
		})
		return
	}

	user, err := h.s.GetByID(c.Request.Context(), id)
	if err != nil {
		h.logger.Errorf("failed to get user %d: %v", id, err)
		if errors.Is(err, service.ErrUserNotFound) {
			c.JSON(apperrors.ErrNotFound.Status, apperrors.ErrorResponse{
				Error: "Пользователь не найден",
			})
			return
		}
		c.JSON(apperrors.ErrInternal.Status, apperrors.ErrorResponse{
			Error: "Не удалось получить данные пользователя",
		})
		return
	}

	c.JSON(http.StatusOK, user)
}

// Create создает нового пользователя
// @Summary Создать нового пользователя
// @Description Создает новую учетную запись пользователя (только для администраторов)
// @Tags admin
// @Accept json
// @Produce json
// @Security Bearer
// @Param input body model.UserCreate true "Данные пользователя"
// @Success 201 {object} model.UserResponse "Пользователь успешно создан"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректные данные"
// @Failure 401 {object} apperrors.ErrorResponse "Не авторизован"
// @Failure 409 {object} apperrors.ErrorResponse "Пользователь с таким email уже существует"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/users [post]
func (h *UserHandler) Create(c *gin.Context) {
	var input model.UserCreate
	if err := c.ShouldBindJSON(&input); err != nil {
		h.logger.Warnf("invalid request body: %v", err)
		c.JSON(apperrors.ErrBadRequest.Status, apperrors.ErrorResponse{
			Error: "Некорректные данные",
		})
		return
	}

	createdUser, err := h.s.Create(c.Request.Context(), &input)
	if err != nil {
		h.logger.Errorf("failed to create user: %v", err)
		if errors.Is(err, service.ErrEmailExists) {
			c.JSON(apperrors.ErrBadRequest.Status, apperrors.ErrorResponse{
				Error: "Пользователь с таким email уже существует",
			})
			return
		}
		c.JSON(apperrors.ErrInternal.Status, apperrors.ErrorResponse{
			Error: "Не удалось создать пользователя",
		})
		return
	}

	c.JSON(http.StatusCreated, createdUser)
}

// Update обновляет данные пользователя
// @Summary Обновить данные пользователя
// @Description Обновляет информацию о пользователе по ID (только для администраторов)
// @Tags admin
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "ID пользователя"
// @Param input body model.UserUpdate true "Обновленные данные пользователя"
// @Success 200 {object} model.UserResponse "Данные пользователя обновлены"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректные данные"
// @Failure 401 {object} apperrors.ErrorResponse "Не авторизован"
// @Failure 403 {object} apperrors.ErrorResponse "Доступ запрещен"
// @Failure 404 {object} apperrors.ErrorResponse "Пользователь не найден"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/users/{id} [put]
func (h *UserHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(apperrors.ErrBadRequest.Status, apperrors.ErrorResponse{
			Error: "Некорректный ID пользователя",
		})
		return
	}

	var update model.UserUpdate
	if err := c.ShouldBindJSON(&update); err != nil {
		h.logger.Warnf("invalid request body: %v", err)
		c.JSON(apperrors.ErrBadRequest.Status, apperrors.ErrorResponse{
			Error: "Некорректные данные для обновления",
		})
		return
	}

	updatedUser, err := h.s.Update(c.Request.Context(), id, &update)
	if err != nil {
		h.logger.Errorf("failed to update user %d: %v", id, err)
		if errors.Is(err, service.ErrUserNotFound) {
			c.JSON(apperrors.ErrNotFound.Status, apperrors.ErrorResponse{
				Error: "Пользователь не найден",
			})
			return
		}
		c.JSON(apperrors.ErrInternal.Status, apperrors.ErrorResponse{
			Error: "Не удалось обновить данные пользователя",
		})
		return
	}

	c.JSON(http.StatusOK, updatedUser)
}

// Delete удаляет пользователя
// @Summary Удалить пользователя
// @Description Удаляет учетную запись пользователя по ID (только для администраторов)
// @Tags users
// @Produce json
// @Security Bearer
// @Param id path int true "ID пользователя"
// @Success 204 "Пользователь успешно удален"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный ID"
// @Failure 401 {object} apperrors.ErrorResponse "Не авторизован"
// @Failure 403 {object} apperrors.ErrorResponse "Доступ запрещен"
// @Failure 404 {object} apperrors.ErrorResponse "Пользователь не найден"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /users/{id} [delete]
func (h *UserHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(apperrors.ErrBadRequest.Status, apperrors.ErrorResponse{
			Error: "Некорректный ID пользователя",
		})
		return
	}

	if err := h.s.Delete(c.Request.Context(), id); err != nil {
		h.logger.Errorf("failed to delete user %d: %v", id, err)
		if errors.Is(err, service.ErrUserNotFound) {
			c.JSON(apperrors.ErrNotFound.Status, apperrors.ErrorResponse{
				Error: "Пользователь не найден",
			})
			return
		}
		c.JSON(apperrors.ErrInternal.Status, apperrors.ErrorResponse{
			Error: "Не удалось удалить пользователя",
		})
		return
	}

	c.Status(http.StatusNoContent)
}

// Me возвращает информацию о текущем пользователе
// @Summary Получить информацию о текущем пользователе
// @Description Возвращает информацию о текущем аутентифицированном пользователе
// @Tags users
// @Produce json
// @Security Bearer
// @Success 200 {object} model.UserResponse "Данные пользователя"
// @Failure 401 {object} apperrors.ErrorResponse "Не авторизован"
// @Failure 404 {object} apperrors.ErrorResponse "Пользователь не найден"
// @Router /me [get]
func (h *UserHandler) Me(c *gin.Context) {
	start := time.Now()
	h.logger.Info("handling get current user request")

	// Получаем ID пользователя из контекста (устанавливается в middleware аутентификации)
	userID, exists := c.Get("userID")
	if !exists {
		h.logger.Error("user ID not found in context")
		c.JSON(http.StatusUnauthorized, apperrors.ErrorResponse{
			Error: "Требуется аутентификация",
		})
		return
	}

	// Преобразуем ID в int
	id, ok := userID.(int)
	if !ok {
		h.logger.Error("invalid user ID type in context")
		c.JSON(http.StatusInternalServerError, apperrors.ErrorResponse{
			Error: "Ошибка сервера",
		})
		return
	}

	// Получаем пользователя из сервиса
	user, err := h.s.GetByID(c.Request.Context(), id)
	if err != nil {
		h.logger.Errorf("failed to get user: %v", err)
		if errors.Is(err, apperrors.ErrNotFound) {
			c.JSON(http.StatusNotFound, apperrors.ErrorResponse{
				Error: "Пользователь не найден",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, apperrors.ErrorResponse{
			Error: "Не удалось получить информацию о пользователе",
		})
		return
	}

	h.logger.Infof("user data retrieved in %v", time.Since(start))
	c.JSON(http.StatusOK, user)
}
