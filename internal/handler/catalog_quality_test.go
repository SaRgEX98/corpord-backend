package handler

import (
	"corpord-api/internal/handler/middleware"
	"corpord-api/model"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestCatalogQualityDefenseInDepthACL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, role    string
		actor, status int
	}{{"anonymous", "", 0, 401}, {"passenger", model.RoleUser, 12, 403}, {"missingClaims", "", 12, 403}, {"adminValidation", model.RoleAdmin, 12, 400}} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			g := r.Group("/admin")
			g.Use(func(c *gin.Context) {
				if tc.actor > 0 {
					c.Set(middleware.UserIDCtx, tc.actor)
				}
				if tc.role != "" {
					c.Set(middleware.ClaimsCtx, &model.Claims{UserID: tc.actor, Role: tc.role})
				}
			})
			NewCatalogQuality(nil).Register(g)
			req := httptest.NewRequest("POST", "/admin/catalog-quality/stops/not-an-id/mapping", strings.NewReader(`{"city_id":1,"actor_id":999}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("want %d got %d %s", tc.status, w.Code, w.Body.String())
			}
		})
	}
}

func TestCatalogQualityConcurrentWriterErrorsAreRetryable(t *testing.T) {
	for _, sqlstate := range []string{"40P01", "40001", "55P03"} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		qualityError(c, &pgconn.PgError{Code: sqlstate, Message: "internal database details"})
		if w.Code != 503 || !strings.Contains(w.Body.String(), "quality_busy") || strings.Contains(w.Body.String(), "internal database") {
			t.Fatalf("unsafe concurrency error %s: %d %s", sqlstate, w.Code, w.Body.String())
		}
	}
}
