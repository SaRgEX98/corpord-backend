package model

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// Trip contains writable trip fields and server-assigned id, created_at and updated_at in create responses.
// @Description Trip contains writable trip fields and server-assigned id, created_at and updated_at in create responses.
type Trip struct {
	ID        int       `json:"id" db:"id"`
	BusID     int       `json:"bus_id" binding:"required" db:"bus_id"`
	DriverID  int       `json:"driver_id" binding:"required" db:"driver_id"`
	StartTime time.Time `json:"start_time" binding:"required" db:"start_time"`
	EndTime   time.Time `json:"end_time" binding:"required" db:"end_time"`
	Status    string    `json:"status" db:"status"`
	BasePrice float64   `json:"base_price" db:"base_price"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// TripUpdate changes only supplied trip fields. ID comes from the URL; created_at and updated_at are ignored.
// @Description TripUpdate changes only supplied trip fields. ID comes from the URL; created_at and updated_at are ignored.
type TripUpdate struct {
	ID        int        `json:"-" db:"id"`
	BusID     *int       `json:"bus_id" db:"bus_id"`
	DriverID  *int       `json:"driver_id" db:"driver_id"`
	StartTime *time.Time `json:"start_time" db:"start_time"`
	EndTime   *time.Time `json:"end_time" db:"end_time"`
	Status    *string    `json:"status" db:"status"`
	BasePrice *float64   `json:"base_price" db:"base_price"`
	CreatedAt *time.Time `json:"created_at" db:"created_at"`
	UpdatedAt *time.Time `json:"updated_at" db:"updated_at"`
}

// TripShortInfo is a compact trip card returned by GET /trips/.
// @Description TripShortInfo is a compact trip card returned by GET /trips/.
type TripShortInfo struct {
	TripID     int       `db:"trip_id" json:"trip_id"`
	BusPlate   string    `db:"license_plate" json:"license_plate"`
	BusName    string    `db:"brand" json:"brand"`
	DriverName string    `db:"driver_name" json:"driver_name"`
	StartStop  string    `db:"start_stop" json:"start_stop"`
	EndStop    string    `db:"end_stop" json:"end_stop"`
	StartTime  time.Time `db:"start_time" json:"start_time"`
	EndTime    time.Time `db:"end_time" json:"end_time"`
	BasePrice  float64   `db:"base_price" json:"base_price"`
}

// TripResponse contains separate trip, bus and driver identifiers and stops ordered by stop_order. Stops is [] when empty; end_time can be null.
// @Description TripResponse contains separate trip, bus and driver identifiers and stops ordered by stop_order. Stops is [] when empty; end_time can be null.
type TripResponse struct {
	ID        int             `json:"id" db:"id"`
	Bus       ViewBus         `json:"bus" db:"bus"`
	Driver    DriverView      `json:"driver" db:"driver"`
	StartTime time.Time       `json:"start_time" db:"start_time"`
	EndTime   *time.Time      `json:"end_time" db:"end_time"`
	Status    string          `json:"status" db:"status"`
	BasePrice float64         `json:"base_price" db:"base_price"`
	Stops     TripStopDetails `json:"stops" db:"stops"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
}

// TripStopDetail contains the trip-stop relation id, trip_id and stop_id, schedule, price and the nested stop with its own id.
// @Description TripStopDetail contains the trip-stop relation id, trip_id and stop_id, schedule, price and the nested stop with its own id.
type TripStopDetail struct {
	TripStop
	Stop Stop `json:"stop"`
}

// TripStopDetails is an ordered JSON array of trip-stop relations with their stop details.
// @Description TripStopDetails is an ordered JSON array of trip-stop relations with their stop details.
type TripStopDetails []TripStopDetail

// Scan decodes the ordered JSON aggregate returned by PostgreSQL.
func (s *TripStopDetails) Scan(src any) error {
	var data []byte
	switch v := src.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	case nil:
		*s = TripStopDetails{}
		return nil
	default:
		return fmt.Errorf("unsupported trip stops type %T", src)
	}
	if err := json.Unmarshal(data, s); err != nil {
		return err
	}
	if *s == nil {
		*s = TripStopDetails{}
	}
	return nil
}

func validTripStatus(status string) bool {
	switch status {
	case "scheduled", "boarding", "departed", "in_transit", "arrived", "completed", "cancelled", "delayed":
		return true
	default:
		return false
	}
}

func (t *Trip) Validate() error {
	if t.BusID <= 0 || t.DriverID <= 0 {
		return invalidInput("bus_id and driver_id must be positive")
	}
	if t.StartTime.IsZero() || t.EndTime.IsZero() || !t.EndTime.After(t.StartTime) {
		return invalidInput("end_time must be after start_time")
	}
	if !validTripStatus(t.Status) {
		return invalidInput("invalid trip status")
	}
	if t.BasePrice < 0 || math.IsNaN(t.BasePrice) || math.IsInf(t.BasePrice, 0) {
		return invalidInput("base_price must be finite and non-negative")
	}
	return nil
}

func (tu *TripUpdate) Validate() error {
	if tu.BusID == nil && tu.Status == nil && tu.BasePrice == nil && tu.StartTime == nil && tu.EndTime == nil && tu.DriverID == nil {
		return invalidInput("no fields to update")
	}
	if (tu.BusID != nil && *tu.BusID <= 0) || (tu.DriverID != nil && *tu.DriverID <= 0) {
		return invalidInput("bus_id and driver_id must be positive")
	}
	if tu.Status != nil && !validTripStatus(*tu.Status) {
		return invalidInput("invalid trip status")
	}
	if tu.BasePrice != nil && (*tu.BasePrice < 0 || math.IsNaN(*tu.BasePrice) || math.IsInf(*tu.BasePrice, 0)) {
		return invalidInput("base_price must be finite and non-negative")
	}
	if (tu.StartTime != nil && tu.StartTime.IsZero()) || (tu.EndTime != nil && tu.EndTime.IsZero()) {
		return invalidInput("time must not be zero")
	}
	if tu.StartTime != nil && tu.EndTime != nil && !tu.EndTime.After(*tu.StartTime) {
		return invalidInput("end_time must be after start_time")
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
		output["base_price"] = *tu.BasePrice
	}
	return output
}
