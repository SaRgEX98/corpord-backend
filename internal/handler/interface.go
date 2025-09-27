package handler

import (
	"github.com/gin-gonic/gin"
)

// Handler defines the interface for HTTP handlers
type Handler interface {
	// InitRoutes initializes all the routes for the application
	InitRoutes() *gin.Engine
}
