package model

import "strings"

type Driver struct {
	ID          int          `json:"id" db:"id"`
	FirstName   string       `json:"first_name" db:"first_name"`
	LastName    string       `json:"last_name" db:"last_name"`
	MiddleName  string       `json:"middle_name" db:"middle_name"`
	PhoneNumber string       `json:"phone_number" db:"phone_number"`
	Status      DriverStatus `json:"status"`
}

type DriverView struct {
	ID          int    `json:"id" db:"id"`
	FirstName   string `json:"first_name" db:"first_name"`
	LastName    string `json:"last_name" db:"last_name"`
	MiddleName  string `json:"middle_name" db:"middle_name"`
	PhoneNumber string `json:"phone_number" db:"phone_number"`
	Status      string `json:"status" db:"d_status_name"`
}

type DriverStatus struct {
	ID   int    `json:"id" db:"id"`
	Name string `json:"name" db:"name"`
}

// DriverOutput contains the driver id and a human-readable status name.
// @Description DriverOutput contains the driver id and a human-readable status name.
type DriverOutput struct {
	ID          int    `json:"id" db:"id"`
	FirstName   string `json:"first_name" db:"first_name"`
	LastName    string `json:"last_name" db:"last_name"`
	MiddleName  string `json:"middle_name" db:"middle_name"`
	PhoneNumber string `json:"phone_number" db:"phone_number"`
	Status      string `json:"status" db:"driver_status"`
}

type DriverInput struct {
	ID          int    `json:"-" db:"id"`
	FirstName   string `json:"first_name" db:"first_name"`
	LastName    string `json:"last_name" db:"last_name"`
	MiddleName  string `json:"middle_name" db:"middle_name"`
	PhoneNumber string `json:"phone_number" db:"phone_number"`
	Status      int    `json:"status" db:"status"`
}

func (ds *DriverStatus) Validate() error {
	if strings.TrimSpace(ds.Name) == "" {
		return invalidInput("status name is required")
	}
	return nil
}

func (d *DriverInput) Validate() error {
	if strings.TrimSpace(d.FirstName) == "" || strings.TrimSpace(d.LastName) == "" || strings.TrimSpace(d.PhoneNumber) == "" {
		return invalidInput("first_name, last_name and phone_number are required")
	}
	if d.Status <= 0 {
		return invalidInput("status must be positive")
	}
	return nil
}
