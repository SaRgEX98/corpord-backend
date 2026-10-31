package model

import (
	"encoding/json"
	"errors"
	"math"
	"testing"
	"time"
)

func TestTripUpdateAcceptsStartTimeOnlyAndPreservesDecimalPrice(t *testing.T) {
	start := time.Date(2026, 12, 1, 10, 0, 0, 0, time.UTC)
	update := TripUpdate{StartTime: &start}
	if err := update.Validate(); err != nil {
		t.Fatalf("start_time-only rejected: %v", err)
	}
	price := 123.45
	update.BasePrice = &price
	if got, ok := update.ToMap()["base_price"].(float64); !ok || got != price {
		t.Fatalf("price must be value float64, got %#v", update.ToMap()["base_price"])
	}
}

func TestDomainValidatorsRejectInvalidInputs(t *testing.T) {
	now := time.Now()
	earlier := now.Add(-time.Hour)
	empty := " "
	negative := -1
	badLatitude := 91.0
	infinite := math.Inf(1)
	cases := []struct {
		name  string
		input interface{ Validate() error }
	}{
		{"empty category", &BusCategory{}}, {"empty bus status", &BusStatus{}}, {"empty driver status", &DriverStatus{}},
		{"empty bus update", &BusUpdate{}}, {"blank bus brand", &BusUpdate{Brand: &empty}},
		{"negative capacity", &BusUpdate{Capacity: &negative}}, {"invalid bus", &Bus{LicensePlate: "A", Brand: "B", Capacity: 0, CategoryID: 1, StatusID: 1}},
		{"invalid driver", &DriverInput{FirstName: "A", LastName: "B", PhoneNumber: "1", Status: 0}},
		{"invalid latitude", &StopUpdate{Latitude: &badLatitude}}, {"empty stop", &Stop{}},
		{"empty trip", &TripUpdate{}}, {"negative bus ID", &TripUpdate{BusID: &negative}},
		{"nonfinite price", &TripUpdate{BasePrice: &infinite}}, {"reversed trip", &TripUpdate{StartTime: &now, EndTime: &earlier}},
		{"invalid trip stop", &TripStop{TripID: 1, StopID: 1, StopOrder: 1, ArrivalTime: now, DepartureTime: earlier}},
		{"negative stop order", &TripStopUpdate{StopOrder: &negative}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate()
			var validation *ValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("want ValidationError, got %v", err)
			}
		})
	}
}

func TestUpdateIDsCannotBeSuppliedInJSON(t *testing.T) {
	type idCase struct {
		name   string
		target any
		id     func() int
	}
	bus := BusUpdate{ID: 1}
	stop := StopUpdate{ID: 2}
	trip := TripUpdate{ID: 3}
	tripStop := TripStopUpdate{ID: 4}
	driver := DriverInput{ID: 5}
	cases := []idCase{
		{"bus", &bus, func() int { return bus.ID }},
		{"stop", &stop, func() int { return stop.ID }},
		{"trip", &trip, func() int { return trip.ID }},
		{"trip stop", &tripStop, func() int { return tripStop.ID }},
		{"driver", &driver, func() int { return driver.ID }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := tc.id()
			if err := json.Unmarshal([]byte(`{"id":999,"ID":888,"-":777}`), tc.target); err != nil {
				t.Fatal(err)
			}
			if tc.id() != before {
				t.Fatalf("client replaced path ID: %d", tc.id())
			}
		})
	}
}

func TestTripResponseKeepsIndependentIDsAndStopsArray(t *testing.T) {
	response := TripResponse{ID: 1, Bus: ViewBus{ID: 2}, Driver: DriverView{ID: 3}, Stops: TripStopDetails{}}
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got["id"] != float64(1) || got["bus"].(map[string]any)["id"] != float64(2) || got["driver"].(map[string]any)["id"] != float64(3) {
		t.Fatalf("IDs lost/flattened: %s", data)
	}
	if stops, ok := got["stops"].([]any); !ok || len(stops) != 0 {
		t.Fatalf("expected stops:[], got %s", data)
	}
	if _, exists := got["-"]; exists {
		t.Fatalf("invalid JSON property: %s", data)
	}
}
