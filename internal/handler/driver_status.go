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

func (h *DriverStatus) All(c *gin.Context) {
	h.logger.Debug("All")
	output := h.s.All(c.Request.Context())
	if output == nil {
		c.AbortWithError(apperrors.ErrNotFound.Status, apperrors.ErrNotFound)
		return
	}
	c.JSON(http.StatusOK, output)
}

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
