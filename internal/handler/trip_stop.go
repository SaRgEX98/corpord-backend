package handler

import (
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/model"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
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

// All retrieves a list of trip stops
// @Summary Получить все остановки
// @Description Возвращает все остановки в системе
// @Tags trip_stops
// @Produce json
// @Success 200 {object} model.TripStop "Модель остановок"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /trip_stops [get]
func (s *tripStop) All(c *gin.Context) {
	s.logger.Debug("TripStops All")
	all, err := s.s.All(c.Request.Context())
	if err != nil {
		s.logger.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, all)
}

// ByID retrieves a single row of trip_stop
// @Summary Получить остановку
// @Description Возвращает остановку по ID
// @Param id path int true "ID остановки"
// @Tags trip_stops
// @Produce json
// @Success 200 {object} model.TripStop "Модель пути"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /trip_stops/{id} [get]
func (s *tripStop) ByID(c *gin.Context) {
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, output)
}

// Create creates a new stop (Admin only)
// @Summary Создать новую остановки (только админ)
// @Description Создает новую запись остановки (требуются права администратора)
// @Security Bearer
// @Tags admin/trip_stops
// @Accept json
// @Produce json
// @Param input body model.TripStop true "Данные остановки"
// @Success 201 {object} apperrors.SuccessResponse "Остановка успешно создана"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректные данные"
// @Failure 401 {object} apperrors.ErrorResponse "Не авторизован"
// @Failure 403 {object} apperrors.ErrorResponse "Доступ запрещен"
// @Failure 500 {object} apperrors.ErrorResponse "Ошибка сервера"
// @Router /admin/trip_stops [post]
func (s *tripStop) Create(c *gin.Context) {
	s.logger.Debug("TripStops Create")
	var stop model.TripStop
	if err := c.ShouldBindJSON(&stop); err != nil {
		s.logger.Error(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"message": err.Error(),
		})
		return
	}
	err := s.s.Create(c.Request.Context(), &stop)
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

// Update updates an existing stop (Admin only)
// @Summary Обновить данные об остановки (только админ)
// @Description Обновляет информацию об остановке по ID (требуются права администратора)
// @Security Bearer
// @Tags admin/trip_stops
// @Accept json
// @Produce json
// @Param id path int true "ID остановки"
// @Param input body model.TripStopUpdate true "Обновленные данные остановки"
// @Success 200 {object} apperrors.SuccessResponse "Данные остановки обновлены"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректные данные"
// @Failure 401 {object} apperrors.ErrorResponse "Не авторизован"
// @Failure 403 {object} apperrors.ErrorResponse "Доступ запрещен"
// @Failure 404 {object} apperrors.ErrorResponse "Водитель не найден"
// @Failure 500 {object} apperrors.ErrorResponse "Ошибка сервера"
// @Router /admin/trip_stops/{id} [put]
func (s *tripStop) Update(c *gin.Context) {
	s.logger.Debug("TripStops Update")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		s.logger.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}
	var stop model.TripStopUpdate
	if err = c.ShouldBindJSON(&stop); err != nil {
		s.logger.Error(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"message": err.Error(),
		})
		return
	}
	stop.ID = id
	err = s.s.Update(c.Request.Context(), &stop)
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
// @Tags admin/trip_stops
// @Produce json
// @Param id path int true "ID остановки"
// @Success 204 "Остановка успешно удалена"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный ID"
// @Failure 401 {object} apperrors.ErrorResponse "Не авторизован"
// @Failure 403 {object} apperrors.ErrorResponse "Доступ запрещен"
// @Failure 404 {object} apperrors.ErrorResponse "Остановка не найдена"
// @Failure 500 {object} apperrors.ErrorResponse "Ошибка сервера"
// @Router /admin/trip_stops/{id} [delete]
func (s *tripStop) Delete(c *gin.Context) {
	s.logger.Debug("TripStops Delete")
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
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
	})
}
