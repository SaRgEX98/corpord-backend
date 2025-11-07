package model

import (
	"errors"
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
	ID        int      `json:"-,omitempty" db:"id"`
	Name      *string  `json:"name" db:"name"`
	Address   *string  `json:"address" db:"address"`
	Latitude  *float64 `json:"latitude" db:"latitude"`
	Longitude *float64 `json:"longitude" db:"longitude"`
}

func (su *StopUpdate) Validate() error {
	if su.Name == nil && su.Address == nil && su.Longitude == nil && su.Latitude == nil {
		return errors.New("no fields specified")
	}
	return nil
}

func (su *StopUpdate) ToMap() map[string]interface{} {
	stop := make(map[string]interface{})
	if su.Name != nil {
		stop = map[string]interface{}{}
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
