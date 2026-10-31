package handler

import (
	"context"
	"corpord-api/internal/config"
	"corpord-api/internal/geo"
	"corpord-api/internal/repository/pg"
	"corpord-api/internal/routes"
	"database/sql"
	"errors"
	"github.com/gin-gonic/gin"
	"strconv"
	"sync"
	"time"
)

type geoSuggester interface {
	Suggest(context.Context, geo.SearchRequest) ([]geo.Place, error)
}
type RouteHandler struct {
	service   *routes.Service
	geo       geoSuggester
	geoMu     sync.Mutex
	geoTokens float64
	geoLast   time.Time
	geoSlots  chan struct{}
}

func NewRoutes(s *routes.Service, cfg config.Geo) *RouteHandler {
	h := &RouteHandler{service: s, geoTokens: 10, geoLast: time.Now(), geoSlots: make(chan struct{}, 4)}
	if cfg.PhotonBaseURL != "" {
		h.geo, _ = geo.NewClient(cfg.PhotonBaseURL, cfg.Timeout)
	}
	return h
}
func routeError(c *gin.Context, err error) {
	var e *routes.Error
	if errors.As(err, &e) {
		c.JSON(e.Status, gin.H{"error": e.Message, "code": e.Code})
		return
	}
	status, code, message := 500, "internal_error", "Внутренняя ошибка сервера"
	switch {
	case errors.Is(err, sql.ErrNoRows):
		status, code, message = 404, "not_found", "Запись не найдена"
	case pg.IsPgError(err, "23505"):
		status, code, message = 409, "already_exists", "Запись уже существует"
	case pg.IsPgError(err, "23503"):
		status, code, message = 409, "invalid_reference", "Связанная запись не найдена"
	case pg.IsPgError(err, "23514"), pg.IsPgError(err, "23502"), pg.IsPgError(err, "22003"):
		status, code, message = 400, "invalid_request", "Некорректные данные"
	}
	c.JSON(status, gin.H{"error": message, "code": code})
}
func queryInt(c *gin.Context, key string, def int) (int, bool) {
	v, ok := c.GetQuery(key)
	if !ok {
		return def, true
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		routeError(c, &routes.Error{Status: 400, Code: "invalid_request", Message: "Некорректный " + key})
		return 0, false
	}
	return n, true
}
func (h *RouteHandler) Suggest(c *gin.Context) {
	limit, ok := queryInt(c, "limit", 8)
	if !ok {
		return
	}
	items, err := h.service.Repo.Suggest(c.Request.Context(), c.Query("q"), c.Query("origin_id"), limit)
	if err != nil {
		routeError(c, err)
		return
	}
	c.JSON(200, gin.H{"items": items})
}
func (h *RouteHandler) Stops(c *gin.Context) {
	items, err := h.service.Repo.Stops(c.Request.Context(), c.Param("id"))
	if err != nil {
		routeError(c, err)
		return
	}
	c.JSON(200, gin.H{"items": items})
}
func searchQuery(c *gin.Context) (routes.Search, bool) {
	passengers, ok := queryInt(c, "passengers", 1)
	if !ok {
		return routes.Search{}, false
	}
	limit, ok := queryInt(c, "limit", 20)
	if !ok {
		return routes.Search{}, false
	}
	if passengers != 1 || limit < 1 || limit > 50 {
		routeError(c, &routes.Error{Status: 400, Code: "invalid_request", Message: "Некорректное число пассажиров или limit"})
		return routes.Search{}, false
	}
	return routes.Search{From: c.Query("from_id"), To: c.Query("to_id"), Date: c.Query("date"), Month: c.Query("month"), Sort: c.Query("sort"), Cursor: c.Query("cursor"), Passengers: passengers, Limit: limit}, true
}
func (h *RouteHandler) Search(c *gin.Context) {
	q, ok := searchQuery(c)
	if !ok {
		return
	}
	out, err := h.service.Search(c.Request.Context(), q)
	if err != nil {
		routeError(c, err)
		return
	}
	c.JSON(200, out)
}
func (h *RouteHandler) Calendar(c *gin.Context) {
	q, ok := searchQuery(c)
	if !ok {
		return
	}
	out, err := h.service.Calendar(c.Request.Context(), q)
	if err != nil {
		routeError(c, err)
		return
	}
	c.JSON(200, out)
}
func (h *RouteHandler) Quote(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		routeError(c, &routes.Error{Status: 400, Code: "invalid_request", Message: "Некорректный ID рейса"})
		return
	}
	var q routes.QuoteRequest
	if c.ShouldBindJSON(&q) != nil {
		routeError(c, &routes.Error{Status: 400, Code: "invalid_request", Message: "Некорректный JSON"})
		return
	}
	out, err := h.service.Quote(c.Request.Context(), id, q)
	if err != nil {
		routeError(c, err)
		return
	}
	c.JSON(200, out)
}
func (h *RouteHandler) Cities(c *gin.Context) {
	items, err := h.service.Repo.Cities(c.Request.Context())
	if err != nil {
		routeError(c, err)
		return
	}
	c.JSON(200, gin.H{"items": items})
}
func (h *RouteHandler) SaveCity(c *gin.Context) {
	var city routes.City
	if c.ShouldBindJSON(&city) != nil {
		routeError(c, &routes.Error{Status: 400, Code: "invalid_request", Message: "Некорректный JSON"})
		return
	}
	city.ID = 0
	if c.Request.Method == "PUT" {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil || id < 1 {
			routeError(c, &routes.Error{Status: 400, Code: "invalid_request", Message: "Некорректный ID"})
			return
		}
		city.ID = id
	}
	if err := h.service.Repo.SaveCity(c.Request.Context(), &city); err != nil {
		routeError(c, err)
		return
	}
	status := 200
	if c.Request.Method == "POST" {
		status = 201
	}
	c.JSON(status, city)
}

