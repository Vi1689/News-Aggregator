package worker

import (
	"context"
	"log"
	"sync"
	"time"
)

type WorkerPool struct {
	workers int
	tasks   chan func()
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewWorkerPool(workers int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		workers: workers,
		tasks:   make(chan func(), 1000),
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (wp *WorkerPool) Start() {
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
	log.Printf("Started %d workers", wp.workers)
}

func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()
	for {
		select {
		case <-wp.ctx.Done():
			log.Printf("Worker %d stopping", id)
			return
		case task := <-wp.tasks:
			wp.executeTask(task)
		}
	}
}

func (wp *WorkerPool) executeTask(task func()) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Worker panic recovered: %v", r)
		}
	}()
	task()
}

func (wp *WorkerPool) Submit(task func()) {
	select {
	case wp.tasks <- task:
	case <-wp.ctx.Done():
		log.Println("Worker pool stopped, task rejected")
	}
}

func (wp *WorkerPool) Stop() {
	log.Println("Stopping worker pool...")
	wp.cancel()
	close(wp.tasks)
	wp.wg.Wait()
	log.Println("Worker pool stopped")
}

// Фоновый воркер для периодических задач
type Scheduler struct {
	workers map[string]*scheduledTask
	mu      sync.RWMutex
}

type scheduledTask struct {
	name     string
	interval time.Duration
	fn       func()
	stop     chan struct{}
	running  bool
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		workers: make(map[string]*scheduledTask),
	}
}

func (s *Scheduler) Schedule(name string, interval time.Duration, fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task, exists := s.workers[name]; exists && task.running {
		task.stop <- struct{}{}
	}

	task := &scheduledTask{
		name:     name,
		interval: interval,
		fn:       fn,
		stop:     make(chan struct{}),
		running:  true,
	}

	s.workers[name] = task

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("Scheduled task %s panic: %v", name, r)
						}
					}()
					fn()
				}()
			case <-task.stop:
				task.running = false
				log.Printf("Scheduled task %s stopped", name)
				return
			}
		}
	}()

	log.Printf("Scheduled task %s with interval %v", name, interval)
}

func (s *Scheduler) Stop(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task, exists := s.workers[name]; exists && task.running {
		task.stop <- struct{}{}
		delete(s.workers, name)
	}
}

func (s *Scheduler) StopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for name, task := range s.workers {
		if task.running {
			task.stop <- struct{}{}
		}
		delete(s.workers, name)
	}
	log.Println("All scheduled tasks stopped")
}
