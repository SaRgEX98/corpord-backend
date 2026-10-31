package handler

import (
	"corpord-api/internal/apperrors"
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/model"
	"github.com/gin-gonic/gin"
	"net/http"
)

type Stop interface {
	All(c *gin.Context)
	ByID(c *gin.Context)
	Create(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
}

type stop struct {
	logger *logger.Logger
	s      service.Stop
}

func NewStop(logger *logger.Logger, s service.Stop) Stop {
	return &stop{
		logger: logger,
		s:      s,
	}
}

// @Summary Получить список остановок
// @Description Возвращает список остановок с идентификаторами записей.
// @Tags stops
// @Produce json
// @Success 200 {array} model.Stop "Успешный ответ"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /stops/ [get]
func (s *stop) All(c *gin.Context) {
	output, err := s.s.All(c.Request.Context())
	if err != nil {
		s.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, output)
}

// @Summary Получить запись остановки
// @Description Возвращает данные остановки по ID.
// @Tags stops
// @Produce json
// @Param id path int true "Положительный ID записи" minimum(1)
// @Success 200 {object} model.Stop "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /stops/{id} [get]
func (s *stop) ByID(c *gin.Context) {
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

// @Summary Создать запись остановки
// @Description Создает запись остановки. Требуется роль admin.
// @Tags admin/stops
// @Security Bearer
// @Accept json
// @Produce json
// @Param input body model.Stop true "Данные запроса"
// @Success 201 {object} model.Stop "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 403 {object} apperrors.ErrorResponse "Требуются права администратора"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/stops/ [post]
func (s *stop) Create(c *gin.Context) {
	var input model.Stop
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

// @Summary Обновить запись остановки
// @Description Обновляет запись остановки по ID из URL. Требуется роль admin. Передавайте только изменяемые поля; пропущенные поля сохраняются.
// @Tags admin/stops
// @Security Bearer
// @Accept json
// @Produce json
// @Param id path int true "Положительный ID записи" minimum(1)
// @Param input body model.StopUpdate true "Данные запроса"
// @Success 200 {object} apperrors.SuccessResponse "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 403 {object} apperrors.ErrorResponse "Требуются права администратора"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/stops/{id} [put]
func (s *stop) Update(c *gin.Context) {
	id, ok := domainID(c)
	if !ok {
		return
	}
	var input model.StopUpdate
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

// @Summary Удалить запись остановки
// @Description Удаляет запись остановки по ID. Требуется роль admin.
// @Tags admin/stops
// @Security Bearer
// @Param id path int true "Положительный ID записи" minimum(1)
// @Success 204 "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 403 {object} apperrors.ErrorResponse "Требуются права администратора"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/stops/{id} [delete]
func (s *stop) Delete(c *gin.Context) {
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
