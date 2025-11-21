package model

import (
	"errors"
	"time"
)

type Bus struct {
	ID           int        `json:"id" db:"id"`
	LicensePlate string     `json:"license_plate" db:"license_plate"`
	Brand        string     `json:"brand" db:"brand"`
	Capacity     int        `json:"capacity" db:"capacity"`
	CategoryID   int        `json:"category_id" db:"category_id"`
	StatusID     int        `json:"status_id" db:"status_id"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at" db:"deleted_at"`

	Category *BusCategory `json:"category" db:"category"`
	Status   *BusStatus   `json:"status" db:"status"`
}

type ViewBus struct {
	ID           int    `json:"id" binding:"required" db:"id"`
	LicensePlate string `json:"license_plate" binding:"required" db:"license_plate"`
	Brand        string `json:"brand" binding:"required" db:"brand"`
	Capacity     int    `json:"capacity" binding:"required" db:"capacity"`
	Category     string `json:"category" db:"category_name"`
	Status       string `json:"status" db:"status_name"`
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

func (bc *BusCategory) Validate() error {
	if &bc.ID == nil && &bc.Name == nil {
		return errors.New("at least one field must be updated")
	}
	return nil
}

type BusStatus struct {
	ID   int    `json:"id" db:"id"`
	Name string `json:"name" db:"name"`
}

func (bs *BusStatus) Validate() error {
	if &bs.ID == nil && &bs.Name == nil {
		return errors.New("at least one field must be updated")
	}
	return nil
}

func (b *BusUpdate) Validate() error {
	if b.LicensePlate == nil && b.Brand == nil && b.Capacity == nil && b.CategoryID == nil && b.StatusID == nil {
		return errors.New("at least one field must be updated")
	}
	return nil
}
