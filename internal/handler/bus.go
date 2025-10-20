package handler

import (
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/model"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
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

func (h *BusHandler) InitRoutes(v1 *gin.RouterGroup) {
	v1.GET("/", h.GetAllBuses)
	v1.GET("/:id", h.GetBus)
	admin := v1.Group("/admin/bus")
	{
		admin.POST("/", h.CreateBus)
		admin.PUT("/:id", h.UpdateBus)
		admin.DELETE("/:id", h.DeleteBus)
	}
}

// GetAllBuses retrieves a list of all buses
// @Summary Получить список всех автобусов
// @Description Возвращает список всех автобусов в системе
// @Tags buses
// @Produce json
// @Success 200 {array} model.Bus "Список автобусов"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /api/v1/buses [get]
func (h *BusHandler) GetAllBuses(c *gin.Context) {
	buses, err := h.bus.GetAllBuses(c.Request.Context())
	if err != nil {
		h.logger.Errorf("failed to get all buses: %v", err)
		c.JSON(ErrInternal.Status, ErrorResponse{Error: ErrInternal.Message})
		return
	}

	c.JSON(http.StatusOK, buses)
}

// GetBus retrieves a single bus by ID
// @Summary Получить автобус по ID
// @Description Возвращает информацию об автобусе по его идентификатору
// @Tags buses
// @Produce json
// @Param id path int true "ID автобуса"
// @Success 200 {object} model.Bus "Данные автобуса"
// @Failure 400 {object} ErrorResponse "Некорректный ID"
// @Failure 404 {object} ErrorResponse "Автобус не найден"
// @Router /api/v1/buses/{id} [get]
func (h *BusHandler) GetBus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(ErrBadRequest.Status, ErrorResponse{Error: "Некорректный ID автобуса"})
		return
	}

	bus, err := h.bus.GetBus(c.Request.Context(), id)
	if err != nil {
		h.logger.Errorf("failed to get bus %d: %v", id, err)
		if errors.Is(err, service.ErrBusNotFound) {
			c.JSON(ErrNotFound.Status, ErrorResponse{Error: "Автобус не найден"})
			return
		}
		c.JSON(ErrInternal.Status, ErrorResponse{Error: ErrInternal.Message})
		return
	}

	c.JSON(http.StatusOK, bus)
}

// @Router /api/v1/admin/bus [post]
// CreateBus creates a new bus (Admin only)
// @Summary Создать новый автобус (только админ)
// @Description Создает новую запись об автобусе (требуются права администратора)
// @Security ApiKeyAuth
// @Tags admin
// @Accept json
// @Produce json
// @Param input body model.Bus true "Данные автобуса"
// @Success 200 {object} SuccessResponse "Автобус успешно создан"
// @Failure 400 {object} ErrorResponse "Некорректные данные"
// @Failure 401 {object} ErrorResponse "Не авторизован"
// @Failure 403 {object} ErrorResponse "Доступ запрещен"
// @Failure 500 {object} ErrorResponse "Ошибка сервера"
// @Router /api/v1/admin/bus [post]
func (h *BusHandler) CreateBus(c *gin.Context) {
	var bus model.Bus
	if err := c.ShouldBindJSON(&bus); err != nil {
		h.logger.Warnf("invalid request body: %v", err)
		c.JSON(ErrBadRequest.Status, ErrorResponse{Error: "Некорректные данные автобуса"})
		return
	}

	if err := h.bus.CreateBus(c.Request.Context(), bus); err != nil {
		h.logger.Errorf("failed to create bus: %v", err)
		c.JSON(ErrInternal.Status, ErrorResponse{Error: "Не удалось создать автобус"})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{Message: "Автобус успешно создан"})
}

// UpdateBus updates an existing bus (Admin only)
// @Summary Обновить данные автобуса (только админ)
// @Description Обновляет информацию об автобусе по ID (требуются права администратора)
// @Security ApiKeyAuth
// @Tags admin
// @Accept json
// @Produce json
// @Param id path int true "ID автобуса"
// @Param input body model.BusUpdate true "Обновленные данные автобуса"
// @Success 200 {object} SuccessResponse "Данные автобуса обновлены"
// @Failure 400 {object} ErrorResponse "Некорректные данные"
// @Failure 401 {object} ErrorResponse "Не авторизован"
// @Failure 403 {object} ErrorResponse "Доступ запрещен"
// @Failure 404 {object} ErrorResponse "Автобус не найден"
// @Failure 500 {object} ErrorResponse "Ошибка сервера"
// @Router /api/v1/admin/bus/{id} [put]
func (h *BusHandler) UpdateBus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(ErrBadRequest.Status, ErrorResponse{Error: "Некорректный ID автобуса"})
		return
	}

	var update model.BusUpdate
	update.ID = id
	if err := c.ShouldBindJSON(&update); err != nil {
		h.logger.Warnf("invalid request body: %v", err)
		c.JSON(ErrBadRequest.Status, ErrorResponse{Error: "Некорректные данные для обновления"})
		return
	}

	if err := h.bus.UpdateBus(c.Request.Context(), update); err != nil {
		h.logger.Errorf("failed to update bus %d: %v", id, err)
		if errors.Is(err, service.ErrBusNotFound) {
			c.JSON(ErrNotFound.Status, ErrorResponse{Error: "Автобус не найден"})
			return
		}
		c.JSON(ErrInternal.Status, ErrorResponse{Error: "Не удалось обновить данные автобуса"})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{Message: "Данные автобуса успешно обновлены"})
}

// DeleteBus removes a bus by ID (Admin only)
// @Summary Удалить автобус (только админ)
// @Description Удаляет запись об автобусе по ID (требуются права администратора)
// @Security ApiKeyAuth
// @Tags admin
// @Produce json
// @Param id path int true "ID автобуса"
// @Success 204 "Автобус успешно удален"
// @Failure 400 {object} ErrorResponse "Некорректный ID"
// @Failure 401 {object} ErrorResponse "Не авторизован"
// @Failure 403 {object} ErrorResponse "Доступ запрещен"
// @Failure 404 {object} ErrorResponse "Автобус не найден"
// @Failure 500 {object} ErrorResponse "Ошибка сервера"
// @Router /api/v1/admin/bus/{id} [delete]
func (h *BusHandler) DeleteBus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(ErrBadRequest.Status, ErrorResponse{Error: "Некорректный ID автобуса"})
		return
	}

	if err := h.bus.DeleteBus(c.Request.Context(), id); err != nil {
		h.logger.Errorf("failed to delete bus %d: %v", id, err)
		if errors.Is(err, service.ErrBusNotFound) {
			c.JSON(ErrNotFound.Status, ErrorResponse{Error: "Автобус не найден"})
			return
		}
		c.JSON(ErrInternal.Status, ErrorResponse{Error: "Не удалось удалить автобус"})
		return
	}

	c.Status(http.StatusNoContent)
}