func (h *RouteHandler) geoAllowed() bool {
	h.geoMu.Lock()
	defer h.geoMu.Unlock()
	now := time.Now()
	h.geoTokens += now.Sub(h.geoLast).Seconds() * 2
	if h.geoTokens > 10 {
		h.geoTokens = 10
	}
	h.geoLast = now
	if h.geoTokens < 1 {
		return false
	}
	h.geoTokens--
	return true
}
func (h *RouteHandler) GeoSuggest(c *gin.Context) {
	if h.geo == nil {
		routeError(c, &routes.Error{Status: 503, Code: "geo_unavailable", Message: "Поиск адресов временно недоступен"})
		return
	}
	limit, ok := queryInt(c, "limit", 8)
	if !ok {
		return
	}
	req := geo.SearchRequest{Query: c.Query("q"), Limit: limit, Language: "ru"}
	if lat, exists := c.GetQuery("lat"); exists {
		v, err := strconv.ParseFloat(lat, 64)
		if err != nil {
			routeError(c, &routes.Error{Status: 400, Code: "invalid_request", Message: "Некорректные координаты"})
			return
		}
		req.Lat = &v
	}
	if lon, exists := c.GetQuery("lon"); exists {
		v, err := strconv.ParseFloat(lon, 64)
		if err != nil {
			routeError(c, &routes.Error{Status: 400, Code: "invalid_request", Message: "Некорректные координаты"})
			return
		}
		req.Lon = &v
	}
	if !h.geoAllowed() {
		c.Header("Retry-After", "1")
		routeError(c, &routes.Error{Status: 429, Code: "geo_rate_limited", Message: "Повторите поиск позже"})
		return
	}
	select {
	case h.geoSlots <- struct{}{}:
		defer func() { <-h.geoSlots }()
	default:
		c.Header("Retry-After", "1")
		routeError(c, &routes.Error{Status: 429, Code: "geo_rate_limited", Message: "Повторите поиск позже"})
		return
	}
	items, err := h.geo.Suggest(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, geo.ErrInvalidRequest):
			routeError(c, &routes.Error{Status: 400, Code: "invalid_request", Message: "Некорректный запрос адреса"})
		case errors.Is(err, geo.ErrRateLimited):
			c.Header("Retry-After", "1")
			routeError(c, &routes.Error{Status: 429, Code: "geo_rate_limited", Message: "Повторите поиск позже"})
		default:
			routeError(c, &routes.Error{Status: 503, Code: "geo_unavailable", Message: "Поиск адресов временно недоступен"})
		}
		return
	}
	c.JSON(200, gin.H{"items": items})
}

func (h *RouteHandler) Location(c *gin.Context) {
	out, err := h.service.Repo.Location(c.Request.Context(), c.Param("id"))
	if err != nil {
		routeError(c, err)
		return
	}
	c.JSON(200, out)
}
