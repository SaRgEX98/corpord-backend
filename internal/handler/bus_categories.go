package handler

import (
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

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

func (h *BusCategoryHandler) GetAll(c *gin.Context) {
	output, err := h.bc.GetAll(c.Request.Context())
	if err != nil {
		h.logger.Errorf("error while getting categories: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request",
		})
		return
	}
	c.JSON(http.StatusOK, output)

}

func (h *BusCategoryHandler) GetById(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid id",
		})
		return
	}

	output, err := h.bc.GetById(c.Request.Context(), id)
	if err != nil {
		h.logger.Errorf("failed to get category: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request",
		})
		return
	}

	c.JSON(http.StatusOK, output)

}
