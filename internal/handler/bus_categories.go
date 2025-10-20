package handler

import (
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/model"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// BusCategoryHandler обрабатывает HTTP-запросы для работы с категориями автобусов
type BusCategoryHandler struct {
	logger *logger.Logger
	bc     service.BusCategory
}

func NewBusCategory(logger *logger.Logger, bc service.BusCategory) *BusCategoryHandler {
	return &BusCategoryHandler{
		logger: logger,
		bc:     bc,
	}
}

// GetAll возвращает список всех категорий автобусов
// @Summary Получить все категории автобусов
// @Description Возвращает список всех доступных категорий автобусов. Доступно всем пользователям
// @Tags bus_categories
// @Produce json
// @Success 200 {array} model.BusCategory "Список категорий автобусов"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /api/v1/bus/categories [get]
func (h *BusCategoryHandler) GetAll(c *gin.Context) {
	output, err := h.bc.GetAll(c.Request.Context())
	if err != nil {
		h.logger.Errorf("error while getting categories: %v", err)
		c.JSON(ErrInternal.Status, ErrorResponse{Error: "Не удалось получить список категорий"})
		return
	}
	c.JSON(http.StatusOK, output)
}

// GetById возвращает категорию автобуса по ID
// @Summary Получить категорию по ID
// @Description Возвращает информацию о категории автобуса по её идентификатору. Доступно всем пользователям
// @Tags bus_categories
// @Produce json
// @Param id path int true "ID категории"
// @Success 200 {object} model.BusCategory "Данные категории"
// @Failure 400 {object} ErrorResponse "Некорректный ID"
// @Failure 404 {object} ErrorResponse "Категория не найдена"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /api/v1/bus/categories/{id} [get]
func (h *BusCategoryHandler) GetById(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(ErrBadRequest.Status, ErrorResponse{Error: "Некорректный ID категории"})
		return
	}

	output, err := h.bc.GetById(c.Request.Context(), id)
	if err != nil {
		h.logger.Errorf("failed to get category: %v", err)
		if errors.Is(err, service.ErrBusCategoryNotFound) {
			c.JSON(ErrNotFound.Status, ErrorResponse{Error: "Категория не найдена"})
			return
		}
		c.JSON(ErrInternal.Status, ErrorResponse{Error: "Не удалось получить данные категории"})
		return
	}

	c.JSON(http.StatusOK, output)
}

// Create создает новую категорию автобуса
// @Summary Создать новую категорию автобуса
// @Description Создает новую категорию автобуса (только для администраторов)
// @Tags bus_categories
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param Authorization header string true "Bearer <token>"
// @Param input body model.BusCategory true "Данные категории"
// @Success 201 {object} SuccessResponse "Категория успешно создана"
// @Failure 400 {object} ErrorResponse "Некорректные данные"
// @Failure 401 {object} ErrorResponse "Не авторизован"
// @Failure 403 {object} ErrorResponse "Доступ запрещен. Требуются права администратора"
// @Failure 409 {object} ErrorResponse "Категория с таким названием уже существует"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /api/v1/admin/bus/categories [post]
func (h *BusCategoryHandler) Create(c *gin.Context) {
	var input model.BusCategory
	if err := c.ShouldBindJSON(&input); err != nil {
		h.logger.Warnf("invalid request body: %v", err)
		c.JSON(ErrBadRequest.Status, ErrorResponse{Error: "Некорректные данные категории"})
		return
	}

	if err := h.bc.Create(c.Request.Context(), input); err != nil {
		h.logger.Errorf("failed to create category: %v", err)
		if errors.Is(err, service.ErrBusCategoryExists) {
			c.JSON(http.StatusConflict, ErrorResponse{Error: "Категория с таким названием уже существует"})
			return
		}
		c.JSON(ErrInternal.Status, ErrorResponse{Error: "Не удалось создать категорию"})
		return
	}

	c.JSON(http.StatusCreated, SuccessResponse{Message: "Категория успешно создана"})
}

// Delete удаляет категорию автобуса
// @Summary Удалить категорию автобуса
// @Description Удаляет категорию автобуса по ID (только для администраторов)
// @Tags bus_categories
// @Produce json
// @Security ApiKeyAuth
// @Param Authorization header string true "Bearer <token>"
// @Param id path int true "ID категории"
// @Success 204 "Категория успешно удалена"
// @Failure 400 {object} ErrorResponse "Некорректный ID"
// @Failure 401 {object} ErrorResponse "Не авторизован"
// @Failure 403 {object} ErrorResponse "Доступ запрещен. Требуются права администратора"
// @Failure 404 {object} ErrorResponse "Категория не найдена"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /api/v1/admin/bus/categories/{id} [delete]
func (h *BusCategoryHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(ErrBadRequest.Status, ErrorResponse{Error: "Некорректный ID категории"})
		return
	}

	if err := h.bc.Delete(c.Request.Context(), id); err != nil {
		h.logger.Errorf("failed to delete category %d: %v", id, err)
		if errors.Is(err, service.ErrBusCategoryNotFound) {
			c.JSON(ErrNotFound.Status, ErrorResponse{Error: "Категория не найдена"})
			return
		}
		c.JSON(ErrInternal.Status, ErrorResponse{Error: "Не удалось удалить категорию"})
		return
	}

	c.Status(http.StatusNoContent)
}

// Update обновляет данные категории автобуса
// @Summary Обновить категорию автобуса
// @Description Обновляет информацию о категории автобуса по ID (только для администраторов)
// @Tags bus_categories
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param Authorization header string true "Bearer <token>"
// @Param id path int true "ID категории"
// @Param input body model.BusCategory true "Обновленные данные категории"
// @Success 200 {object} model.BusCategory "Обновленные данные категории"
// @Failure 400 {object} ErrorResponse "Некорректные данные"
// @Failure 401 {object} ErrorResponse "Не авторизован"
// @Failure 403 {object} ErrorResponse "Доступ запрещен. Требуются права администратора"
// @Failure 404 {object} ErrorResponse "Категория не найдена"
// @Failure 409 {object} ErrorResponse "Категория с таким названием уже существует"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /api/v1/admin/bus/categories/{id} [put]
func (h *BusCategoryHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(ErrBadRequest.Status, ErrorResponse{Error: "Некорректный ID категории"})
		return
	}

	var category model.BusCategory
	if err := c.ShouldBindJSON(&category); err != nil {
		h.logger.Warnf("invalid request body: %v", err)
		c.JSON(ErrBadRequest.Status, ErrorResponse{Error: "Некорректные данные для обновления"})
		return
	}

	category.ID = id

	updatedCategory, err := h.bc.Update(c.Request.Context(), category)
	if err != nil {
		h.logger.Errorf("failed to update category %d: %v", id, err)
		switch {
		case errors.Is(err, service.ErrBusCategoryNotFound):
			c.JSON(ErrNotFound.Status, ErrorResponse{Error: "Категория не найдена"})
		case errors.Is(err, service.ErrBusCategoryExists):
			c.JSON(http.StatusConflict, ErrorResponse{Error: "Категория с таким названием уже существует"})
		default:
			c.JSON(ErrInternal.Status, ErrorResponse{Error: "Не удалось обновить категорию"})
		}
		return
	}

	c.JSON(http.StatusOK, updatedCategory)
}
