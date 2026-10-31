package scheduler

import (
	"context"
	"corpord-api/internal/logger"
	"go.uber.org/zap"
	"testing"
	"time"
)

type cancelTask struct {
	started  chan struct{}
	finished chan struct{}
}

func (t *cancelTask) Run(ctx context.Context) error {
	close(t.started)
	<-ctx.Done()
	close(t.finished)
	return ctx.Err()
}
func TestStopCancelsAndWaitsForTask(t *testing.T) {
	task := &cancelTask{started: make(chan struct{}), finished: make(chan struct{})}
	s := New(&logger.Logger{SugaredLogger: zap.NewNop().Sugar()}, time.Millisecond)
	s.AddTask(task)
	s.Start()
	select {
	case <-task.started:
	case <-time.After(time.Second):
		t.Fatal("task not started")
	}
	s.Stop()
	s.Stop()
	select {
	case <-task.finished:
	default:
		t.Fatal("Stop did not wait for canceled task")
	}
}
