package geo

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const fixture = `{"type":"FeatureCollection","features":[{"geometry":{"type":"Point","coordinates":[60.6,56.8]},"properties":{"name":"Южный автовокзал","street":"улица 8 Марта","housenumber":"145","city":"Екатеринбург","state":"Свердловская область","country":"Россия","countrycode":"RU","osm_type":"N","osm_id":123,"osm_value":"bus_station"}}]}`

func TestSuggest(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/prefix/api" || r.URL.Query().Get("q") != "Южный & вокзал" || r.URL.Query().Get("lang") != "ru" || r.URL.Query().Get("countrycode") != "RU" || r.URL.Query().Get("lat") != "56.8" {
			t.Errorf("unexpected request: %s", r.URL)
		}
		fmt.Fprint(w, fixture)
	}))
	defer s.Close()
	c, _ := NewClient(s.URL+"/prefix/", time.Second)
	lat, lon := 56.8, 60.6
	got, err := c.Suggest(context.Background(), SearchRequest{Query: " Южный & вокзал ", Limit: 5, Lat: &lat, Lon: &lon})
	if err != nil || len(got) != 1 {
		t.Fatalf("%+v %v", got, err)
	}
	if got[0].ID != "osm:N:123" || got[0].Latitude != lat || got[0].Longitude != lon || !strings.Contains(got[0].Label, "улица 8 Марта 145") {
		t.Fatalf("%+v", got[0])
	}
}
func TestProviderFailures(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		want       error
	}{
		{"rate", "", 429, ErrRateLimited}, {"down", "private diagnostic", 503, ErrUnavailable}, {"redirect", "", 302, ErrUnavailable}, {"bad json", "oops", 200, ErrUnavailable}, {"missing features", "{}", 200, ErrUnavailable}, {"too big", strings.Repeat(" ", maxResponseBytes+1), 200, ErrUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); fmt.Fprint(w, tc.body) }))
			defer s.Close()
			c, _ := NewClient(s.URL, time.Second)
			_, err := c.Suggest(context.Background(), SearchRequest{Query: "Москва", Limit: 5})
			if !errors.Is(err, tc.want) {
				t.Fatalf("%v", err)
			}
		})
	}
}
func TestValidation(t *testing.T) {
	for _, u := range []string{"", "https://", "ftp://localhost", "http://user:pass@localhost", "http://localhost/?secret=x", "http://localhost/#x"} {
		if _, err := NewClient(u, time.Second); err == nil {
			t.Errorf("accepted %q", u)
		}
	}
	c, _ := NewClient("http://localhost", time.Second)
	bad := math.NaN()
	lat := 91.0
	for _, req := range []SearchRequest{{Query: "а", Limit: 1}, {Query: "Москва", Limit: 11}, {Query: "Москва", Limit: 0}, {Query: "Мос\nква", Limit: 1}, {Query: "Москва", Limit: 1, Lat: &lat}, {Query: "Москва", Limit: 1, Lat: &lat, Lon: &bad}, {Query: "Москва", Limit: 1, Language: "xx"}} {
		if _, err := c.Suggest(context.Background(), req); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("%+v: %v", req, err)
		}
	}
}
func TestCancellationAndTimeout(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer s.Close()
	c, _ := NewClient(s.URL, 20*time.Millisecond)
	if _, err := c.Suggest(context.Background(), SearchRequest{Query: "Москва", Limit: 1}); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Suggest(ctx, SearchRequest{Query: "Москва", Limit: 1}); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}
func TestInvalidFeaturesAreSkipped(t *testing.T) {
	body := strings.TrimSuffix(fixture, "]}") + "," + strings.TrimPrefix(strings.TrimSuffix(fixture, "]}"), `{"type":"FeatureCollection","features":[`) + "]}"
	// Duplicate IDs must not make repeated choices. Foreign/non-point results are discarded.
	for _, tc := range []struct {
		body  string
		count int
	}{{body, 1}, {strings.ReplaceAll(fixture, `"RU"`, `"DE"`), 0}, {strings.ReplaceAll(fixture, `"Point"`, `"Polygon"`), 0}, {strings.ReplaceAll(fixture, `[60.6,56.8]`, `[181,56.8]`), 0}, {strings.ReplaceAll(fixture, `"osm_id":123`, `"osm_id":0`), 0}, {strings.ReplaceAll(fixture, `[60.6,56.8]`, `[null,null]`), 0}} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, tc.body) }))
		c, _ := NewClient(s.URL, time.Second)
		got, err := c.Suggest(context.Background(), SearchRequest{Query: "Москва", Limit: 10})
		s.Close()
		if err != nil || len(got) != tc.count {
			t.Fatalf("%v %+v", err, got)
		}
	}
}
