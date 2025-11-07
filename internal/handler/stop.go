package handler

import (
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/model"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
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

// All retrieves a list of stops
// @Summary Получить все остановки
// @Description Возвращает все остановки в системе
// @Tags stops
// @Produce json
// @Success 200 {object} model.Stop "Модель остановок"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /stops [get]
func (s *stop) All(c *gin.Context) {
	output, err := s.s.All(c.Request.Context())
	if err != nil {
		s.logger.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, output)
}

// ByID retrieves a single row of stop
// @Summary Получить остановку
// @Description Возвращает остановку по ID
// @Param id path int true "ID остановки"
// @Tags stop
// @Produce json
// @Success 200 {object} model.TripStop "Модель пути"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /stops/{id} [get]
func (s *stop) ByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		s.logger.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}
	output, err := s.s.ByID(c.Request.Context(), id)
	if err != nil {
		s.logger.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{})
		return
	}
	c.JSON(http.StatusOK, output)
}

// Create creates a new stop (Admin only)
// @Summary Создать новую остановки (только админ)
// @Description Создает новую запись остановки (требуются права администратора)
// @Security Bearer
// @Tags admin/stops
// @Accept json
// @Produce json
// @Param input body model.TripStop true "Данные остановки"
// @Success 201 {object} apperrors.SuccessResponse "Остановка успешно создана"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректные данные"
// @Failure 401 {object} apperrors.ErrorResponse "Не авторизован"
// @Failure 403 {object} apperrors.ErrorResponse "Доступ запрещен"
// @Failure 500 {object} apperrors.ErrorResponse "Ошибка сервера"
// @Router /admin/stops [post]
func (s *stop) Create(c *gin.Context) {
	var input model.Stop
	if err := c.ShouldBindJSON(&input); err != nil {
		s.logger.Error(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"message": err.Error(),
		})
		return
	}
	err := s.s.Create(c.Request.Context(), &input)
	if err != nil {
		s.logger.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"message": "success",
	})
}

// Update updates an existing stop (Admin only)
// @Summary Обновить данные об остановке (только админ)
// @Description Обновляет информацию об остановке по ID (требуются права администратора)
// @Security Bearer
// @Tags admin/stops
// @Accept json
// @Produce json
// @Param id path int true "ID остановки"
// @Param input body model.StopUpdate true "Обновленные данные остановки"
// @Success 200 {object} apperrors.SuccessResponse "Данные остановки обновлены"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректные данные"
// @Failure 401 {object} apperrors.ErrorResponse "Не авторизован"
// @Failure 403 {object} apperrors.ErrorResponse "Доступ запрещен"
// @Failure 404 {object} apperrors.ErrorResponse "Водитель не найден"
// @Failure 500 {object} apperrors.ErrorResponse "Ошибка сервера"
// @Router /admin/stops/{id} [put]
func (s *stop) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		s.logger.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}
	var input model.StopUpdate
	if err = c.ShouldBindJSON(&input); err != nil {
		s.logger.Error(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"message": err.Error(),
		})
		return
	}
	input.ID = id
	err = s.s.Update(c.Request.Context(), &input)
	if err != nil {
		s.logger.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
	})
}

// Delete removes a stop by ID (Admin only)
// @Summary Удалить остановки (только админ)
// @Description Удаляет запись об остановке по ID (требуются права администратора)
// @Security Bearer
// @Tags admin/stops
// @Produce json
// @Param id path int true "ID остановки"
// @Success 204 "Остановка успешно удалена"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный ID"
// @Failure 401 {object} apperrors.ErrorResponse "Не авторизован"
// @Failure 403 {object} apperrors.ErrorResponse "Доступ запрещен"
// @Failure 404 {object} apperrors.ErrorResponse "Остановка не найдена"
// @Failure 500 {object} apperrors.ErrorResponse "Ошибка сервера"
// @Router /admin/stops/{id} [delete]
func (s *stop) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		s.logger.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}
	err = s.s.Delete(c.Request.Context(), id)
	if err != nil {
		s.logger.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
	})
}
