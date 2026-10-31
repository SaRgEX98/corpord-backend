package handler

import (
	"corpord-api/internal/apperrors"
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/model"
	"github.com/gin-gonic/gin"
	"net/http"
)

type BusHandler struct {
	bus    service.Bus
	logger *logger.Logger
}

func NewBus(logger *logger.Logger, bus service.Bus) *BusHandler {
	return &BusHandler{
		bus:    bus,
		logger: logger,
	}
}

// @Summary Получить список автобусов
// @Description Возвращает список автобусов с идентификаторами записей.
// @Tags bus
// @Produce json
// @Success 200 {array} model.ViewBus "Успешный ответ"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /bus/ [get]
func (h *BusHandler) GetAllBuses(c *gin.Context) {
	output, err := h.bus.GetAllBuses(c.Request.Context())
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, output)
}

// @Summary Получить запись автобуса
// @Description Возвращает данные автобуса по ID.
// @Tags bus
// @Produce json
// @Param id path int true "Положительный ID записи" minimum(1)
// @Success 200 {object} model.ViewBus "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /bus/{id} [get]
func (h *BusHandler) GetBus(c *gin.Context) {
	id, ok := domainID(c)
	if !ok {
		return
	}
	output, err := h.bus.GetBus(c.Request.Context(), id)
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, output)
}

// @Summary Создать запись автобуса
// @Description Создает запись автобуса. Требуется роль admin.
// @Tags admin/bus
// @Security Bearer
// @Accept json
// @Produce json
// @Param input body model.BusCreate true "Данные запроса"
// @Success 201 {object} apperrors.SuccessResponse "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 403 {object} apperrors.ErrorResponse "Требуются права администратора"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/bus/ [post]
func (h *BusHandler) CreateBus(c *gin.Context) {
	var input model.Bus
	if err := c.ShouldBindJSON(&input); err != nil {
		writeDomainError(c, &model.ValidationError{Message: "Некорректные данные запроса"})
		return
	}
	err := h.bus.CreateBus(c.Request.Context(), input)
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusCreated, apperrors.SuccessResponse{Message: "created"})
}

// @Summary Обновить запись автобуса
// @Description Обновляет запись автобуса по ID из URL. Требуется роль admin. Передавайте только изменяемые поля; пропущенные поля сохраняются.
// @Tags admin/bus
// @Security Bearer
// @Accept json
// @Produce json
// @Param id path int true "Положительный ID записи" minimum(1)
// @Param input body model.BusUpdate true "Данные запроса"
// @Success 200 {object} apperrors.SuccessResponse "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 403 {object} apperrors.ErrorResponse "Требуются права администратора"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/bus/{id} [put]
func (h *BusHandler) UpdateBus(c *gin.Context) {
	id, ok := domainID(c)
	if !ok {
		return
	}
	var input model.BusUpdate
	if err := c.ShouldBindJSON(&input); err != nil {
		writeDomainError(c, &model.ValidationError{Message: "Некорректные данные запроса"})
		return
	}
	input.ID = id
	err := h.bus.UpdateBus(c.Request.Context(), input)
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, apperrors.SuccessResponse{Message: "updated"})
}

// @Summary Удалить запись автобуса
// @Description Удаляет запись автобуса по ID. Требуется роль admin.
// @Tags admin/bus
// @Security Bearer
// @Param id path int true "Положительный ID записи" minimum(1)
// @Success 204 "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 403 {object} apperrors.ErrorResponse "Требуются права администратора"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/bus/{id} [delete]
func (h *BusHandler) DeleteBus(c *gin.Context) {
	id, ok := domainID(c)
	if !ok {
		return
	}
	err := h.bus.DeleteBus(c.Request.Context(), id)
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
