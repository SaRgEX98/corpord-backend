package scheduler

import (
	"context"
	"time"

	"corpord-api/internal/logger"
)

// Task описывает задачу, которую выполняет планировщик
type Task interface {
	Run(ctx context.Context) error
}

// Scheduler управляет периодическим выполнением задач
type Scheduler struct {
	logger   *logger.Logger
	interval time.Duration
	tasks    []Task
	ticker   *time.Ticker
	done     chan struct{}
}

// New создает новый планировщик с заданным интервалом
func New(logger *logger.Logger, interval time.Duration) *Scheduler {
	return &Scheduler{
		logger:   logger,
		interval: interval,
		tasks:    []Task{},
		done:     make(chan struct{}),
	}
}

// AddTask добавляет задачу в планировщик
func (s *Scheduler) AddTask(t Task) {
	s.tasks = append(s.tasks, t)
}

// Start запускает планировщик
func (s *Scheduler) Start() {
	s.ticker = time.NewTicker(s.interval)
	go func() {
		for {
			select {
			case <-s.ticker.C:
				s.runTasks()
			case <-s.done:
				s.ticker.Stop()
				return
			}
		}
	}()
}

// Stop останавливает планировщик
func (s *Scheduler) Stop() {
	close(s.done)
}

// runTasks выполняет все задачи планировщика
func (s *Scheduler) runTasks() {
	ctx, cancel := context.WithTimeout(context.Background(), s.interval)
	defer cancel()

	for _, t := range s.tasks {
		if err := t.Run(ctx); err != nil {
			s.logger.Warnf("scheduler task failed: %v", err)
		}
	}
}
