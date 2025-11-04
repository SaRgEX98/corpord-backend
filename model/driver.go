package model

import "errors"

type Driver struct {
	ID          int          `json:"id" db:"id"`
	FirstName   string       `json:"first_name" db:"first_name"`
	LastName    string       `json:"last_name" db:"last_name"`
	MiddleName  string       `json:"middle_name" db:"middle_name"`
	PhoneNumber string       `json:"phone_number" db:"phone_number"`
	Status      DriverStatus `json:"status"`
}

type DriverStatus struct {
	ID   int    `json:"id" db:"id"`
	Name string `json:"name" db:"name"`
}

type DriverOutput struct {
	ID          int    `json:"-,omitempty" db:"id"`
	FirstName   string `json:"first_name" db:"first_name"`
	LastName    string `json:"last_name" db:"last_name"`
	MiddleName  string `json:"middle_name" db:"middle_name"`
	PhoneNumber string `json:"phone_number" db:"phone_number"`
	Status      string `json:"status" db:"driver_status"`
}

type DriverInput struct {
	ID          int    `json:"-,omitempty" db:"id"`
	FirstName   string `json:"first_name" db:"first_name"`
	LastName    string `json:"last_name" db:"last_name"`
	MiddleName  string `json:"middle_name" db:"middle_name"`
	PhoneNumber string `json:"phone_number" db:"phone_number"`
	Status      int    `json:"status" db:"status"`
}

func (ds *DriverStatus) Validate() error {
	if &ds.ID == nil && &ds.Name == nil {
		return errors.New("id or name is required")
	}
	return nil
}
