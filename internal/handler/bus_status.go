package handler

import (
	"corpord-api/internal/apperrors"
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/model"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
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

// All godoc
// @Summary Получить все статусы автобусов
// @Description Возвращает список всех доступных статусов автобусов. Доступно всем пользователям
// @Tags bus/statuses
// @Produce json
// @Success 200 {array} model.BusStatus "Список статусов автобусов"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /bus/statuses [get]
func (h *BusStatusHandler) All(c *gin.Context) {
	output, err := h.bs.All(c.Request.Context())
	if err != nil {
		if errors.Is(err, service.ErrBusStatusNotFound) {
			c.JSON(apperrors.ErrNotFound.Status, apperrors.ErrorResponse{Error: apperrors.ErrNotFound.Message})
			return
		}
		c.JSON(apperrors.ErrBadRequest.Status, apperrors.ErrorResponse{Error: apperrors.ErrBadRequest.Message})
		return
	}

	c.JSON(http.StatusOK, output)
}

// ByID godoc
// @Summary Получить статус автобуса по ID
// @Description Возвращает данные статуса автобуса по его идентификатору
// @Tags bus/statuses
// @Produce json
// @Param id path int true "ID статуса"
// @Success 200 {object} model.BusStatus "Данные статуса"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный ID"
// @Failure 404 {object} apperrors.ErrorResponse "Статус не найден"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /bus/statuses/{id} [get]
func (h *BusStatusHandler) ByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(apperrors.ErrBadRequest.Status, apperrors.ErrorResponse{Error: apperrors.ErrBadRequest.Message})
		return
	}

	output, err := h.bs.ByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrBusStatusNotFound) {
			c.JSON(apperrors.ErrNotFound.Status, apperrors.ErrorResponse{Error: apperrors.ErrNotFound.Message})
			return
		}
		c.JSON(apperrors.ErrBadRequest.Status, apperrors.ErrorResponse{Error: apperrors.ErrBadRequest.Message})
		return
	}

	c.JSON(http.StatusOK, output)
}

// Create godoc
// @Summary Создать новый статус автобуса
// @Description Создает новую запись статуса автобуса. Доступно только администраторам
// @Tags admin/bus/statuses
// @Accept json
// @Produce json
// @Security Bearer
// @Param input body model.BusStatus true "Данные статуса"
// @Success 201 {object} apperrors.SuccessResponse "Статус успешно создан"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректные данные"
// @Failure 401 {object} apperrors.ErrorResponse "Не авторизован"
// @Failure 403 {object} apperrors.ErrorResponse "Доступ запрещен. Требуются права администратора"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/bus/statuses [post]
func (h *BusStatusHandler) Create(c *gin.Context) {
	var req model.BusStatus
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(apperrors.ErrBadRequest.Status, apperrors.ErrorResponse{Error: apperrors.ErrBadRequest.Message})
		return
	}

	err := h.bs.Create(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrBusStatusNotFound) {
			c.JSON(apperrors.ErrNotFound.Status, apperrors.ErrorResponse{Error: apperrors.ErrNotFound.Message})
			return
		}
		c.JSON(apperrors.ErrBadRequest.Status, apperrors.ErrorResponse{Error: apperrors.ErrBadRequest.Message})
		return
	}

	c.JSON(http.StatusCreated, apperrors.SuccessResponse{Message: "Статус успешно создан"})
}

// Update godoc
// @Summary Обновить статус автобуса
// @Description Обновляет данные статуса автобуса по его идентификатору. Доступно только администраторам
// @Tags admin/bus/statuses
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "ID статуса"
// @Param input body model.BusStatus true "Обновленные данные статуса"
// @Success 200 {object} model.BusStatus "Обновленные данные статуса"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректные данные"
// @Failure 401 {object} apperrors.ErrorResponse "Не авторизован"
// @Failure 403 {object} apperrors.ErrorResponse "Доступ запрещен. Требуются права администратора"
// @Failure 404 {object} apperrors.ErrorResponse "Статус не найден"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/bus/statuses/{id} [put]
func (h *BusStatusHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(apperrors.ErrBadRequest.Status, apperrors.ErrorResponse{Error: apperrors.ErrBadRequest.Message})
		return
	}

	var req model.BusStatus
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(apperrors.ErrBadRequest.Status, apperrors.ErrorResponse{Error: apperrors.ErrBadRequest.Message})
		return
	}

	req.ID = id
	status, err := h.bs.Update(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrBusStatusNotFound) {
			c.JSON(apperrors.ErrNotFound.Status, apperrors.ErrorResponse{Error: apperrors.ErrNotFound.Message})
			return
		}
		c.JSON(apperrors.ErrBadRequest.Status, apperrors.ErrorResponse{Error: apperrors.ErrBadRequest.Message})
		return
	}

	c.JSON(http.StatusOK, status)
}

// Delete godoc
// @Summary Удалить статус автобуса
// @Description Удаляет статус автобуса по его идентификатору. Доступно только администраторам
// @Tags admin/bus/statuses
// @Produce json
// @Security Bearer
// @Param id path int true "ID статуса"
// @Success 204 "Статус успешно удален"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный ID"
// @Failure 401 {object} apperrors.ErrorResponse "Не авторизован"
// @Failure 403 {object} apperrors.ErrorResponse "Доступ запрещен. Требуются права администратора"
// @Failure 404 {object} apperrors.ErrorResponse "Статус не найден"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/bus/statuses/{id} [delete]
func (h *BusStatusHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(apperrors.ErrBadRequest.Status, apperrors.ErrorResponse{Error: apperrors.ErrBadRequest.Message})
		return
	}

	err = h.bs.Delete(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrBusStatusNotFound) {
			c.JSON(apperrors.ErrNotFound.Status, apperrors.ErrorResponse{Error: apperrors.ErrNotFound.Message})
			return
		}
		c.JSON(apperrors.ErrBadRequest.Status, apperrors.ErrorResponse{Error: apperrors.ErrBadRequest.Message})
		return
	}

	c.Status(http.StatusNoContent)
}
