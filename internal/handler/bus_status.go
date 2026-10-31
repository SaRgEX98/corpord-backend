package handler

import (
	"corpord-api/internal/apperrors"
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/model"
	"github.com/gin-gonic/gin"
	"net/http"
)

type BusStatusHandler struct {
	logger *logger.Logger
	bs     service.BusStatus
}

func NewBusStatus(logger *logger.Logger, bs service.BusStatus) *BusStatusHandler {
	return &BusStatusHandler{
		logger: logger,
		bs:     bs,
	}
}

// @Summary Получить список статусов автобусов
// @Description Возвращает список статусов автобусов с идентификаторами записей.
// @Tags bus/statuses
// @Produce json
// @Success 200 {array} model.BusStatus "Успешный ответ"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /bus/statuses/ [get]
func (h *BusStatusHandler) All(c *gin.Context) {
	output, err := h.bs.All(c.Request.Context())
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, output)
}

// @Summary Получить запись статуса автобусов
// @Description Возвращает данные статуса автобусов по ID.
// @Tags bus/statuses
// @Produce json
// @Param id path int true "Положительный ID записи" minimum(1)
// @Success 200 {object} model.BusStatus "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /bus/statuses/{id} [get]
func (h *BusStatusHandler) ByID(c *gin.Context) {
	id, ok := domainID(c)
	if !ok {
		return
	}
	output, err := h.bs.ByID(c.Request.Context(), id)
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, output)
}

// @Summary Создать запись статуса автобусов
// @Description Создает запись статуса автобусов. Требуется роль admin. Поле name обязательно; id назначается сервером.
// @Tags admin/bus/statuses
// @Security Bearer
// @Accept json
// @Produce json
// @Param input body model.BusStatus true "Данные запроса"
// @Success 201 {object} apperrors.SuccessResponse "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 403 {object} apperrors.ErrorResponse "Требуются права администратора"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/bus/statuses/ [post]
func (h *BusStatusHandler) Create(c *gin.Context) {
	var input model.BusStatus
	if err := c.ShouldBindJSON(&input); err != nil {
		writeDomainError(c, &model.ValidationError{Message: "Некорректные данные запроса"})
		return
	}
	err := h.bs.Create(c.Request.Context(), input)
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusCreated, apperrors.SuccessResponse{Message: "created"})
}

// @Summary Обновить запись статуса автобусов
// @Description Обновляет запись статуса автобусов по ID из URL. Требуется роль admin. Поле name обязательно; id в JSON игнорируется.
// @Tags admin/bus/statuses
// @Security Bearer
// @Accept json
// @Produce json
// @Param id path int true "Положительный ID записи" minimum(1)
// @Param input body model.BusStatus true "Данные запроса"
// @Success 200 {object} model.BusStatus "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 403 {object} apperrors.ErrorResponse "Требуются права администратора"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/bus/statuses/{id} [put]
func (h *BusStatusHandler) Update(c *gin.Context) {
	id, ok := domainID(c)
	if !ok {
		return
	}
	var input model.BusStatus
	if err := c.ShouldBindJSON(&input); err != nil {
		writeDomainError(c, &model.ValidationError{Message: "Некорректные данные запроса"})
		return
	}
	input.ID = id
	output, err := h.bs.Update(c.Request.Context(), input)
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, output)
}

// @Summary Удалить запись статуса автобусов
// @Description Удаляет запись статуса автобусов по ID. Требуется роль admin.
// @Tags admin/bus/statuses
// @Security Bearer
// @Param id path int true "Положительный ID записи" minimum(1)
// @Success 204 "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 403 {object} apperrors.ErrorResponse "Требуются права администратора"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/bus/statuses/{id} [delete]
func (h *BusStatusHandler) Delete(c *gin.Context) {
	id, ok := domainID(c)
	if !ok {
		return
	}
	err := h.bs.Delete(c.Request.Context(), id)
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
