package routes

import (
	"testing"
	"time"
)

func TestValidationAndCursorScope(t *testing.T) {
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	for _, id := range []string{"1", "city:0", "place:1", "stop:-1", "city:1:2"} {
		if _, _, err := ParseID(id); err == nil {
			t.Errorf("accepted %q", id)
		}
	}
	q := Search{From: "stop:1", To: "city:2", Date: "2026-12-01"}
	if err := q.Validate(false, now); err != nil {
		t.Fatal(err)
	}
	departure := now.Add(time.Hour)
	amount := int64(30)
	seg := Segment{TripID: 3, BoardingRelation: 4, AlightingRelation: 5, DepartureTime: departure, AmountMinor: &amount}
	q.Cursor = *encodeCursor(q, seg)
	if _, err := decodeCursor(q); err != nil {
		t.Fatal(err)
	}
	q.To = "city:3"
	if _, err := decodeCursor(q); err == nil {
		t.Fatal("cursor reused across routes")
	}
	for _, date := range []string{"2026-02-30", "2028-01-01", "garbage"} {
		q.Date = date
		if q.Validate(false, now) == nil {
			t.Errorf("accepted date %s", date)
		}
	}
	if ValidTimezone("Local") || ValidTimezone("guess") {
		t.Fatal("invalid zone accepted")
	}
}
func TestPriceUnknownAndDepartureTimezone(t *testing.T) {
	now := time.Now().UTC()
	amount := int64(10)
	s := Segment{DepartureTime: time.Date(2026, 11, 30, 22, 0, 0, 0, time.UTC), Timezone: "Asia/Yekaterinburg", ArrivalTimezone: "Europe/Moscow", AmountMinor: &amount}
	if err := s.decorate(now); err != nil {
		t.Fatal(err)
	}
	if s.DepartureTime.Format("2006-01-02") != "2026-12-01" || s.Price.AmountMinor != 10 || s.Availability.Status != "unknown" || s.Availability.Seats != nil {
		t.Fatalf("bad decoration %+v", s)
	}
	s.AmountMinor = nil
	s.Price = nil
	if err := s.decorate(now); err != nil || s.Price != nil {
		t.Fatalf("unknown price %+v %v", s, err)
	}
	s.AmountMinor = &amount
	s.ArrivalTimezone = ""
	if err := s.decorate(now); err != nil || s.Price != nil {
		t.Fatal("unknown arrival must not get price")
	}
}
