package handler

import (
	"corpord-api/internal/apperrors"
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/model"
	"github.com/gin-gonic/gin"
	"net/http"
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

// @Summary Получить список пользователей
// @Description Возвращает пользователей. Требуется роль admin. /users — совместимый административный alias.
// @Tags admin/users
// @Security Bearer
// @Produce json
// @Success 200 {array} model.UserResponse "Успешный ответ"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 403 {object} apperrors.ErrorResponse "Требуются права администратора"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/users [get]
// @Router /users [get]
func (h *UserHandler) All(c *gin.Context) {
	output, err := h.s.GetAll(c.Request.Context())
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, output)
}

// @Summary Получить пользователя по ID
// @Description Возвращает пользователя по ID. Требуется роль admin. /users/{id} — совместимый административный alias.
// @Tags admin/users
// @Security Bearer
// @Produce json
// @Param id path int true "Положительный ID записи" minimum(1)
// @Success 200 {object} model.UserResponse "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 403 {object} apperrors.ErrorResponse "Требуются права администратора"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/users/{id} [get]
// @Router /users/{id} [get]
func (h *UserHandler) Get(c *gin.Context) {
	id, ok := domainID(c)
	if !ok {
		return
	}
	output, err := h.s.GetByID(c.Request.Context(), id)
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, output)
}

// @Summary Создать пользователя
// @Description Создает локальную учетную запись. Требуется роль admin, email, name и пароль длиной 1–72 байта. /users — совместимый административный alias.
// @Tags admin/users
// @Security Bearer
// @Accept json
// @Produce json
// @Param input body model.UserCreate true "Данные запроса"
// @Success 201 {object} model.UserResponse "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 403 {object} apperrors.ErrorResponse "Требуются права администратора"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/users [post]
// @Router /users [post]
func (h *UserHandler) Create(c *gin.Context) {
	var input model.UserCreate
	if err := c.ShouldBindJSON(&input); err != nil {
		writeDomainError(c, &model.ValidationError{Message: "Некорректные данные запроса"})
		return
	}
	output, err := h.s.Create(c.Request.Context(), &input)
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusCreated, output)
}

// @Summary Частично обновить пользователя
// @Description Обновляет переданные поля пользователя по ID. Требуется роль admin.
// @Tags admin/users
// @Security Bearer
// @Accept json
// @Produce json
// @Param id path int true "Положительный ID записи" minimum(1)
// @Param input body model.UserUpdate true "Данные запроса"
// @Success 200 {object} model.UserResponse "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 403 {object} apperrors.ErrorResponse "Требуются права администратора"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/users/{id} [put]
func (h *UserHandler) Update(c *gin.Context) {
	id, ok := domainID(c)
	if !ok {
		return
	}
	var input model.UserUpdate
	if err := c.ShouldBindJSON(&input); err != nil {
		writeDomainError(c, &model.ValidationError{Message: "Некорректные данные запроса"})
		return
	}

	output, err := h.s.Update(c.Request.Context(), id, &input)
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, output)
}

// @Summary Удалить пользователя
// @Description Удаляет пользователя по ID. Требуется роль admin. /users/{id} — совместимый административный alias.
// @Tags admin/users
// @Security Bearer
// @Param id path int true "Положительный ID записи" minimum(1)
// @Success 204 "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 403 {object} apperrors.ErrorResponse "Требуются права администратора"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/users/{id} [delete]
// @Router /users/{id} [delete]
func (h *UserHandler) Delete(c *gin.Context) {
	id, ok := domainID(c)
	if !ok {
		return
	}
	err := h.s.Delete(c.Request.Context(), id)
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Получить свой профиль
// @Description Возвращает профиль пользователя из проверенного access token. ID в запросе не передается.
// @Tags users
// @Security Bearer
// @Produce json
// @Success 200 {object} model.UserResponse "Успешный ответ"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /users/me [get]
func (h *UserHandler) Me(c *gin.Context) {
	userID, exists := c.Get("userID")
	id, ok := userID.(int)
	if !exists || !ok || id <= 0 {
		c.JSON(http.StatusUnauthorized, apperrors.ErrorResponse{Error: apperrors.ErrUnauthorized.Message})
		return
	}
	output, err := h.s.GetByID(c.Request.Context(), id)
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, output)
}
