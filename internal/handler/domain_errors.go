package handler

import (
	"corpord-api/internal/apperrors"
	"corpord-api/internal/repository/pg"
	"corpord-api/internal/service"
	"corpord-api/model"
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func domainID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, apperrors.ErrorResponse{Error: "Некорректный ID"})
		return 0, false
	}
	return id, true
}

// writeDomainError exposes a stable API error and keeps SQL/driver errors private.
func writeDomainError(c *gin.Context, err error) {
	status, message := http.StatusInternalServerError, apperrors.ErrInternal.Message
	var validation *model.ValidationError
	switch {
	case errors.As(err, &validation):
		status, message = http.StatusBadRequest, validation.Message
	case errors.Is(err, apperrors.ErrBadRequest), errors.Is(err, service.ErrNoFields):
		status, message = http.StatusBadRequest, apperrors.ErrBadRequest.Message
	case errors.Is(err, sql.ErrNoRows), errors.Is(err, pg.ErrNotFound), errors.Is(err, pg.ErrNoRowsAffected),
		errors.Is(err, service.ErrUserNotFound), errors.Is(err, service.ErrBusNotFound),
		errors.Is(err, service.ErrBusCategoryNotFound), errors.Is(err, service.ErrBusStatusNotFound):
		status, message = http.StatusNotFound, apperrors.ErrNotFound.Message
	case errors.Is(err, pg.ErrAlreadyExists), errors.Is(err, pg.ErrUniqueViolation),
		errors.Is(err, service.ErrEmailExists), errors.Is(err, service.ErrBusCategoryExists), errors.Is(err, service.ErrBusStatusExists),
		pg.IsPgError(err, pg.ErrorCodeUniqueViolation):
		status, message = http.StatusConflict, "Запись с такими данными уже существует"
	case errors.Is(err, pg.ErrForeignKeyViolation), pg.IsPgError(err, pg.ErrorCodeForeignKeyViolation):
		status, message = http.StatusConflict, "Операция нарушает связь с другой записью"
	case pg.IsPgError(err, "23514"), pg.IsPgError(err, "23502"), pg.IsPgError(err, "22003"):
		status, message = http.StatusBadRequest, apperrors.ErrBadRequest.Message
	}
	c.JSON(status, apperrors.ErrorResponse{Error: message})
}
