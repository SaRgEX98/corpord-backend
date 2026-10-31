// Package geo adapts a configured self-hosted Photon instance. It has no public fallback.
package geo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	ErrInvalidRequest = errors.New("invalid geo request")
	ErrUnavailable    = errors.New("geo provider unavailable")
	ErrRateLimited    = errors.New("geo provider rate limited")
)

const maxResponseBytes = 1 << 20

type SearchRequest struct {
	Query    string
	Limit    int
	Lat, Lon *float64
	Language string
}

// Place IDs identify external OSM objects, never Corpord cities/stops or bookable routes.
type Place struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Label     string  `json:"label"`
	Kind      string  `json:"kind"`
	City      string  `json:"city"`
	Region    string  `json:"region"`
	Country   string  `json:"country"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}
type Client struct {
	endpoint string
	http     *http.Client
}

func NewClient(baseURL string, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || timeout <= 0 || timeout > 10*time.Second {
		return nil, fmt.Errorf("invalid Photon URL or timeout")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api"
	return &Client{endpoint: u.String(), http: &http.Client{Timeout: timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func validCoordinate(v float64, max float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= -max && v <= max
}
func (c *Client) Suggest(ctx context.Context, input SearchRequest) ([]Place, error) {
	q := strings.TrimSpace(input.Query)
	if !utf8.ValidString(q) || utf8.RuneCountInString(q) < 2 || utf8.RuneCountInString(q) > 200 || strings.IndexFunc(q, unicode.IsControl) >= 0 || input.Limit < 1 || input.Limit > 10 || (input.Lat == nil) != (input.Lon == nil) {
		return nil, ErrInvalidRequest
	}
	if input.Lat != nil && (!validCoordinate(*input.Lat, 90) || !validCoordinate(*input.Lon, 180)) {
		return nil, ErrInvalidRequest
	}
	lang := input.Language
	if lang == "" {
		lang = "ru"
	}
	if lang != "ru" && lang != "en" {
		return nil, ErrInvalidRequest
	}
	u, _ := url.Parse(c.endpoint)
	params := url.Values{"q": {q}, "limit": {strconv.Itoa(input.Limit)}, "lang": {lang}, "countrycode": {"RU"}}
	if input.Lat != nil {
		params.Set("lat", strconv.FormatFloat(*input.Lat, 'f', -1, 64))
		params.Set("lon", strconv.FormatFloat(*input.Lon, 'f', -1, 64))
	}
	u.RawQuery = params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, ErrInvalidRequest
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ErrUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil || len(data) > maxResponseBytes {
		return nil, ErrUnavailable
	}
	// Explicit wire tags for Photon underscore names and address fields.
	var wire struct {
		Features []struct {
			Geometry struct {
				Type        string     `json:"type"`
				Coordinates []*float64 `json:"coordinates"`
			} `json:"geometry"`
			Properties struct {
				Name        string      `json:"name"`
				Street      string      `json:"street"`
				HouseNumber string      `json:"housenumber"`
				City        string      `json:"city"`
				State       string      `json:"state"`
				Country     string      `json:"country"`
				CountryCode string      `json:"countrycode"`
				OSMType     string      `json:"osm_type"`
				OSMValue    string      `json:"osm_value"`
				OSMID       json.Number `json:"osm_id"`
			} `json:"properties"`
		} `json:"features"`
	}
	if json.Unmarshal(data, &wire) != nil || wire.Features == nil {
		return nil, ErrUnavailable
	}
	places := make([]Place, 0, input.Limit)
	seen := map[string]bool{}
	for _, feature := range wire.Features {
		p := feature.Properties
		xy := feature.Geometry.Coordinates
		if feature.Geometry.Type != "Point" || len(xy) != 2 || xy[0] == nil || xy[1] == nil || !validCoordinate(*xy[0], 180) || !validCoordinate(*xy[1], 90) || !strings.EqualFold(p.CountryCode, "RU") {
			continue
		}
		id, err := p.OSMID.Int64()
		if err != nil || id <= 0 || (p.OSMType != "N" && p.OSMType != "W" && p.OSMType != "R") {
			continue
		}
		externalID := fmt.Sprintf("osm:%s:%d", p.OSMType, id)
		if seen[externalID] {
			continue
		}
		seen[externalID] = true
		name := strings.TrimSpace(p.Name)
		if name == "" {
			name = strings.TrimSpace(p.Street + " " + p.HouseNumber)
		}
		if name == "" {
			continue
		}
		parts := []string{name}
		for _, s := range []string{strings.TrimSpace(p.Street + " " + p.HouseNumber), p.City, p.State, p.Country} {
			if s != "" && s != parts[len(parts)-1] && s != name {
				parts = append(parts, s)
			}
		}
		places = append(places, Place{ID: externalID, Name: name, Label: strings.Join(parts, ", "), Kind: p.OSMValue, City: p.City, Region: p.State, Country: p.Country, Latitude: *xy[1], Longitude: *xy[0]})
		if len(places) == input.Limit {
			break
		}
	}
	return places, nil
}
