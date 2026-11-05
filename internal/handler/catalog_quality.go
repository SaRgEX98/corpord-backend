package handler

import (
	"context"
	"corpord-api/internal/catalogquality"
	"corpord-api/internal/handler/middleware"
	"corpord-api/model"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
)

type CatalogQualityHandler struct{ repo *catalogquality.Repository }

func NewCatalogQuality(db *sqlx.DB) *CatalogQualityHandler {
	return &CatalogQualityHandler{catalogquality.New(db)}
}
func (h *CatalogQualityHandler) Register(admin *gin.RouterGroup) {
	g := admin.Group("/catalog-quality")
	g.Use(func(c *gin.Context) {
		actor, ok := c.Get(middleware.UserIDCtx)
		id, valid := actor.(int)
		if !ok || !valid || id < 1 {
			c.AbortWithStatusJSON(401, gin.H{"error": "Требуется вход", "code": "unauthorized"})
			return
		}
		raw, _ := c.Get(middleware.ClaimsCtx)
		claims, ok := raw.(*model.Claims)
		if !ok || claims == nil || claims.Role != model.RoleAdmin {
			c.AbortWithStatusJSON(403, gin.H{"error": "Требуются права администратора", "code": "forbidden"})
			return
		}
		c.Next()
	})
	g.GET("/summary", h.Summary)
	g.GET("/stops", h.Queue)
	g.POST("/stops/:id/mapping", h.Mapping)
	g.GET("/stops/:id/history", h.History)
}
func qualityError(c *gin.Context, err error) {
	var e *catalogquality.Error
	var pg *pgconn.PgError
	if errors.As(err, &e) {
		c.JSON(e.Status, gin.H{"error": e.Message, "code": e.Code})
		return
	}
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &pg) && pg.Code == "57014") {
		c.JSON(503, gin.H{"error": "Операция заняла слишком много времени. Повторите запрос", "code": "quality_timeout"})
		return
	}
	if pg != nil && (pg.Code == "40P01" || pg.Code == "40001" || pg.Code == "55P03") {
		c.JSON(503, gin.H{"error": "Справочник обновляется. Обновите очередь и повторите операцию", "code": "quality_busy"})
		return
	}
	if pg != nil && pg.Code == "P0001" && pg.Message == "stop_in_use" {
		c.JSON(409, gin.H{"error": "Остановка используется в расписании", "code": "stop_in_use"})
		return
	}
	c.JSON(500, gin.H{"error": "Не удалось выполнить операцию", "code": "internal_error"})
}
func (h *CatalogQualityHandler) Summary(c *gin.Context) {
	out, err := h.repo.Summary(c.Request.Context())
	if err != nil {
		qualityError(c, err)
		return
	}
	c.JSON(200, out)
}
func (h *CatalogQualityHandler) Queue(c *gin.Context) {
	limit, ok := queryInt(c, "limit", 30)
	if !ok {
		return
	}
	out, err := h.repo.Queue(c.Request.Context(), c.Query("q"), c.Query("cursor"), limit)
	if err != nil {
		qualityError(c, err)
		return
	}
	c.JSON(200, out)
}
func (h *CatalogQualityHandler) Mapping(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		c.JSON(400, gin.H{"error": "Некорректный ID", "code": "invalid_request"})
		return
	}
	var body struct {
		CityID  int    `json:"city_id"`
		Version string `json:"version"`
		Reason  string `json:"reason"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
	if err = c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": "Некорректная форма", "code": "invalid_request"})
		return
	}
	out, err := h.repo.Map(c.Request.Context(), c.GetInt(middleware.UserIDCtx), id, body.CityID, body.Version, body.Reason)
	if err != nil {
		qualityError(c, err)
		return
	}
	c.JSON(200, out)
}
func (h *CatalogQualityHandler) History(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		id = 0
	}
	limit, ok := queryInt(c, "limit", 20)
	if !ok {
		return
	}
	out, err := h.repo.History(c.Request.Context(), id, limit)
	if err != nil {
		qualityError(c, err)
		return
	}
	c.JSON(200, gin.H{"items": out})
}
