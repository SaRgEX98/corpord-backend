package handler

import (
	"corpord-api/internal/apperrors"
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/model"
	"github.com/gin-gonic/gin"
	"net/http"
)

// BusCategoryHandler обрабатывает HTTP-запросы для работы с категориями автобусов
type BusCategoryHandler struct {
	logger *logger.Logger
	bc     service.BusCategory
}

func NewBusCategory(logger *logger.Logger, bc service.BusCategory) *BusCategoryHandler {
	return &BusCategoryHandler{
		logger: logger,
		bc:     bc,
	}
}

// @Summary Получить список категорий автобусов
// @Description Возвращает список категорий автобусов с идентификаторами записей.
// @Tags bus/categories
// @Produce json
// @Success 200 {array} model.BusCategory "Успешный ответ"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /bus/categories/ [get]
func (h *BusCategoryHandler) GetAll(c *gin.Context) {
	output, err := h.bc.GetAll(c.Request.Context())
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, output)
}

// @Summary Получить запись категории автобусов
// @Description Возвращает данные категории автобусов по ID.
// @Tags bus/categories
// @Produce json
// @Param id path int true "Положительный ID записи" minimum(1)
// @Success 200 {object} model.BusCategory "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /bus/categories/{id} [get]
func (h *BusCategoryHandler) GetById(c *gin.Context) {
	id, ok := domainID(c)
	if !ok {
		return
	}
	output, err := h.bc.GetById(c.Request.Context(), id)
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, output)
}

// @Summary Создать запись категории автобусов
// @Description Создает запись категории автобусов. Требуется роль admin. Поле name обязательно; id назначается сервером.
// @Tags admin/bus/categories
// @Security Bearer
// @Accept json
// @Produce json
// @Param input body model.BusCategory true "Данные запроса"
// @Success 201 {object} apperrors.SuccessResponse "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 403 {object} apperrors.ErrorResponse "Требуются права администратора"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/bus/categories/ [post]
func (h *BusCategoryHandler) Create(c *gin.Context) {
	var input model.BusCategory
	if err := c.ShouldBindJSON(&input); err != nil {
		writeDomainError(c, &model.ValidationError{Message: "Некорректные данные запроса"})
		return
	}
	err := h.bc.Create(c.Request.Context(), input)
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusCreated, apperrors.SuccessResponse{Message: "created"})
}

// @Summary Удалить запись категории автобусов
// @Description Удаляет запись категории автобусов по ID. Требуется роль admin.
// @Tags admin/bus/categories
// @Security Bearer
// @Param id path int true "Положительный ID записи" minimum(1)
// @Success 204 "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 403 {object} apperrors.ErrorResponse "Требуются права администратора"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/bus/categories/{id} [delete]
func (h *BusCategoryHandler) Delete(c *gin.Context) {
	id, ok := domainID(c)
	if !ok {
		return
	}
	err := h.bc.Delete(c.Request.Context(), id)
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Обновить запись категории автобусов
// @Description Обновляет запись категории автобусов по ID из URL. Требуется роль admin. Поле name обязательно; id в JSON игнорируется.
// @Tags admin/bus/categories
// @Security Bearer
// @Accept json
// @Produce json
// @Param id path int true "Положительный ID записи" minimum(1)
// @Param input body model.BusCategory true "Данные запроса"
// @Success 200 {object} model.BusCategory "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 403 {object} apperrors.ErrorResponse "Требуются права администратора"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/bus/categories/{id} [put]
func (h *BusCategoryHandler) Update(c *gin.Context) {
	id, ok := domainID(c)
	if !ok {
		return
	}
	var input model.BusCategory
	if err := c.ShouldBindJSON(&input); err != nil {
		writeDomainError(c, &model.ValidationError{Message: "Некорректные данные запроса"})
		return
	}
	input.ID = id
	output, err := h.bc.Update(c.Request.Context(), input)
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, output)
}
