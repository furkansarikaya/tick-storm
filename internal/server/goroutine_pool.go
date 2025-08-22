// Package server implements goroutine pool optimization for connection handling.
package server

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// GoroutinePool manages a pool of worker goroutines for connection handling
type GoroutinePool struct {
	workers    int32
	maxWorkers int32
	minWorkers int32
	taskQueue  chan func()
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	
	// Metrics
	activeWorkers   int32
	totalTasks      uint64
	completedTasks  uint64
	queuedTasks     int32
}

// NewGoroutinePool creates a new goroutine pool
func NewGoroutinePool(minWorkers, maxWorkers int) *GoroutinePool {
	if minWorkers <= 0 {
		minWorkers = runtime.NumCPU()
	}
	if maxWorkers <= 0 {
		maxWorkers = runtime.NumCPU() * 4
	}
	if maxWorkers < minWorkers {
		maxWorkers = minWorkers
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	pool := &GoroutinePool{
		minWorkers: int32(minWorkers),
		maxWorkers: int32(maxWorkers),
		taskQueue:  make(chan func(), maxWorkers*2), // Buffer for tasks
		ctx:        ctx,
		cancel:     cancel,
	}
	
	// Start minimum workers
	for i := 0; i < minWorkers; i++ {
		pool.startWorker()
	}
	
	// Start pool manager
	go pool.manage()
	
	return pool
}

// Submit submits a task to the pool
func (p *GoroutinePool) Submit(task func()) bool {
	select {
	case p.taskQueue <- task:
		atomic.AddUint64(&p.totalTasks, 1)
		atomic.AddInt32(&p.queuedTasks, 1)
		
		// Auto-scale if needed
		if p.shouldScaleUp() {
			p.scaleUp()
		}
		return true
	default:
		// Queue is full, try to scale up
		if p.scaleUp() {
			select {
			case p.taskQueue <- task:
				atomic.AddUint64(&p.totalTasks, 1)
				atomic.AddInt32(&p.queuedTasks, 1)
				return true
			default:
				return false // Still full
			}
		}
		return false
	}
}

// startWorker starts a new worker goroutine
func (p *GoroutinePool) startWorker() {
	if atomic.LoadInt32(&p.workers) >= p.maxWorkers {
		return
	}
	
	atomic.AddInt32(&p.workers, 1)
	atomic.AddInt32(&p.activeWorkers, 1)
	
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer atomic.AddInt32(&p.workers, -1)
		
		idleTimer := time.NewTimer(30 * time.Second)
		defer idleTimer.Stop()
		
		for {
			select {
			case task := <-p.taskQueue:
				if task != nil {
					atomic.AddInt32(&p.queuedTasks, -1)
					
					// Execute task
					func() {
						defer func() {
							if r := recover(); r != nil {
								// Log panic recovery
							}
							atomic.AddUint64(&p.completedTasks, 1)
						}()
						task()
					}()
					
					// Reset idle timer
					if !idleTimer.Stop() {
						select {
						case <-idleTimer.C:
						default:
						}
					}
					idleTimer.Reset(30 * time.Second)
				}
				
			case <-idleTimer.C:
				// Worker has been idle, check if we can scale down
				if p.shouldScaleDown() {
					atomic.AddInt32(&p.activeWorkers, -1)
					return
				}
				idleTimer.Reset(30 * time.Second)
				
			case <-p.ctx.Done():
				atomic.AddInt32(&p.activeWorkers, -1)
				return
			}
		}
	}()
}

// shouldScaleUp determines if the pool should add more workers
func (p *GoroutinePool) shouldScaleUp() bool {
	workers := atomic.LoadInt32(&p.workers)
	queued := atomic.LoadInt32(&p.queuedTasks)
	
	// Scale up if queue is getting full and we haven't reached max workers
	return workers < p.maxWorkers && queued > workers*2
}

// shouldScaleDown determines if the pool should remove workers
func (p *GoroutinePool) shouldScaleDown() bool {
	workers := atomic.LoadInt32(&p.workers)
	queued := atomic.LoadInt32(&p.queuedTasks)
	
	// Scale down if we have more workers than minimum and low queue
	return workers > p.minWorkers && queued < workers/4
}

// scaleUp adds a new worker if possible
func (p *GoroutinePool) scaleUp() bool {
	if atomic.LoadInt32(&p.workers) < p.maxWorkers {
		p.startWorker()
		return true
	}
	return false
}

// manage handles pool management tasks
func (p *GoroutinePool) manage() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Periodic health check and optimization
			p.optimize()
			
		case <-p.ctx.Done():
			return
		}
	}
}

// optimize performs periodic pool optimization
func (p *GoroutinePool) optimize() {
	workers := atomic.LoadInt32(&p.workers)
	queued := atomic.LoadInt32(&p.queuedTasks)
	
	// If queue is consistently high, scale up
	if queued > workers*3 && workers < p.maxWorkers {
		p.startWorker()
	}
}

// Stop gracefully stops the goroutine pool
func (p *GoroutinePool) Stop(timeout time.Duration) {
	p.cancel()
	
	// Close task queue to signal workers
	close(p.taskQueue)
	
	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// All workers finished gracefully
	case <-time.After(timeout):
		// Timeout reached
	}
}

// Stats returns pool statistics
func (p *GoroutinePool) Stats() PoolStats {
	return PoolStats{
		Workers:        atomic.LoadInt32(&p.workers),
		ActiveWorkers:  atomic.LoadInt32(&p.activeWorkers),
		QueuedTasks:    atomic.LoadInt32(&p.queuedTasks),
		TotalTasks:     atomic.LoadUint64(&p.totalTasks),
		CompletedTasks: atomic.LoadUint64(&p.completedTasks),
		MinWorkers:     p.minWorkers,
		MaxWorkers:     p.maxWorkers,
	}
}

// PoolStats represents goroutine pool statistics
type PoolStats struct {
	Workers        int32  `json:"workers"`
	ActiveWorkers  int32  `json:"active_workers"`
	QueuedTasks    int32  `json:"queued_tasks"`
	TotalTasks     uint64 `json:"total_tasks"`
	CompletedTasks uint64 `json:"completed_tasks"`
	MinWorkers     int32  `json:"min_workers"`
	MaxWorkers     int32  `json:"max_workers"`
}
