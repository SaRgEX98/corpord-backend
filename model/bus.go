package model

import (
	"strings"
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

// ViewBus contains the bus id and human-readable category and status names.
// @Description ViewBus contains the bus id and human-readable category and status names.
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

// BusUpdate changes only supplied fields; omitted fields are preserved and ID comes from the URL.
// @Description BusUpdate changes only supplied fields; omitted fields are preserved and ID comes from the URL.
type BusUpdate struct {
	ID           int     `json:"-" db:"id"`
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
	if strings.TrimSpace(bc.Name) == "" {
		return invalidInput("category name is required")
	}
	return nil
}

type BusStatus struct {
	ID   int    `json:"id" db:"id"`
	Name string `json:"name" db:"name"`
}

func (bs *BusStatus) Validate() error {
	if strings.TrimSpace(bs.Name) == "" {
		return invalidInput("status name is required")
	}
	return nil
}

func (b *BusUpdate) Validate() error {
	if b.LicensePlate == nil && b.Brand == nil && b.Capacity == nil && b.CategoryID == nil && b.StatusID == nil {
		return invalidInput("at least one field must be updated")
	}
	if (b.LicensePlate != nil && strings.TrimSpace(*b.LicensePlate) == "") || (b.Brand != nil && strings.TrimSpace(*b.Brand) == "") {
		return invalidInput("license_plate and brand must not be empty")
	}
	if (b.Capacity != nil && *b.Capacity <= 0) || (b.CategoryID != nil && *b.CategoryID <= 0) || (b.StatusID != nil && *b.StatusID <= 0) {
		return invalidInput("capacity, category_id and status_id must be positive")
	}
	return nil
}

func (b *Bus) Validate() error {
	if strings.TrimSpace(b.LicensePlate) == "" || strings.TrimSpace(b.Brand) == "" {
		return invalidInput("license_plate and brand are required")
	}
	if b.Capacity <= 0 || b.CategoryID <= 0 || b.StatusID <= 0 {
		return invalidInput("capacity, category_id and status_id must be positive")
	}
	return nil
}

func (b *BusUpdate) ToMap() map[string]interface{} {
	fields := make(map[string]interface{})
	if b.LicensePlate != nil {
		fields["license_plate"] = *b.LicensePlate
	}
	if b.Brand != nil {
		fields["brand"] = *b.Brand
	}
	if b.Capacity != nil {
		fields["capacity"] = *b.Capacity
	}
	if b.CategoryID != nil {
		fields["category_id"] = *b.CategoryID
	}
	if b.StatusID != nil {
		fields["status_id"] = *b.StatusID
	}
	return fields
}
