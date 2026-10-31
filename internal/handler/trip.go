package handler

import (
	"corpord-api/internal/apperrors"
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/model"
	"github.com/gin-gonic/gin"
	"net/http"
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

// @Summary Получить полный список рейсов
// @Description Возвращает рейсы с id, вложенными bus и driver с собственными id и массивом stops, отсортированным по stop_order. Рейс без остановок содержит stops: [].
// @Tags trips
// @Produce json
// @Success 200 {array} model.TripResponse "Успешный ответ"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /trips/all [get]
func (h *Trip) All(c *gin.Context) {
	output, err := h.s.All(c.Request.Context())
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, output)
}

// @Summary Получить краткий список рейсов
// @Description Возвращает краткие карточки рейсов: trip_id, автобус, водитель, начальная и конечная остановки, время и базовая цена.
// @Tags trips
// @Produce json
// @Success 200 {array} model.TripShortInfo "Успешный ответ"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /trips/ [get]
func (h *Trip) AllShort(c *gin.Context) {
	output, err := h.s.AllShort(c.Request.Context())
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, output)
}

// @Summary Получить рейс по ID
// @Description Возвращает выбранный рейс с отдельными id рейса, автобуса и водителя. Остановки в stops отсортированы по stop_order; end_time может быть null.
// @Tags trips
// @Produce json
// @Param id path int true "Положительный ID записи" minimum(1)
// @Success 200 {object} model.TripResponse "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /trips/{id} [get]
func (h *Trip) ByID(c *gin.Context) {
	id, ok := domainID(c)
	if !ok {
		return
	}
	output, err := h.s.ById(c.Request.Context(), id)
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, output)
}

// @Summary Создать рейс
// @Description Создает рейс. Требуется роль admin. ID и временные метки назначаются сервером; status по умолчанию scheduled. Цена неотрицательная, end_time позже start_time.
// @Tags admin/trips
// @Security Bearer
// @Accept json
// @Produce json
// @Param input body model.Trip true "Данные запроса"
// @Success 201 {object} model.Trip "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 403 {object} apperrors.ErrorResponse "Требуются права администратора"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/trips/ [post]
func (h *Trip) Create(c *gin.Context) {
	var input model.Trip
	if err := c.ShouldBindJSON(&input); err != nil {
		writeDomainError(c, &model.ValidationError{Message: "Некорректные данные запроса"})
		return
	}
	err := h.s.Create(c.Request.Context(), &input)
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusCreated, input)
}

// @Summary Частично обновить рейс
// @Description Обновляет только переданные bus_id, driver_id, start_time, end_time, status, base_price. Разрешено обновление одного start_time. ID берется из URL; created_at и updated_at из JSON игнорируются. Требуется роль admin.
// @Tags admin/trips
// @Security Bearer
// @Accept json
// @Produce json
// @Param id path int true "Положительный ID записи" minimum(1)
// @Param input body model.TripUpdate true "Данные запроса"
// @Success 200 {object} apperrors.SuccessResponse "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 403 {object} apperrors.ErrorResponse "Требуются права администратора"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/trips/{id} [put]
func (h *Trip) Update(c *gin.Context) {
	id, ok := domainID(c)
	if !ok {
		return
	}
	var input model.TripUpdate
	if err := c.ShouldBindJSON(&input); err != nil {
		writeDomainError(c, &model.ValidationError{Message: "Некорректные данные запроса"})
		return
	}
	input.ID = id
	err := h.s.Update(c.Request.Context(), &input)
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, apperrors.SuccessResponse{Message: "updated"})
}

// @Summary Удалить рейс
// @Description Удаляет рейс по ID. Требуется роль admin.
// @Tags admin/trips
// @Security Bearer
// @Param id path int true "Положительный ID записи" minimum(1)
// @Success 204 "Успешный ответ"
// @Failure 400 {object} apperrors.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} apperrors.ErrorResponse "Требуется аутентификация"
// @Failure 403 {object} apperrors.ErrorResponse "Требуются права администратора"
// @Failure 404 {object} apperrors.ErrorResponse "Запись не найдена"
// @Failure 409 {object} apperrors.ErrorResponse "Конфликт данных или связанных записей"
// @Failure 500 {object} apperrors.ErrorResponse "Внутренняя ошибка сервера"
// @Router /admin/trips/{id} [delete]
func (h *Trip) Delete(c *gin.Context) {
	id, ok := domainID(c)
	if !ok {
		return
	}
	err := h.s.Delete(c.Request.Context(), id)
	if err != nil {
		h.logger.Error(err)
		writeDomainError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
