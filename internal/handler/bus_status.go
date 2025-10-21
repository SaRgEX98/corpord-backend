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

	c.JSON(http.StatusOK, apperrors.SuccessResponse{Message: "created"})
}

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

	c.JSON(http.StatusNoContent, apperrors.SuccessResponse{Message: "deleted"})
}
