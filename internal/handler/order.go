package handler

import (
	"corpord-api/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type OrderHandler struct {
	os service.Order
}

func NewOrder(os service.Order) *OrderHandler {
	return &OrderHandler{
		os: os,
	}
}

func (oh *OrderHandler) All(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}
