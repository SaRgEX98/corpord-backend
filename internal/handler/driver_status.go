package handler

import (
	"corpord-api/internal/apperrors"
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/model"
	"github.com/gin-gonic/gin"
	"net/http"
)

type DriverStatus struct {
	logger *logger.Logger
	s      service.DriverStatus
}

func NewDriverStatus(l *logger.Logger, s service.DriverStatus) *DriverStatus {
	return &DriverStatus{
		logger: l,
		s:      s,
	}
}

// @Summary Получить список статусов водителей
// @Description Возвращает список статусов водителей с идентификаторами записей.
// @Tags driver/status
// @Produce json
// @Success 200 {array} model.DriverStatus "Успешный ответ"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /driver/status/ [get]
func (h *DriverStatus) All(c *gin.Context) {
	output, err := h.s.All(c.Request.Context())
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, output)
}

// @Summary Получить запись статуса водителя
// @Description Возвращает данные статуса водителя по ID.
// @Tags driver/status
// @Produce json
// @Param id path int true "Положительный ID записи" minimum(1)
// @Success 200 {object} model.DriverStatus "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /driver/status/{id} [get]
func (h *DriverStatus) ById(c *gin.Context) {
	id, ok := domainID(c)
	if !ok {
		return
	}
	output, err := h.s.ById(c.Request.Context(), id)
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, output)
}

// @Summary Создать запись статуса водителя
// @Description Создает запись статуса водителя. Требуется роль admin. Поле name обязательно; id назначается сервером.
// @Tags admin/driver/status
// @Security Bearer
// @Accept json
// @Produce json
// @Param input body model.DriverStatus true "Данные запроса"
// @Success 201 {object} apperrors.SuccessResponse "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 403 {object} apperrors.ErrorResponse "Требуются права администратора"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/driver/status/ [post]
func (h *DriverStatus) Create(c *gin.Context) {
	var input model.DriverStatus
	if err := c.ShouldBindJSON(&input); err != nil {
		writeDomainError(c, &model.ValidationError{Message: "Некорректные данные запроса"})
		return
	}
	err := h.s.Create(c.Request.Context(), &input)
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusCreated, apperrors.SuccessResponse{Message: "created"})
}

// @Summary Обновить запись статуса водителя
// @Description Обновляет запись статуса водителя по ID из URL. Требуется роль admin. Поле name обязательно; id в JSON игнорируется.
// @Tags admin/driver/status
// @Security Bearer
// @Accept json
// @Produce json
// @Param id path int true "Положительный ID записи" minimum(1)
// @Param input body model.DriverStatus true "Данные запроса"
// @Success 200 {object} apperrors.SuccessResponse "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 403 {object} apperrors.ErrorResponse "Требуются права администратора"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/driver/status/{id} [put]
func (h *DriverStatus) Update(c *gin.Context) {
	id, ok := domainID(c)
	if !ok {
		return
	}
	var input model.DriverStatus
	if err := c.ShouldBindJSON(&input); err != nil {
		writeDomainError(c, &model.ValidationError{Message: "Некорректные данные запроса"})
		return
	}
	input.ID = id
	err := h.s.Update(c.Request.Context(), &input)
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, apperrors.SuccessResponse{Message: "updated"})
}

// @Summary Удалить запись статуса водителя
// @Description Удаляет запись статуса водителя по ID. Требуется роль admin.
// @Tags admin/driver/status
// @Security Bearer
// @Param id path int true "Положительный ID записи" minimum(1)
// @Success 204 "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 403 {object} apperrors.ErrorResponse "Требуются права администратора"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/driver/status/{id} [delete]
func (h *DriverStatus) Delete(c *gin.Context) {
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
