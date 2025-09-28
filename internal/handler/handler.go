package handler

import (
	"corpord-api/internal/config"
	"corpord-api/internal/handler/middleware"
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/internal/token"
	"github.com/gin-gonic/gin"

	"corpord-api/model"
)

type handler struct {
	user   *UserHandler
	auth   *AuthHandler
	logger *logger.Logger
	s      *service.Service
	r      *gin.Engine
	cfg    *config.Config
	t      token.Manager
}

// New creates a new handler instance with all dependencies
func New(logger *logger.Logger, s *service.Service, cfg *config.Config, t token.Manager) Handler {
	return &handler{
		user:   NewUser(logger, s.User),
		auth:   NewAuthHandler(s.Auth, logger),
		logger: logger,
		s:      s,
		r:      gin.Default(),
		cfg:    cfg,
		t:      t,
	}
}

// InitRoutes initializes all the routes for the application
func (h *handler) InitRoutes() *gin.Engine {
	h.logger.Info("Initializing routes")

	// Add global middleware
	h.r.Use(middleware.RequestLogger(h.logger))

	// API v1 routes
	v1 := h.r.Group("api/v1")
	{
		// Public routes - no authentication required
		auth := v1.Group("/auth")
		{
			auth.POST("/register", h.auth.Register) // New user registration
			auth.POST("/login", h.auth.Login)       // User login
		}

		// Protected routes - require valid JWT token
		authorized := v1.Group("")
		authorized.Use(middleware.AuthMiddleware(h.logger, h.t))
		{
			// Example of admin-only route
			admin := authorized.Group("/admin")
			admin.Use(middleware.RoleMiddleware(model.RoleAdmin, h.logger))
			{
				// Add admin routes here
				// admin.GET("/users", h.user.GetAllUsers)
				users := admin.Group("/users")
				{
					users.PUT("/:id", h.user.Update) // Update user

				}

			}

			// User management
			users := authorized.Group("/users")
			{
				users.GET("", h.user.All)           // Get all users
				users.GET("/:id", h.user.Get)       // Get user by ID
				users.POST("", h.user.Create)       // Create user (kept for backward compatibility)
				users.DELETE("/:id", h.user.Delete) // Delete user
			}
		}
	}

	return h.r
}
