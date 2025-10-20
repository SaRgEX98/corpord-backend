package handler

import (
	"corpord-api/internal/logger"
	"net/http"
	"time"

	"corpord-api/internal/service"
	"corpord-api/model"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	service service.Auth
	logger  *logger.Logger
}

func NewAuthHandler(s service.Auth, l *logger.Logger) *AuthHandler {
	return &AuthHandler{
		service: s,
		logger:  l,
	}
}

// Register handles user registration
func (h *AuthHandler) Register(c *gin.Context) {
	start := time.Now()
	h.logger.Info("handling user registration request")

	var req model.UserCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("invalid request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	user, err := h.service.Register(c.Request.Context(), &req)
	if err != nil {
		h.logger.Warnf("registration failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Infof("user registered successfully in %v", time.Since(start))
	c.JSON(http.StatusCreated, user)
}

// Login handles user authentication
func (h *AuthHandler) Login(c *gin.Context) {
	start := time.Now()
	h.logger.Info("handling user login request")

	var req model.UserLogin
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("invalid request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	token, err := h.service.Login(c.Request.Context(), req)
	if err != nil {
		h.logger.Warnf("login failed for %s: %v", req.Email, err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	h.logger.Infof("user %s logged in successfully in %v", req.Email, time.Since(start))
	c.JSON(http.StatusOK, gin.H{"token": token})
}
