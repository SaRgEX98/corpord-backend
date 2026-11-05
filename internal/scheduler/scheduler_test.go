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

type functionTask func(context.Context) error

func (f functionTask) Run(ctx context.Context) error { return f(ctx) }
func TestWorkerBudgetCanExceedPollCadence(t *testing.T) {
	s := NewWithTaskTimeout(&logger.Logger{SugaredLogger: zap.NewNop().Sugar()}, time.Millisecond, time.Second)
	completed := false
	s.AddTask(functionTask(func(ctx context.Context) error {
		select {
		case <-time.After(20 * time.Millisecond):
			completed = true
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}))
	s.runTasks()
	s.Stop()
	if !completed {
		t.Fatal("network-backed worker was canceled at poll cadence instead of its execution budget")
	}
}
