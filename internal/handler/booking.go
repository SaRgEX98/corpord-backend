package handler

import (
	"context"
	"corpord-api/internal/booking"
	"corpord-api/internal/handler/middleware"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
)

type BookingHandler struct{ service *booking.Service }

func NewBooking(db *sqlx.DB) *BookingHandler { return &BookingHandler{service: booking.New(db)} }
func (h *BookingHandler) Register(public, authorized *gin.RouterGroup) {
	public.GET("/trips/:id/availability", h.Availability)
	authorized.POST("/me/bookings", h.Create)
	authorized.GET("/me/bookings", h.List)
	authorized.GET("/me/bookings/:id", h.Get)
	authorized.POST("/me/bookings/:id/cancel", h.Cancel)
	authorized.GET("/me/passengers", h.Passengers)
	authorized.POST("/me/passengers", h.CreatePassenger)
	authorized.PUT("/me/passengers/:id", h.UpdatePassenger)
	authorized.DELETE("/me/passengers/:id", h.DeletePassenger)
}
func (h *BookingHandler) Expire(ctx context.Context, limit int) error {
	return h.service.Expire(ctx, limit)
}
func bookingError(c *gin.Context, err error) {
	var e *booking.Error
	if errors.As(err, &e) {
		c.JSON(e.Status, gin.H{"error": e.Message, "code": e.Code})
		return
	}
	var pgerr *pgconn.PgError
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || (errors.As(err, &pgerr) && (pgerr.Code == "57014" || pgerr.Code == "55P03" || pgerr.Code == "40P01")) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Время ожидания истекло. Повторите запрос", "code": "booking_timeout"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось выполнить запрос", "code": "internal_error"})
}
func bookingInvalid(c *gin.Context) {
	bookingError(c, &booking.Error{Status: 400, Code: "invalid_request", Message: "Некорректные параметры запроса"})
}
func bookingOwner(c *gin.Context) (int, bool) {
	owner, ok := c.Get(middleware.UserIDCtx)
	id, isInt := owner.(int)
	if !ok || !isInt || id < 1 {
		c.JSON(401, gin.H{"error": "Требуется вход", "code": "unauthorized"})
		return 0, false
	}
	return id, true
}
func bookingID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		bookingInvalid(c)
		return 0, false
	}
	return id, true
}
func bookingBody(c *gin.Context, out any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		bookingInvalid(c)
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		bookingInvalid(c)
		return false
	}
	return true
}
func bookingLimit(c *gin.Context) (int, bool) {
	if !c.Request.URL.Query().Has("limit") {
		return 30, true
	}
	n, err := strconv.Atoi(c.Query("limit"))
	if err != nil || n < 1 || n > 100 {
		bookingInvalid(c)
		return 0, false
	}
	return n, true
}
func (h *BookingHandler) Availability(c *gin.Context) {
	id, ok := bookingID(c)
	if !ok {
		return
	}
	b, err := strconv.Atoi(c.Query("boarding_stop_id"))
	if err != nil {
		bookingInvalid(c)
		return
	}
	a, err := strconv.Atoi(c.Query("alighting_stop_id"))
	if err != nil {
		bookingInvalid(c)
		return
	}
	if id > int64(^uint(0)>>1) {
		bookingInvalid(c)
		return
	}
	out, err := h.service.Availability(c.Request.Context(), int(id), b, a)
	if err != nil {
		bookingError(c, err)
		return
	}
	c.JSON(200, out)
}
func (h *BookingHandler) Create(c *gin.Context) {
	owner, ok := bookingOwner(c)
	if !ok {
		return
	}
	var body booking.CreateRequest
	if !bookingBody(c, &body) {
		return
	}
	out, created, err := h.service.Create(c.Request.Context(), owner, c.GetHeader("Idempotency-Key"), body)
	if err != nil {
		bookingError(c, err)
		return
	}
	status := 200
	if created {
		status = 201
	}
	c.JSON(status, out)
}
func (h *BookingHandler) List(c *gin.Context) {
	owner, ok := bookingOwner(c)
	if !ok {
		return
	}
	limit, ok := bookingLimit(c)
	if !ok {
		return
	}
	out, err := h.service.List(c.Request.Context(), owner, limit, c.Query("cursor"))
	if err != nil {
		bookingError(c, err)
		return
	}
	c.JSON(200, out)
}
func (h *BookingHandler) Get(c *gin.Context) {
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
		bookingError(c, err)
		return
	}
	c.JSON(200, out)
}
func (h *BookingHandler) Cancel(c *gin.Context) {
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
		bookingError(c, err)
		return
	}
	c.JSON(200, out)
}
func (h *BookingHandler) Passengers(c *gin.Context) {
	owner, ok := bookingOwner(c)
	if !ok {
		return
	}
	limit, ok := bookingLimit(c)
	if !ok {
		return
	}
	out, err := h.service.Passengers(c.Request.Context(), owner, limit, c.Query("cursor"))
	if err != nil {
		bookingError(c, err)
		return
	}
	c.JSON(200, out)
}
func (h *BookingHandler) CreatePassenger(c *gin.Context) { h.savePassenger(c, 0) }
func (h *BookingHandler) UpdatePassenger(c *gin.Context) {
	id, ok := bookingID(c)
	if !ok {
		return
	}
	h.savePassenger(c, id)
}
func (h *BookingHandler) savePassenger(c *gin.Context, id int64) {
	owner, ok := bookingOwner(c)
	if !ok {
		return
	}
	var body struct {
		FullName string `json:"full_name"`
	}
	if !bookingBody(c, &body) {
		return
	}
	out, err := h.service.SavePassenger(c.Request.Context(), owner, id, body.FullName)
	if err != nil {
		bookingError(c, err)
		return
	}
	status := 200
	if id == 0 {
		status = 201
	}
	c.JSON(status, out)
}
func (h *BookingHandler) DeletePassenger(c *gin.Context) {
	owner, ok := bookingOwner(c)
	if !ok {
		return
	}
	id, ok := bookingID(c)
	if !ok {
		return
	}
	if err := h.service.DeletePassenger(c.Request.Context(), owner, id); err != nil {
		bookingError(c, err)
		return
	}
	c.Status(204)
}
