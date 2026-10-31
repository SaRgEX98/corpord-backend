package handler

import (
	"context"
	"corpord-api/internal/config"
	"corpord-api/internal/geo"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

type fakeGeo struct {
	err   error
	calls int
}

func (f *fakeGeo) Suggest(_ context.Context, _ geo.SearchRequest) ([]geo.Place, error) {
	f.calls++
	return []geo.Place{{ID: "osm:N:1", Name: "Москва"}}, f.err
}
func TestGeoHandlerDisabledRateLimitsAndErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name     string
		provider bool
		err      error
		status   int
		code     string
	}{
		{"disabled", false, nil, 503, "geo_unavailable"}, {"timeout", true, geo.ErrUnavailable, 503, "geo_unavailable"}, {"rate", true, geo.ErrRateLimited, 429, "geo_rate_limited"}, {"invalid", true, geo.ErrInvalidRequest, 400, "invalid_request"}, {"unexpected", true, errors.New("private error"), 503, "geo_unavailable"}, {"success", true, nil, 200, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := NewRoutes(nil, config.Geo{})
			if tc.provider {
				h.geo = &fakeGeo{err: tc.err}
			}
			r := gin.New()
			r.GET("/geo", h.GeoSuggest)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/geo?q=Москва", nil))
			if w.Code != tc.status {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
			var body map[string]any
			if json.Unmarshal(w.Body.Bytes(), &body) != nil {
				t.Fatal("bad json")
			}
			if tc.code != "" && body["code"] != tc.code {
				t.Fatalf("bad code %s", w.Body.String())
			}
		})
	}
	h := NewRoutes(nil, config.Geo{})
	f := &fakeGeo{}
	h.geo = f
	h.geoTokens = 0
	r := gin.New()
	r.GET("/geo", h.GeoSuggest)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/geo?q=Москва", nil))
	if w.Code != 429 || w.Header().Get("Retry-After") == "" || f.calls != 0 {
		t.Fatalf("unbounded upstream %d calls %d", w.Code, f.calls)
	}
}
func TestRouteQueryRejectsMalformedNumbersBeforeDatabase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewRoutes(nil, config.Geo{})
	r := gin.New()
	r.GET("/search", h.Search)
	for _, query := range []string{"passengers=garbage", "passengers=0", "passengers=2", "limit=-1", "limit=51", "limit=garbage"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/search?"+query, nil))
		if w.Code != 400 {
			t.Fatalf("%s status %d", query, w.Code)
		}
	}
}
