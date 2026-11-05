package scheduler

import (
	"context"
	"time"
)

type checkoutWorker interface {
	Worker(context.Context, int) error
}
type CheckoutTask struct{ service checkoutWorker }

func NewCheckoutTask(service checkoutWorker) *CheckoutTask { return &CheckoutTask{service} }
func (t *CheckoutTask) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return t.service.Worker(ctx, 25)
}
