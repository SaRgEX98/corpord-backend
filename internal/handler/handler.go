package handler

import (
	"corpord-api/internal/handler/middleware"
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"github.com/gin-gonic/gin"
)

type handler struct {
	logger *logger.Logger
	s      *service.Service
	r      *gin.Engine
	uh     *UserHandler
}

func New(logger *logger.Logger, s *service.Service) Handler {
	return &handler{
		logger: logger,
		s:      s,
		r:      gin.Default(),
		uh:     NewUser(logger, s.User),
	}
}

// InitRoutes initializes all the routes for the application
func (h *handler) InitRoutes() *gin.Engine {
	h.logger.Info("Initializing routes")

	// Add middleware
	h.r.Use(middleware.RequestLogger(h.logger))

	// API v1 routes
	v1 := h.r.Group("api/v1")
	{
		// User routes (public)
		auth := v1.Group("/auth")
		{
			auth.POST("/sign-up", h.uh.Create)    // Создание пользователя
			auth.POST("/sign-in", h.uh.Authorize) // Аутентификация
		}

		// Protected routes (require authentication)
		authorized := v1.Group("/")
		{
			// User management
			userRoutes := authorized.Group("/users", middleware.CheckAuthorization(h.logger))
			{
				userRoutes.GET("/", h.uh.All)          // Получение списка пользователей
				userRoutes.GET("/:id", h.uh.Get)       // Получение пользователя по ID
				userRoutes.PUT("/:id", h.uh.Update)    // Обновление пользователя
				userRoutes.DELETE("/:id", h.uh.Delete) // Удаление пользователя
			}
		}
	}

	return h.r
}
