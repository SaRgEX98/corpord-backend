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

func (h *Driver) All(c *gin.Context) {
	drivers, err := h.s.All(c.Request.Context())
	if err != nil {
		h.logger.Error(err)
		c.JSON(http.StatusInternalServerError, apperrors.ErrInternal.Message)
		return
	}
	c.JSON(http.StatusOK, drivers)
}

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
