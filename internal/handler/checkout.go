package handler

import (
	"context"
	"corpord-api/internal/checkout"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
)

type CheckoutHandler struct{ service *checkout.Service }

func NewCheckout(service *checkout.Service) *CheckoutHandler {
	return &CheckoutHandler{service: service}
}
func (h *CheckoutHandler) Register(_ *gin.RouterGroup, authorized *gin.RouterGroup) {
	private := authorized.Group("/me/checkouts")
	private.Use(func(c *gin.Context) { c.Header("Cache-Control", "no-store"); c.Next() })
	private.POST("", h.Create)
	private.GET("", h.List)
	private.GET("/:id", h.Get)
	private.POST("/:id/cancel", h.Cancel)
	private.POST("/:id/reconcile", h.Reconcile)
}
func checkoutError(c *gin.Context, err error) {
	var e *checkout.Error
	if errors.As(err, &e) {
		if e.Status == 429 {
			c.Header("Retry-After", "5")
		}
		c.JSON(e.Status, gin.H{"error": e.Message, "code": e.Code})
		return
	}
	var pg *pgconn.PgError
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &pg) && (pg.Code == "57014" || pg.Code == "55P03" || pg.Code == "40P01" || pg.Code == "40001")) {
		c.JSON(503, gin.H{"error": "Операция заняла слишком много времени. Повторите запрос", "code": "provider_unavailable"})
		return
	}
	c.JSON(500, gin.H{"error": "Не удалось выполнить запрос", "code": "internal_error"})
}
func (h *CheckoutHandler) Create(c *gin.Context) {
	owner, ok := bookingOwner(c)
	if !ok {
		return
	}
	var body struct {
		HoldID int64 `json:"hold_id"`
	}
	if !bookingBody(c, &body) {
		return
	}
	out, fresh, err := h.service.Create(c.Request.Context(), owner, c.GetHeader("Idempotency-Key"), body.HoldID)
	if err != nil {
		checkoutError(c, err)
		return
	}
	status := 200
	if fresh {
		status = 201
	}
	c.JSON(status, out)
}
func (h *CheckoutHandler) List(c *gin.Context) {
	owner, ok := bookingOwner(c)
	if !ok {
		return
	}
	limit, ok := bookingLimit(c)
	if !ok {
		return
	}
	out, err := h.service.List(c.Request.Context(), owner, limit, c.Query("status"), c.Query("cursor"))
	if err != nil {
		checkoutError(c, err)
		return
	}
	c.JSON(200, out)
}
func (h *CheckoutHandler) Get(c *gin.Context) {
	owner, ok := bookingOwner(c)
	if !ok {
		return
	}
	id, ok := bookingID(c)
	if !ok {
		return
	}
	out, err := h.service.Get(c.Request.Context(), owner, id)
	if err != nil {
		checkoutError(c, err)
		return
	}
	c.JSON(200, out)
}
func (h *CheckoutHandler) Cancel(c *gin.Context) {
	owner, ok := bookingOwner(c)
	if !ok {
		return
	}
	id, ok := bookingID(c)
	if !ok {
		return
	}
	out, err := h.service.Cancel(c.Request.Context(), owner, id)
	if err != nil {
		checkoutError(c, err)
		return
	}
	c.JSON(200, out)
}
func (h *CheckoutHandler) Reconcile(c *gin.Context) {
	owner, ok := bookingOwner(c)
	if !ok {
		return
	}
	id, ok := bookingID(c)
	if !ok {
		return
	}
	if err := h.service.Reconcile(c.Request.Context(), owner, id); err != nil {
		checkoutError(c, err)
		return
	}
	c.JSON(202, gin.H{"queued": true})
}
