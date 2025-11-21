package handler

import (
	"corpord-api/internal/apperrors"
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/model"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

type Driver struct {
	logger *logger.Logger
	s      service.Driver
}

func NewDriver(logger *logger.Logger, s service.Driver) *Driver {
	return &Driver{
		logger: logger,
		s:      s,
	}
}

// All retrieves a list of all drivers
// @Summary Получить список всех водителей
// @Description Возвращает список всех водителей в системе
// @Tags drivers
// @Produce json
// @Success 200 {array} model.Driver "Список водителей"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /driver [get]
func (h *Driver) All(c *gin.Context) {
	drivers, err := h.s.All(c.Request.Context())
	if err != nil {
		h.logger.Error(err)
		c.JSON(http.StatusInternalServerError, apperrors.ErrInternal.Message)
		return
	}
	c.JSON(http.StatusOK, drivers)
}

// ByID retrieves a single driver by ID
// @Summary Получить водителя по ID
// @Description Возвращает информацию о водителе по его идентификатору
// @Tags drivers
// @Produce json
// @Param id path int true "ID водителя"
// @Success 200 {object} model.Driver "Данные водителя"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный ID"
// @Failure 404 {object} apperrors.ErrorResponse "Водитель не найден"
// @Router /driver/{id} [get]
func (h *Driver) ByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		h.logger.Error(err)
		c.AbortWithError(apperrors.ErrBadRequest.Status, err)
		return
	}
	driver, err := h.s.ByID(c.Request.Context(), id)
	if err != nil {
		h.logger.Error(err)
		c.AbortWithError(apperrors.ErrNotFound.Status, err)
		return
	}
	c.JSON(http.StatusOK, driver)
}

// Create creates a new driver (Admin only)
// @Summary Создать нового водителя (только админ)
// @Description Создает новую запись о водителе (требуются права администратора)
// @Security Bearer
// @Tags admin/driver
// @Accept json
// @Produce json
// @Param input body model.DriverInput true "Данные водителя"
// @Success 201 {object} apperrors.SuccessResponse "Водитель успешно создан"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректные данные"
// @Failure 401 {object} apperrors.ErrorResponse "Не авторизован"
// @Failure 403 {object} apperrors.ErrorResponse "Доступ запрещен"
// @Failure 500 {object} apperrors.ErrorResponse "Ошибка сервера"
// @Router /admin/driver [post]
func (h *Driver) Create(c *gin.Context) {
	var driver model.DriverInput
	err := c.ShouldBind(&driver)
	if err != nil {
		h.logger.Error(err)
		c.AbortWithError(apperrors.ErrBadRequest.Status, err)
	}

	err = h.s.Create(c.Request.Context(), driver)
	if err != nil {
		h.logger.Error(err)
		c.AbortWithError(apperrors.ErrInternal.Status, err)
	}
	c.JSON(http.StatusCreated, apperrors.SuccessResponse{
		Message: "created",
	})
}

// Update updates an existing driver (Admin only)
// @Summary Обновить данные водителя (только админ)
// @Description Обновляет информацию о водителе по ID (требуются права администратора)
// @Security Bearer
// @Tags admin/driver
// @Accept json
// @Produce json
// @Param id path int true "ID водителя"
// @Param input body model.DriverInput true "Обновленные данные водителя"
// @Success 200 {object} apperrors.SuccessResponse "Данные водителя обновлены"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректные данные"
// @Failure 401 {object} apperrors.ErrorResponse "Не авторизован"
// @Failure 403 {object} apperrors.ErrorResponse "Доступ запрещен"
// @Failure 404 {object} apperrors.ErrorResponse "Водитель не найден"
// @Failure 500 {object} apperrors.ErrorResponse "Ошибка сервера"
// @Router /admin/driver/{id} [put]
func (h *Driver) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		h.logger.Error(err)
		c.AbortWithError(apperrors.ErrBadRequest.Status, err)
		return
	}

	var driver model.DriverInput
	driver.ID = id
	err = c.ShouldBind(&driver)
	if err != nil {
		h.logger.Error(err)
		c.AbortWithError(apperrors.ErrBadRequest.Status, err)
		return
	}
	err = h.s.Update(c.Request.Context(), driver)
	if err != nil {
		h.logger.Error(err)
		c.AbortWithError(apperrors.ErrInternal.Status, err)
		return
	}

	c.JSON(http.StatusOK, apperrors.SuccessResponse{
		Message: "updated",
	})
}

// Delete removes a driver by ID (Admin only)
// @Summary Удалить водителя (только админ)
// @Description Удаляет запись о водителе по ID (требуются права администратора)
// @Security Bearer
// @Tags admin/driver
// @Produce json
// @Param id path int true "ID водителя"
// @Success 204 "Водитель успешно удален"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный ID"
// @Failure 401 {object} apperrors.ErrorResponse "Не авторизован"
// @Failure 403 {object} apperrors.ErrorResponse "Доступ запрещен"
// @Failure 404 {object} apperrors.ErrorResponse "Водитель не найден"
// @Failure 500 {object} apperrors.ErrorResponse "Ошибка сервера"
// @Router /admin/driver/{id} [delete]
func (h *Driver) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		h.logger.Error(err)
		c.AbortWithError(apperrors.ErrBadRequest.Status, err)
		return
	}
	err = h.s.Delete(c.Request.Context(), id)
	if err != nil {
		h.logger.Error(err)
		c.AbortWithError(apperrors.ErrInternal.Status, err)
		return
	}
	c.JSON(http.StatusNoContent, apperrors.SuccessResponse{
		Message: "deleted",
	})
}
