package model

import (
	"errors"
	"time"
)

type Trip struct {
	ID        int       `json:"-,omitempty" db:"id"`
	BusID     int       `json:"bus_id" binding:"required" db:"bus_id"`
	DriverID  int       `json:"driver_id" binding:"required" db:"driver_id"`
	StartTime time.Time `json:"start_time" binding:"required" db:"start_time"`
	EndTime   time.Time `json:"end_time" binding:"required" db:"end_time"`
	Status    string    `json:"status" db:"status"`
	BasePrice int       `json:"base_price" binding:"required" db:"base_price"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type TripUpdate struct {
	ID        int        `db:"id"`
	BusID     *int       `json:"bus_id" db:"bus_id"`
	DriverID  *int       `json:"driver_id" db:"driver_id"`
	StartTime *time.Time `json:"start_time" db:"start_time"`
	EndTime   *time.Time `json:"end_time" db:"end_time"`
	Status    *string    `json:"status" db:"status"`
	BasePrice *int       `json:"base_price" db:"base_price"`
	CreatedAt *time.Time `json:"created_at" db:"created_at"`
	UpdatedAt *time.Time `json:"updated_at" db:"updated_at"`
}

type TripResponse struct {
	ID         int `json:"-,omitempty" db:"id"`
	ViewBus    `json:"bus"`
	DriverView `json:"driver"`
	StartTime  time.Time `json:"start_time" db:"start_time"`
	EndTime    time.Time `json:"end_time" db:"end_time"`
	Status     string    `json:"status" db:"status"`
	BasePrice  float32   `json:"base_price" db:"base_price"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

func (tu *TripUpdate) Validate() error {
	if tu.BusID == nil && tu.Status == nil && tu.BasePrice == nil && tu.EndTime == nil && tu.DriverID == nil {
		return errors.New("no fields to update")
	}
	return nil
}

func (tu *TripUpdate) ToMap() map[string]interface{} {
	output := make(map[string]interface{})
	if tu.BusID != nil {
		output["bus_id"] = *tu.BusID
	}
	if tu.DriverID != nil {
		output["driver_id"] = *tu.DriverID
	}
	if tu.StartTime != nil {
		output["start_time"] = *tu.StartTime
	}
	if tu.EndTime != nil {
		output["end_time"] = *tu.EndTime
	}
	if tu.Status != nil {
		output["status"] = *tu.Status
	}
	if tu.BasePrice != nil {
		output["base_price"] = &tu.BasePrice
	}
	return output
}
