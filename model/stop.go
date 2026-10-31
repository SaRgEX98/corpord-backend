package model

import (
	"math"
	"strings"
	"time"
)

type Stop struct {
	EffectiveTimezone *string   `json:"effective_timezone" db:"effective_timezone"`
	CityID            *int      `json:"city_id" db:"city_id"`
	Timezone          *string   `json:"timezone" db:"timezone"`
	ID                int       `json:"id,omitempty" db:"id"`
	Name              string    `json:"name" db:"name"`
	Address           string    `json:"address" db:"address"`
	Latitude          float64   `json:"latitude" db:"latitude"`
	Longitude         float64   `json:"longitude" db:"longitude"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`
}

type StopUpdate struct {
	CityID    *int     `json:"city_id" db:"city_id"`
	Timezone  *string  `json:"timezone" db:"timezone"`
	ID        int      `json:"-" db:"id"`
	Name      *string  `json:"name" db:"name"`
	Address   *string  `json:"address" db:"address"`
	Latitude  *float64 `json:"latitude" db:"latitude"`
	Longitude *float64 `json:"longitude" db:"longitude"`
}

func (su *StopUpdate) Validate() error {
	if err := validateStopLocation(su.CityID, su.Timezone); err != nil {
		return err
	}
	if su.Name == nil && su.Address == nil && su.Longitude == nil && su.Latitude == nil && su.CityID == nil && su.Timezone == nil {
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
	if err := validateStopLocation(s.CityID, s.Timezone); err != nil {
		return err
	}
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
	if su.CityID != nil {
		if *su.CityID == 0 {
			stop["city_id"] = nil
		} else {
			stop["city_id"] = *su.CityID
		}
	}
	if su.Timezone != nil {
		if *su.Timezone == "" {
			stop["timezone"] = nil
		} else {
			stop["timezone"] = *su.Timezone
		}
	}
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

func validateStopLocation(city *int, tz *string) error {
	if city != nil && *city < 0 {
		return invalidInput("city_id must be positive or zero to clear")
	}
	if tz != nil && *tz != "" && !validStopTimezone(*tz) {
		return invalidInput("invalid IANA timezone")
	}
	return nil
}

func validStopTimezone(tz string) bool {
	if tz == "Local" {
		return false
	}
	_, err := time.LoadLocation(tz)
	return err == nil
}
