package scheduler

import "context"

// Expiry work is bounded; correctness also checks DB time on every allocation.
type holdExpirer interface {
	Expire(context.Context, int) error
}

type BookingExpiryTask struct{ service holdExpirer }

func NewBookingExpiryTask(service holdExpirer) *BookingExpiryTask {
	return &BookingExpiryTask{service: service}
}

func (t *BookingExpiryTask) Run(ctx context.Context) error {
	return t.service.Expire(ctx, 250)
}
