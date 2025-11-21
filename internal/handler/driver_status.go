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

// All возвращает список всех статусов водителя
// @Summary Получить все статусы водителя
// @Description Возвращает список всех статусов водителя. Доступно всем пользователям
// @Tags driver/status
// @Produce json
// @Success 200 {array} model.DriverStatus "Список статусов водителя"
// @Failure 404 {object} apperrors.ErrorResponse "Ничего не найдено"
// @Router /driver/status [get]
func (h *DriverStatus) All(c *gin.Context) {
	h.logger.Debug("All")
	output := h.s.All(c.Request.Context())
	if output == nil {
		c.AbortWithError(apperrors.ErrNotFound.Status, apperrors.ErrNotFound)
		return
	}
	c.JSON(http.StatusOK, output)
}

// ById возвращает статус водителя по ID
// @Summary Получить статус водителя по ID
// @Description Возвращает информацию о статусе водителя по его идентификатору. Доступно всем пользователям
// @Tags driver/status
// @Produce json
// @Param id path int true "ID статуса"
// @Success 200 {object} model.DriverStatus "Данные статуса"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный ID"
// @Failure 404 {object} apperrors.ErrorResponse "Категория не найдена"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /bus/categories/{id} [get]
func (h *DriverStatus) ById(c *gin.Context) {
	h.logger.Debug("ById")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		h.logger.Error(err)
		c.JSON(http.StatusBadRequest, apperrors.ErrBadRequest)
		return
	}
	output, err := h.s.ById(c.Request.Context(), id)
	if err != nil {
		h.logger.Error(err)
		c.AbortWithError(apperrors.ErrBadRequest.Status, err)
	}
	if output.Name == "" {
		h.logger.Error(err)
		c.JSON(apperrors.ErrNotFound.Status, apperrors.ErrNotFound.Message)
		return
	}
	c.JSON(http.StatusOK, output)
}

// Create создаёт статус водителя
// @Summary Создать статус водителя
// @Description Создание статуса водителя
// @Tags admin/driver/status
// @Produce json
// @Param id path int true "ID статуса"
// @Success 200 {object} model.DriverStatus "Данные статуса"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный ID"
// @Failure 404 {object} apperrors.ErrorResponse "Категория не найдена"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/driver/status [post]
func (h *DriverStatus) Create(c *gin.Context) {
	h.logger.Debug("Create")
	var status model.DriverStatus
	err := c.ShouldBindJSON(&status)
	if err != nil {
		h.logger.Error(err)
		c.AbortWithError(apperrors.ErrBadRequest.Status, err)
	}
	err = h.s.Create(c.Request.Context(), &status)
	if err != nil {
		h.logger.Error(err)
		c.AbortWithError(apperrors.ErrBadRequest.Status, err)
	}
	c.JSON(http.StatusOK, apperrors.SuccessResponse{
		Message: "created",
	})
}

// ById измменяет статус по ID
// @Summary Изменить статус по ID
// @Description Изменение статуса по ID
// @Tags admin/driver/status
// @Produce json
// @Param id path int true "ID статуса"
// @Success 200 {object} model.DriverStatus "Данные статуса"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный ID"
// @Failure 404 {object} apperrors.ErrorResponse "Категория не найдена"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/driver/status/{id} [put]
func (h *DriverStatus) Update(c *gin.Context) {
	h.logger.Debug("Update")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		h.logger.Error(err)
		c.JSON(http.StatusBadRequest, apperrors.ErrBadRequest)
	}
	var status model.DriverStatus
	status.ID = id
	err = c.ShouldBindJSON(&status)
	if err != nil {
		h.logger.Error(err)
		c.AbortWithError(apperrors.ErrBadRequest.Status, err)
	}
	err = h.s.Update(c.Request.Context(), &status)
	if err != nil {
		h.logger.Error(err)
		c.AbortWithError(apperrors.ErrBadRequest.Status, err)
	}
	c.JSON(http.StatusOK, apperrors.SuccessResponse{
		Message: "updated",
	})
}

// Delete удаляет статус водителя
// @Summary Удалить статус водителя
// @Description Удаление статуса водителя по ID
// @Tags admin/driver/status
// @Produce json
// @Param id path int true "ID статуса"
// @Success 204 {object} model.DriverStatus "Данные статуса"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный ID"
// @Failure 404 {object} apperrors.ErrorResponse "Категория не найдена"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/driver/status/{id} [delete]
func (h *DriverStatus) Delete(c *gin.Context) {
	h.logger.Debug("Delete")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		h.logger.Error(err)
		c.AbortWithError(apperrors.ErrBadRequest.Status, apperrors.ErrBadRequest)
	}
	err = h.s.Delete(c.Request.Context(), id)
	if err != nil {
		h.logger.Error(err)
		c.AbortWithError(apperrors.ErrBadRequest.Status, err)
	}
	c.JSON(http.StatusNoContent, apperrors.SuccessResponse{
		Message: "deleted",
	})
}
