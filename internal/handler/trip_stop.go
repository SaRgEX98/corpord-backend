package handler

import (
	"corpord-api/internal/apperrors"
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/model"
	"github.com/gin-gonic/gin"
	"net/http"
)

type TripStop interface {
	All(c *gin.Context)
	ByID(c *gin.Context)
	Create(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
}

type tripStop struct {
	logger *logger.Logger
	s      service.TripStop
}

func NewTripStop(logger *logger.Logger, s service.TripStop) TripStop {
	return &tripStop{
		logger: logger,
		s:      s,
	}
}

// @Summary Получить список остановок рейсов
// @Description Возвращает список остановок рейсов с идентификаторами записей.
// @Tags trip_stops
// @Produce json
// @Success 200 {array} model.TripStop "Успешный ответ"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /trip_stops/ [get]
func (s *tripStop) All(c *gin.Context) {
	output, err := s.s.All(c.Request.Context())
	if err != nil {
		s.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, output)
}

// @Summary Получить запись остановки рейса
// @Description Возвращает данные остановки рейса по ID.
// @Tags trip_stops
// @Produce json
// @Param id path int true "Положительный ID записи" minimum(1)
// @Success 200 {object} model.TripStop "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /trip_stops/{id} [get]
func (s *tripStop) ByID(c *gin.Context) {
	id, ok := domainID(c)
	if !ok {
		return
	}
	output, err := s.s.ByID(c.Request.Context(), id)
	if err != nil {
		s.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, output)
}

// @Summary Создать запись остановки рейса
// @Description Создает запись остановки рейса. Требуется роль admin.
// @Tags admin/trip_stops
// @Security Bearer
// @Accept json
// @Produce json
// @Param input body model.TripStop true "Данные запроса"
// @Success 201 {object} model.TripStop "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 403 {object} apperrors.ErrorResponse "Требуются права администратора"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/trip_stops/ [post]
func (s *tripStop) Create(c *gin.Context) {
	var input model.TripStop
	if err := c.ShouldBindJSON(&input); err != nil {
		writeDomainError(c, &model.ValidationError{Message: "Некорректные данные запроса"})
		return
	}
	err := s.s.Create(c.Request.Context(), &input)
	if err != nil {
		s.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusCreated, input)
}

// @Summary Обновить запись остановки рейса
// @Description Обновляет запись остановки рейса по ID из URL. Требуется роль admin. Передавайте только изменяемые поля; пропущенные поля сохраняются.
// @Tags admin/trip_stops
// @Security Bearer
// @Accept json
// @Produce json
// @Param id path int true "Положительный ID записи" minimum(1)
// @Param input body model.TripStopUpdate true "Данные запроса"
// @Success 200 {object} apperrors.SuccessResponse "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 403 {object} apperrors.ErrorResponse "Требуются права администратора"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/trip_stops/{id} [put]
func (s *tripStop) Update(c *gin.Context) {
	id, ok := domainID(c)
	if !ok {
		return
	}
	var input model.TripStopUpdate
	if err := c.ShouldBindJSON(&input); err != nil {
		writeDomainError(c, &model.ValidationError{Message: "Некорректные данные запроса"})
		return
	}
	input.ID = id
	err := s.s.Update(c.Request.Context(), &input)
	if err != nil {
		s.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, apperrors.SuccessResponse{Message: "updated"})
}

// @Summary Удалить запись остановки рейса
// @Description Удаляет запись остановки рейса по ID. Требуется роль admin.
// @Tags admin/trip_stops
// @Security Bearer
// @Param id path int true "Положительный ID записи" minimum(1)
// @Success 204 "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 403 {object} apperrors.ErrorResponse "Требуются права администратора"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/trip_stops/{id} [delete]
func (s *tripStop) Delete(c *gin.Context) {
	id, ok := domainID(c)
	if !ok {
		return
	}
	err := s.s.Delete(c.Request.Context(), id)
	if err != nil {
		s.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
