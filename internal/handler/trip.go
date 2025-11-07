package handler

import (
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/model"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

type Trip struct {
	logger *logger.Logger
	s      service.Trip
}

func NewTrip(logger *logger.Logger, s service.Trip) *Trip {
	return &Trip{
		logger: logger,
		s:      s,
	}
}

// All retrieves a list of all trips
// @Summary Получить список всех маршрутов
// @Description Возвращает список всех маршрутов в системе
// @Tags trips
// @Produce json
// @Success 200 {array} model.Trip "Список маршрутов"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /trips [get]
func (h *Trip) All(c *gin.Context) {
	h.logger.Debug("trips all")
	trips, err := h.s.All(c.Request.Context())
	if err != nil {
		h.logger.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, trips)
}

// ByID retrieves a single row of trip
// @Summary Получить маршрут
// @Description Возвращает маршрут в системе по ID
// @Param id path int true "ID маршрута"
// @Tags trips
// @Produce json
// @Success 200 {object} model.Trip "Модель пути"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /trips/{id} [get]
func (h *Trip) ByID(c *gin.Context) {
	h.logger.Debug("trips by id")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		h.logger.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}
	trip, err := h.s.ById(c.Request.Context(), id)
	if err != nil {
		h.logger.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, trip)
}

// Create creates a new trip (Admin only)
// @Summary Создать новый маршрут (только админ)
// @Description Создает новую запись о маршруте (требуются права администратора)
// @Security Bearer
// @Tags admin/trips
// @Accept json
// @Produce json
// @Param input body model.Trip true "Данные маршрута"
// @Success 201 {object} apperrors.SuccessResponse "Маршрут успешно создан"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректные данные"
// @Failure 401 {object} apperrors.ErrorResponse "Не авторизован"
// @Failure 403 {object} apperrors.ErrorResponse "Доступ запрещен"
// @Failure 500 {object} apperrors.ErrorResponse "Ошибка сервера"
// @Router /admin/trips [post]
func (h *Trip) Create(c *gin.Context) {
	h.logger.Debug("trips create")
	var trip model.Trip
	err := c.ShouldBindJSON(&trip)
	if err != nil {
		h.logger.Error(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"message": err.Error(),
		})
		return
	}
	err = h.s.Create(c.Request.Context(), &trip)
	if err != nil {
		h.logger.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusCreated, trip)
}

// Update updates an existing trip (Admin only)
// @Summary Обновить данные о маршруте (только админ)
// @Description Обновляет информацию о маршруте по ID (требуются права администратора)
// @Security Bearer
// @Tags admin/trips
// @Accept json
// @Produce json
// @Param id path int true "ID маршрута"
// @Param input body model.TripUpdate true "Обновленные данные маршрута"
// @Success 200 {object} apperrors.SuccessResponse "Данные маршрута обновлены"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректные данные"
// @Failure 401 {object} apperrors.ErrorResponse "Не авторизован"
// @Failure 403 {object} apperrors.ErrorResponse "Доступ запрещен"
// @Failure 404 {object} apperrors.ErrorResponse "Водитель не найден"
// @Failure 500 {object} apperrors.ErrorResponse "Ошибка сервера"
// @Router /admin/trips/{id} [put]
func (h *Trip) Update(c *gin.Context) {
	h.logger.Debug("trips update")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		h.logger.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
	}
	var trip model.TripUpdate
	err = c.ShouldBindJSON(&trip)
	if err != nil {
		h.logger.Error(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"message": err.Error(),
		})
		return
	}
	trip.ID = id
	err = h.s.Update(c.Request.Context(), &trip)
	if err != nil {
		h.logger.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
	}
	c.JSON(http.StatusOK, trip)
}

// Delete removes a trip by ID (Admin only)
// @Summary Удалить маршрут (только админ)
// @Description Удаляет запись о маршруте по ID (требуются права администратора)
// @Security Bearer
// @Tags admin/trips
// @Produce json
// @Param id path int true "ID маршрута"
// @Success 204 "Маршрут успешно удален"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный ID"
// @Failure 401 {object} apperrors.ErrorResponse "Не авторизован"
// @Failure 403 {object} apperrors.ErrorResponse "Доступ запрещен"
// @Failure 404 {object} apperrors.ErrorResponse "Маршрут не найден"
// @Failure 500 {object} apperrors.ErrorResponse "Ошибка сервера"
// @Router /admin/trips/{id} [delete]
func (h *Trip) Delete(c *gin.Context) {
	h.logger.Debug("trips delete")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		h.logger.Error(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"message": err.Error(),
		})
		return
	}
	err = h.s.Delete(c.Request.Context(), id)
	if err != nil {
		h.logger.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusNoContent, gin.H{
		"message": "ok",
	})
}
