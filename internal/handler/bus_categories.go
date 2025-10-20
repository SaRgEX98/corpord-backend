package handler

import (
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/model"
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

func (h *BusCategoryHandler) Create(c *gin.Context) {
	var input model.BusCategory
	err := c.ShouldBindJSON(&input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err,
		})
		return
	}
	err = h.bc.Create(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status": "created",
	})
}

func (h *BusCategoryHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, err)
	}

	err = h.bc.Delete(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusBadRequest, err)
	}

	c.JSON(http.StatusNoContent, gin.H{
		"status": "deleted",
	})
}

func (h *BusCategoryHandler) Update(c *gin.Context) {
	var category model.BusCategory
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err,
		})
		return
	}

	category.ID = id
	err = c.ShouldBindJSON(&category)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err,
		})
		return
	}

	err = h.bc.Update(c.Request.Context(), category)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "updated",
	})
}
