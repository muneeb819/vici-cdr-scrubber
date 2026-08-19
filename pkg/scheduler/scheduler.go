package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Task represents a scheduled task
type Task struct {
	ID          string
	Name        string
	Schedule    string
	Func        func(ctx context.Context) error
	LastRun     time.Time
	NextRun     time.Time
	Interval    time.Duration
	Enabled     bool
	RetryCount  int
	MaxRetries  int
	OnComplete  func(taskID string)
	OnFailure   func(taskID string, err error)
}

// Scheduler manages scheduled tasks
type Scheduler struct {
	mu      sync.RWMutex
	tasks   map[string]*Task
	ctx     context.Context
	cancel  context.CancelFunc
	running bool
}

// NewScheduler creates a new task scheduler
func NewScheduler() *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		tasks:  make(map[string]*Task),
		ctx:    ctx,
		cancel: cancel,
	}
}

// AddTask adds a new task to the scheduler
func (s *Scheduler) AddTask(task Task) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task.MaxRetries == 0 {
		task.MaxRetries = 3
	}
	task.Enabled = true
	s.tasks[task.ID] = &task
}

// RemoveTask removes a task from the scheduler
func (s *Scheduler) RemoveTask(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tasks, taskID)
}

// EnableTask enables a task
func (s *Scheduler) EnableTask(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if task, exists := s.tasks[taskID]; exists {
		task.Enabled = true
	}
}

// DisableTask disables a task
func (s *Scheduler) DisableTask(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if task, exists := s.tasks[taskID]; exists {
		task.Enabled = false
	}
}

// Start starts the scheduler
func (s *Scheduler) Start() {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	go s.run()
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.cancel()
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
}

// run is the main scheduler loop
func (s *Scheduler) run() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.checkAndRunTasks()
		}
	}
}

// checkAndRunTasks checks and runs due tasks
func (s *Scheduler) checkAndRunTasks() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	for _, task := range s.tasks {
		if task.Enabled && now.After(task.NextRun) {
			go s.executeTask(task)
		}
	}
}

// executeTask executes a task
func (s *Scheduler) executeTask(task *Task) {
	s.mu.Lock()
	task.LastRun = time.Now()
	task.NextRun = time.Now().Add(task.Interval)
	s.mu.Unlock()

	err := task.Func(s.ctx)
	if err != nil {
		if task.RetryCount < task.MaxRetries {
			s.mu.Lock()
			task.RetryCount++
			s.mu.Unlock()
			if task.OnFailure != nil {
				task.OnFailure(task.ID, err)
			}
		} else {
			s.mu.Lock()
			task.Enabled = false
			s.mu.Unlock()
			if task.OnFailure != nil {
				task.OnFailure(task.ID, fmt.Errorf("max retries exceeded: %w", err))
			}
		}
	} else {
		s.mu.Lock()
		task.RetryCount = 0
		s.mu.Unlock()
		if task.OnComplete != nil {
			task.OnComplete(task.ID)
		}
	}
}

// GetTask returns a task by ID
func (s *Scheduler) GetTask(taskID string) *Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tasks[taskID]
}

// GetAllTasks returns all tasks
func (s *Scheduler) GetAllTasks() map[string]*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tasks
}

// IsRunning returns whether the scheduler is running
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetStatus returns scheduler status
func (s *Scheduler) GetStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	enabled := 0
	disabled := 0
	for _, task := range s.tasks {
		if task.Enabled {
			enabled++
		} else {
			disabled++
		}
	}

	return map[string]interface{}{
		"running":     s.running,
		"total_tasks": len(s.tasks),
		"enabled":     enabled,
		"disabled":    disabled,
	}
}
