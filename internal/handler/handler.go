package handler

import (
	_ "corpord-api/docs"
	"corpord-api/internal/config"
	"corpord-api/internal/handler/middleware"
	"corpord-api/internal/logger"
	"corpord-api/internal/service"
	"corpord-api/internal/token"
	"corpord-api/model"
	"github.com/gin-gonic/gin"
	"github.com/swaggo/files"
	"github.com/swaggo/gin-swagger"
)

type handler struct {
	user   *UserHandler
	auth   *AuthHandler
	bus    *BusHandler
	bc     *BusCategoryHandler
	bs     *BusStatusHandler
	ds     *DriverStatus
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
		bus:    NewBus(logger, s.Bus),
		bc:     NewBusCategory(logger, s.BC),
		bs:     NewBusStatus(logger, s.BS),
		ds:     NewDriverStatus(logger, s.DS),
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
	h.r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
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
		bus := v1.Group("/bus")
		{
			bus.GET("/", h.bus.GetAllBuses)
			bus.GET("/:id", h.bus.GetBus)

			busCategories := bus.Group("/categories")
			{
				busCategories.GET("/", h.bc.GetAll)
				busCategories.GET("/:id", h.bc.GetById)
			}
			busStatus := bus.Group("/statuses")
			{
				busStatus.GET("/", h.bs.All)
				busStatus.GET("/:id", h.bs.ByID)
			}
		}
		driver := v1.Group("/driver")
		{
			driverStatus := driver.Group("status")
			{
				driverStatus.GET("/", h.ds.All)
				driverStatus.GET("/:id", h.ds.ById)
			}
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
				adminBus := admin.Group("/adminBus")
				{
					adminBus.POST("/", h.bus.CreateBus)
					adminBus.PUT("/:id", h.bus.UpdateBus)
					adminBus.DELETE("/:id", h.bus.DeleteBus)
					categories := adminBus.Group("/categories")
					{
						categories.POST("/", h.bc.Create)
						categories.DELETE("/:id", h.bc.Delete)
						categories.PUT("/:id", h.bc.Update)
					}
					status := adminBus.Group("/statuses")
					{
						status.POST("/", h.bs.Create)
						status.PUT("/:id", h.bs.Update)
						status.DELETE("/:id", h.bs.Delete)
					}
				}
				adminDriver := admin.Group("/driver")
				{
					status := adminDriver.Group("/status")
					{
						status.POST("/", h.ds.Create)
						status.PUT("/:id", h.ds.Update)
						status.DELETE("/:id", h.ds.Delete)
					}
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

	//h.bus.InitRoutes(v1)

	return h.r
}
