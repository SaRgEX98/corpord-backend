package model

import (
	"errors"
	"time"
)

type Bus struct {
	ID           int       `json:"id" db:"id"`
	LicensePlate string    `json:"license_plate" db:"license_plate"`
	Brand        string    `json:"brand" db:"brand"`
	Capacity     int       `json:"capacity" db:"capacity"`
	CategoryID   int       `json:"category_id" db:"category_id"`
	StatusID     int       `json:"status_id" db:"status_id"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`

	Category *BusCategory `json:"category" db:"category"`
	Status   *BusStatus   `json:"status" db:"status"`
}

type BusCreate struct {
	LicensePlate string `json:"license_plate"`
	Brand        string `json:"brand"`
	Capacity     int    `json:"capacity"`
	CategoryID   int    `json:"category_id"`
	StatusID     int    `json:"status_id"`
}

type BusUpdate struct {
	ID           int     `json:"id,omitempty" db:"id"`
	LicensePlate *string `json:"license_plate"`
	Brand        *string `json:"brand"`
	Capacity     *int    `json:"capacity"`
	CategoryID   *int    `json:"category_id"`
	StatusID     *int    `json:"status_id"`
}

type BusCategory struct {
	ID   int    `json:"id" db:"id"`
	Name string `json:"name" db:"name"`
}

type BusStatus struct {
	ID   int    `json:"id" db:"id"`
	Name string `json:"name" db:"name"`
}

func (b *BusUpdate) Validate() error {
	if b.LicensePlate == nil && b.Brand == nil && b.Capacity == nil && b.CategoryID == nil && b.StatusID == nil {
		return errors.New("at least one field must be updated")
	}
	return nil
}
