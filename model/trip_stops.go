package model

import (
	"errors"
	"time"
)

type TripStop struct {
	ID            int       `json:"id" db:"id"`
	TripID        int       `json:"trip_id" db:"trip_id"`
	StopID        int       `json:"stop_id" db:"stop_id"`
	ArrivalTime   time.Time `json:"arrival_time" db:"arrival_time"`
	DepartureTime time.Time `json:"departure_time" db:"departure_time"`
	StopOrder     int       `json:"stop_order" db:"stop_order"`
	PriceToNext   int       `json:"price_to_next" db:"price_to_next"`
}

type TripStopUpdate struct {
	ID            int        `json:"-,omitempty" db:"id"`
	TripID        *int       `json:"trip_id" db:"trip_id"`
	StopID        *int       `json:"stop_id" db:"stop_id"`
	ArrivalTime   *time.Time `json:"arrival_time" db:"arrival_time"`
	DepartureTime *time.Time `json:"departure_time" db:"departure_time"`
	StopOrder     *int       `json:"stop_order" db:"stop_order"`
	PriceToNext   *int       `json:"price_to_next" db:"price_to_next"`
}

func (tu *TripStopUpdate) Validate() error {
	if tu.ArrivalTime == nil && tu.DepartureTime == nil && tu.StopOrder == nil && tu.PriceToNext == nil &&
		tu.TripID == nil && tu.StopID == nil {
		return errors.New("no fields to update")
	}
	return nil
}

func (tu *TripStopUpdate) ToMap() map[string]interface{} {
	var result map[string]interface{}
	if tu.ArrivalTime != nil {
		result["arrival_time"] = tu.ArrivalTime.Format(time.RFC3339)
	}
	if tu.DepartureTime != nil {
		result["departure_time"] = tu.DepartureTime.Format(time.RFC3339)
	}
	if tu.StopOrder != nil {
		result["stop_order"] = tu.StopOrder
	}
	if tu.PriceToNext != nil {
		result["price_to_next"] = tu.PriceToNext
	}
	if tu.TripID != nil {
		result["trip_id"] = tu.TripID
	}
	if tu.StopID != nil {
		result["stop_id"] = tu.StopID
	}
	return result
}
