package model

import (
	"math"
	"strings"
	"time"
)

type Stop struct {
	ID        int       `json:"id,omitempty" db:"id"`
	Name      string    `json:"name" db:"name"`
	Address   string    `json:"address" db:"address"`
	Latitude  float64   `json:"latitude" db:"latitude"`
	Longitude float64   `json:"longitude" db:"longitude"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type StopUpdate struct {
	ID        int      `json:"-" db:"id"`
	Name      *string  `json:"name" db:"name"`
	Address   *string  `json:"address" db:"address"`
	Latitude  *float64 `json:"latitude" db:"latitude"`
	Longitude *float64 `json:"longitude" db:"longitude"`
}

func (su *StopUpdate) Validate() error {
	if su.Name == nil && su.Address == nil && su.Longitude == nil && su.Latitude == nil {
		return invalidInput("no fields specified")
	}
	if (su.Name != nil && strings.TrimSpace(*su.Name) == "") || (su.Address != nil && strings.TrimSpace(*su.Address) == "") {
		return invalidInput("name and address must not be empty")
	}
	if (su.Latitude != nil && !validCoordinate(*su.Latitude, 90)) || (su.Longitude != nil && !validCoordinate(*su.Longitude, 180)) {
		return invalidInput("coordinates out of range")
	}
	return nil
}

func validCoordinate(v, limit float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= -limit && v <= limit
}

func (s *Stop) Validate() error {
	if strings.TrimSpace(s.Name) == "" || strings.TrimSpace(s.Address) == "" {
		return invalidInput("name and address are required")
	}
	if !validCoordinate(s.Latitude, 90) || !validCoordinate(s.Longitude, 180) {
		return invalidInput("coordinates out of range")
	}
	return nil
}

func (su *StopUpdate) ToMap() map[string]interface{} {
	stop := make(map[string]interface{})
	if su.Name != nil {
		stop["name"] = *su.Name
	}
	if su.Address != nil {
		stop["address"] = *su.Address
	}
	if su.Longitude != nil {
		stop["longitude"] = *su.Longitude
	}
	if su.Latitude != nil {
		stop["latitude"] = *su.Latitude
	}
	return stop
}
