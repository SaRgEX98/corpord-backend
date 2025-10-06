package handler

import (
	"corpord-api/internal/handler/middleware"
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/model"
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
	admin := v1.Group("/admin/bus", middleware.RoleMiddleware(model.RoleAdmin, h.logger))
	{
		admin.POST("/", h.CreateBus)
		admin.PUT("/:id", h.UpdateBus)
		admin.DELETE("/:id", h.DeleteBus)
	}
}

func (h *BusHandler) GetAllBuses(c *gin.Context) {
	buses, err := h.bus.GetAllBuses(c.Request.Context())
	if err != nil {
		h.logger.Errorf("failed to get all buses: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get all buses"})
		return
	}

	c.JSON(http.StatusOK, buses)
}

func (h *BusHandler) GetBus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bus ID"})
		return
	}

	bus, err := h.bus.GetBus(c.Request.Context(), id)
	if err != nil {
		h.logger.Errorf("failed to get bus %d: %v", id, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "bus not found"})
		return
	}

	c.JSON(http.StatusOK, bus)
}

func (h *BusHandler) CreateBus(c *gin.Context) {
	var bus model.Bus
	if err := c.ShouldBindJSON(&bus); err != nil {
		h.logger.Warnf("invalid request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	err := h.bus.CreateBus(c.Request.Context(), bus)
	if err != nil {
		h.logger.Errorf("failed to create bus: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create bus"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "bus created successfully"})
}

func (h *BusHandler) UpdateBus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bus ID"})
		return
	}

	var update model.BusUpdate
	update.ID = id
	if err := c.ShouldBindJSON(&update); err != nil {
		h.logger.Warnf("invalid request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	err = h.bus.UpdateBus(c.Request.Context(), update)
	if err != nil {
		h.logger.Errorf("failed to update bus %d: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update bus"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "bus updated successfully"})
}

func (h *BusHandler) DeleteBus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bus ID"})
		return
	}

	if err := h.bus.DeleteBus(c.Request.Context(), id); err != nil {
		h.logger.Errorf("failed to delete bus %d: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete bus"})
		return
	}

	c.Status(http.StatusNoContent)
}
