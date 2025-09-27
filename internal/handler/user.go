package handler

import (
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/model"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"time"
)

type UserHandler struct {
	logger *logger.Logger
	s      service.User
}

func NewUser(logger *logger.Logger, s service.User) *UserHandler {
	return &UserHandler{
		logger: logger,
		s:      s,
	}
}

// All возвращает список всех пользователей
func (h *UserHandler) All(c *gin.Context) {
	start := time.Now()
	h.logger.Info("handling get all users request")

	users, err := h.s.GetAll(c.Request.Context())
	if err != nil {
		h.logger.Errorf("failed to get users: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	h.logger.Infof("retrieved %d users in %v", len(users), time.Since(start))
	c.JSON(http.StatusOK, users)
}

// Get возвращает пользователя по ID
func (h *UserHandler) Get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	user, err := h.s.GetByID(c.Request.Context(), id)
	if err != nil {
		h.logger.Errorf("failed to get user %d: %v", id, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// Create создает нового пользователя
func (h *UserHandler) Create(c *gin.Context) {
	var user model.UserCreate
	if err := c.ShouldBindJSON(&user); err != nil {
		h.logger.Warnf("invalid request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	createdUser, err := h.s.Create(c.Request.Context(), &user)
	if err != nil {
		h.logger.Errorf("failed to create user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, createdUser)
}

// Update обновляет данные пользователя
func (h *UserHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	var update model.UserUpdate
	if err := c.ShouldBindJSON(&update); err != nil {
		h.logger.Warnf("invalid request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	updatedUser, err := h.s.Update(c.Request.Context(), id, &update)
	if err != nil {
		h.logger.Errorf("failed to update user %d: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update user"})
		return
	}

	c.JSON(http.StatusOK, updatedUser)
}

// Delete удаляет пользователя
func (h *UserHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	if err := h.s.Delete(c.Request.Context(), id); err != nil {
		h.logger.Errorf("failed to delete user %d: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete user"})
		return
	}

	c.Status(http.StatusNoContent)
}

type AuthorizeRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

// Authorize обрабатывает аутентификацию пользователя
func (h *UserHandler) Authorize(c *gin.Context) {
	start := time.Now()
	h.logger.Info("handling user authorization request")

	var req model.UserLogin
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("invalid request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	token, err := h.s.Login(c.Request.Context(), req)
	if err != nil {
		h.logger.Warnf("authorization failed for %s: %v", req.Email, err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	h.logger.Infof("user %s authorized successfully in %v", req.Email, time.Since(start))

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"email": req.Email,
	})
}
