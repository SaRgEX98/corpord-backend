package scheduler

import (
	"context"
	"sync"
	"time"

	"corpord-api/internal/logger"
)

// Task описывает задачу, которую выполняет планировщик
type Task interface {
	Run(ctx context.Context) error
}

// Scheduler управляет периодическим выполнением задач
type Scheduler struct {
	logger    *logger.Logger
	interval  time.Duration
	tasks     []Task
	ctx       context.Context
	cancel    context.CancelFunc
	startOnce sync.Once
	stopOnce  sync.Once
	wg        sync.WaitGroup
}

// New создает новый планировщик с заданным интервалом
func New(logger *logger.Logger, interval time.Duration) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		logger:   logger,
		interval: interval,
		tasks:    []Task{},
		ctx:      ctx,
		cancel:   cancel,
	}
}

// AddTask добавляет задачу в планировщик
func (s *Scheduler) AddTask(t Task) {
	s.tasks = append(s.tasks, t)
}

// Start запускает планировщик
func (s *Scheduler) Start() {
	s.startOnce.Do(func() {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			ticker := time.NewTicker(s.interval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					s.runTasks()
				case <-s.ctx.Done():
					return
				}
			}
		}()
	})
}

// Stop останавливает планировщик
func (s *Scheduler) Stop() {
	s.stopOnce.Do(s.cancel)
	s.wg.Wait()
}

// runTasks выполняет все задачи планировщика
func (s *Scheduler) runTasks() {
	ctx, cancel := context.WithTimeout(s.ctx, s.interval)
	defer cancel()

	for _, t := range s.tasks {
		if ctx.Err() != nil {
			return
		}
		if err := t.Run(ctx); err != nil {
			s.logger.Warnf("scheduler task failed: %v", err)
		}
	}
}
