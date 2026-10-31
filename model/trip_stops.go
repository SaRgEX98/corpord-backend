package model

import (
	"math"
	"time"
)

type TripStop struct {
	ID            int       `json:"id" db:"id"`
	TripID        int       `json:"trip_id" db:"trip_id"`
	StopID        int       `json:"stop_id" db:"stop_id"`
	ArrivalTime   time.Time `json:"arrival_time" db:"arrival_time"`
	DepartureTime time.Time `json:"departure_time" db:"departure_time"`
	StopOrder     int       `json:"stop_order" db:"stop_order"`
	PriceToNext   float64   `json:"price_to_next" db:"price_to_next"`
}

type TripStopResponse struct {
	TripID        int       `json:"trip_id" db:"trip_id"`
	Stop          string    `json:"stop" db:"stop"`
	ArrivalTime   time.Time `json:"arrival_time" db:"arrival_time"`
	DepartureTime time.Time `json:"departure_time" db:"departure_time"`
	StopOrder     int       `json:"stop_order" db:"stop_order"`
	PriceToNext   float64   `json:"price_to_next" db:"price_to_next"`
}

type TripStopUpdate struct {
	ID            int        `json:"-" db:"id"`
	TripID        *int       `json:"trip_id" db:"trip_id"`
	StopID        *int       `json:"stop_id" db:"stop_id"`
	ArrivalTime   *time.Time `json:"arrival_time" db:"arrival_time"`
	DepartureTime *time.Time `json:"departure_time" db:"departure_time"`
	StopOrder     *int       `json:"stop_order" db:"stop_order"`
	PriceToNext   *float64   `json:"price_to_next" db:"price_to_next"`
}

func (tu *TripStopUpdate) Validate() error {
	if tu.ArrivalTime == nil && tu.DepartureTime == nil && tu.StopOrder == nil && tu.PriceToNext == nil &&
		tu.TripID == nil && tu.StopID == nil {
		return invalidInput("no fields to update")
	}
	if (tu.TripID != nil && *tu.TripID <= 0) || (tu.StopID != nil && *tu.StopID <= 0) || (tu.StopOrder != nil && *tu.StopOrder <= 0) {
		return invalidInput("trip_id, stop_id and stop_order must be positive")
	}
	if tu.PriceToNext != nil && (*tu.PriceToNext < 0 || math.IsNaN(*tu.PriceToNext) || math.IsInf(*tu.PriceToNext, 0)) {
		return invalidInput("price_to_next must be finite and non-negative")
	}
	if (tu.ArrivalTime != nil && tu.ArrivalTime.IsZero()) || (tu.DepartureTime != nil && tu.DepartureTime.IsZero()) {
		return invalidInput("time must not be zero")
	}
	if tu.ArrivalTime != nil && tu.DepartureTime != nil && tu.DepartureTime.Before(*tu.ArrivalTime) {
		return invalidInput("departure_time must not precede arrival_time")
	}
	return nil
}

func (t *TripStop) Validate() error {
	if t.TripID <= 0 || t.StopID <= 0 || t.StopOrder <= 0 {
		return invalidInput("trip_id, stop_id and stop_order must be positive")
	}
	if t.ArrivalTime.IsZero() || t.DepartureTime.IsZero() || t.DepartureTime.Before(t.ArrivalTime) {
		return invalidInput("departure_time must not precede arrival_time")
	}
	if t.PriceToNext < 0 || math.IsNaN(t.PriceToNext) || math.IsInf(t.PriceToNext, 0) {
		return invalidInput("price_to_next must be finite and non-negative")
	}
	return nil
}

func (tu *TripStopUpdate) ToMap() map[string]interface{} {
	result := make(map[string]interface{})
	if tu.ArrivalTime != nil {
		result["arrival_time"] = *tu.ArrivalTime
	}
	if tu.DepartureTime != nil {
		result["departure_time"] = *tu.DepartureTime
	}
	if tu.StopOrder != nil {
		result["stop_order"] = *tu.StopOrder
	}
	if tu.PriceToNext != nil {
		result["price_to_next"] = *tu.PriceToNext
	}
	if tu.TripID != nil {
		result["trip_id"] = *tu.TripID
	}
	if tu.StopID != nil {
		result["stop_id"] = *tu.StopID
	}
	return result
}
