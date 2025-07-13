package pools

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// WorkType defines the type of work to be performed
type WorkType int

const (
	WorkTypeNewConnection WorkType = iota
	WorkTypeMenuAction
	WorkTypeGameIO
	WorkTypeAuthentication
	WorkTypeCleanup
	WorkTypeStreamManagement
)

// HandlerFunc defines the function signature for work handlers
type HandlerFunc func(ctx context.Context, conn *Connection) error

// WorkItem represents a unit of work to be processed
type WorkItem struct {
	Type        WorkType
	Connection  *Connection
	Handler     HandlerFunc
	Context     context.Context
	Priority    Priority
	QueuedAt    time.Time
	Data        interface{} // Additional data for the work item
}

// WorkerMetrics tracks worker pool performance
type WorkerMetrics struct {
	ActiveWorkers     int64
	QueuedWork        int64
	ProcessedWork     int64
	FailedWork        int64
	AverageProcessingTime time.Duration
	WorkerUtilization float64
}

// Worker represents a single worker in the pool
type Worker struct {
	id            int
	pool          *WorkerPool
	workQueue     chan WorkItem
	stopChan      chan struct{}
	currentWork   *WorkItem
	totalProcessed int64
	totalFailed   int64
	startTime     time.Time
	logger        *slog.Logger
	
	mu sync.RWMutex
}

// WorkerPool manages a pool of workers
type WorkerPool struct {
	workers       []*Worker
	workQueue     chan WorkItem
	workerCount   int
	shutdownChan  chan struct{}
	metrics       *WorkerMetrics
	logger        *slog.Logger
	
	started       bool
	stopping      bool
	workerTimeout time.Duration
	shutdownTimeout time.Duration
	
	wg sync.WaitGroup
	mu sync.RWMutex
}

// WorkerConfig holds configuration for the worker pool
type WorkerConfig struct {
	PoolSize        int
	QueueSize       int
	WorkerTimeout   time.Duration
	ShutdownTimeout time.Duration
}

// DefaultWorkerConfig returns a default worker configuration
func DefaultWorkerConfig() *WorkerConfig {
	return &WorkerConfig{
		PoolSize:        50,
		QueueSize:       1000,
		WorkerTimeout:   30 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(config *WorkerConfig, logger *slog.Logger) (*WorkerPool, error) {
	if config == nil {
		config = DefaultWorkerConfig()
	}

	if config.PoolSize <= 0 {
		return nil, fmt.Errorf("pool size must be greater than 0")
	}

	wp := &WorkerPool{
		workerCount:     config.PoolSize,
		workQueue:       make(chan WorkItem, config.QueueSize),
		shutdownChan:    make(chan struct{}),
		metrics:         &WorkerMetrics{},
		logger:          logger,
		workerTimeout:   config.WorkerTimeout,
		shutdownTimeout: config.ShutdownTimeout,
		workers:         make([]*Worker, config.PoolSize),
	}

	// Create workers
	for i := 0; i < config.PoolSize; i++ {
		wp.workers[i] = &Worker{
			id:        i,
			pool:      wp,
			workQueue: make(chan WorkItem, 1), // Buffered channel for current work
			stopChan:  make(chan struct{}),
			logger:    logger.With("worker_id", i),
		}
	}

	return wp, nil
}

// Start starts the worker pool
func (wp *WorkerPool) Start(ctx context.Context) error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if wp.started {
		return fmt.Errorf("worker pool already started")
	}

	wp.logger.Info("Starting worker pool", "worker_count", wp.workerCount)

	// Start work dispatcher
	wp.wg.Add(1)
	go wp.dispatch(ctx)

	// Start workers
	for _, worker := range wp.workers {
		wp.wg.Add(1)
		go worker.run(ctx)
	}

	// Start metrics collector
	wp.wg.Add(1)
	go wp.collectMetrics(ctx)

	wp.started = true
	return nil
}

// Stop stops the worker pool gracefully
func (wp *WorkerPool) Stop(ctx context.Context) error {
	wp.mu.Lock()
	if !wp.started || wp.stopping {
		wp.mu.Unlock()
		return nil
	}
	wp.stopping = true
	wp.mu.Unlock()

	wp.logger.Info("Stopping worker pool")

	// Signal shutdown
	close(wp.shutdownChan)

	// Close work queue
	close(wp.workQueue)

	// Stop all workers
	for _, worker := range wp.workers {
		close(worker.stopChan)
	}

	// Wait for shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, wp.shutdownTimeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		wp.logger.Info("Worker pool stopped gracefully")
	case <-shutdownCtx.Done():
		wp.logger.Warn("Worker pool shutdown timeout exceeded")
	}

	return nil
}

// Submit submits work to the pool
func (wp *WorkerPool) Submit(work *WorkItem) error {
	wp.mu.RLock()
	if wp.stopping {
		wp.mu.RUnlock()
		return fmt.Errorf("worker pool is shutting down")
	}
	wp.mu.RUnlock()

	work.QueuedAt = time.Now()

	select {
	case wp.workQueue <- *work:
		atomic.AddInt64(&wp.metrics.QueuedWork, 1)
		return nil
	case <-time.After(wp.workerTimeout):
		return fmt.Errorf("work submission timeout")
	case <-work.Context.Done():
		return work.Context.Err()
	}
}

// GetMetrics returns current worker pool metrics
func (wp *WorkerPool) GetMetrics() WorkerMetrics {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	metrics := *wp.metrics
	
	// Calculate active workers
	var activeWorkers int64
	for _, worker := range wp.workers {
		if worker.isWorking() {
			activeWorkers++
		}
	}
	metrics.ActiveWorkers = activeWorkers

	// Calculate utilization
	if wp.workerCount > 0 {
		metrics.WorkerUtilization = float64(activeWorkers) / float64(wp.workerCount)
	}

	metrics.QueuedWork = int64(len(wp.workQueue))

	return metrics
}

// dispatch distributes work items to available workers
func (wp *WorkerPool) dispatch(ctx context.Context) {
	defer wp.wg.Done()

	for {
		select {
		case work, ok := <-wp.workQueue:
			if !ok {
				return // Work queue closed
			}
			
			// Find available worker
			if err := wp.assignWork(work); err != nil {
				wp.logger.Error("Failed to assign work", "error", err, "work_type", work.Type)
				atomic.AddInt64(&wp.metrics.FailedWork, 1)
			}

		case <-ctx.Done():
			return
		case <-wp.shutdownChan:
			return
		}
	}
}

// assignWork assigns work to an available worker
func (wp *WorkerPool) assignWork(work WorkItem) error {
	// Try to find an available worker
	for _, worker := range wp.workers {
		select {
		case worker.workQueue <- work:
			return nil
		default:
			// Worker is busy, try next one
			continue
		}
	}

	// No available workers, work will be retried
	return fmt.Errorf("no available workers")
}

// collectMetrics periodically collects worker pool metrics
func (wp *WorkerPool) collectMetrics(ctx context.Context) {
	defer wp.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			wp.updateMetrics()
		case <-ctx.Done():
			return
		case <-wp.shutdownChan:
			return
		}
	}
}

// updateMetrics updates worker pool metrics
func (wp *WorkerPool) updateMetrics() {
	var totalProcessed, totalFailed int64
	var totalProcessingTime time.Duration
	var samples int

	for _, worker := range wp.workers {
		worker.mu.RLock()
		totalProcessed += worker.totalProcessed
		totalFailed += worker.totalFailed
		worker.mu.RUnlock()
	}

	wp.mu.Lock()
	wp.metrics.ProcessedWork = totalProcessed
	wp.metrics.FailedWork = totalFailed

	if samples > 0 {
		wp.metrics.AverageProcessingTime = totalProcessingTime / time.Duration(samples)
	}
	wp.mu.Unlock()
}

// Worker methods

// run runs the worker loop
func (w *Worker) run(ctx context.Context) {
	defer w.pool.wg.Done()
	
	w.startTime = time.Now()
	w.logger.Debug("Worker started")

	for {
		select {
		case work := <-w.workQueue:
			w.processWork(ctx, work)
		case <-ctx.Done():
			w.logger.Debug("Worker stopped due to context cancellation")
			return
		case <-w.stopChan:
			w.logger.Debug("Worker stopped")
			return
		}
	}
}

// processWork processes a single work item
func (w *Worker) processWork(ctx context.Context, work WorkItem) {
	w.mu.Lock()
	w.currentWork = &work
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.currentWork = nil
		w.mu.Unlock()
	}()

	startTime := time.Now()
	
	w.logger.Debug("Processing work", 
		"work_type", work.Type, 
		"priority", work.Priority,
		"queue_time", startTime.Sub(work.QueuedAt))

	// Create timeout context for work execution
	workCtx, cancel := context.WithTimeout(work.Context, w.pool.workerTimeout)
	defer cancel()

	// Execute the work
	var err error
	if work.Handler != nil {
		err = work.Handler(workCtx, work.Connection)
	} else {
		err = fmt.Errorf("no handler defined for work type %v", work.Type)
	}

	processingTime := time.Since(startTime)

	// Update worker metrics
	w.mu.Lock()
	if err != nil {
		w.totalFailed++
		w.logger.Error("Work failed", 
			"work_type", work.Type,
			"error", err,
			"processing_time", processingTime)
	} else {
		w.totalProcessed++
		w.logger.Debug("Work completed", 
			"work_type", work.Type,
			"processing_time", processingTime)
	}
	w.mu.Unlock()

	// Update pool metrics
	if err != nil {
		atomic.AddInt64(&w.pool.metrics.FailedWork, 1)
	} else {
		atomic.AddInt64(&w.pool.metrics.ProcessedWork, 1)
	}
}

// isWorking returns true if the worker is currently processing work
func (w *Worker) isWorking() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.currentWork != nil
}

// GetStats returns worker statistics
func (w *Worker) GetStats() (int64, int64) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.totalProcessed, w.totalFailed
}

// GetWorkType returns a string representation of the work type
func (wt WorkType) String() string {
	switch wt {
	case WorkTypeNewConnection:
		return "NewConnection"
	case WorkTypeMenuAction:
		return "MenuAction"
	case WorkTypeGameIO:
		return "GameIO"
	case WorkTypeAuthentication:
		return "Authentication"
	case WorkTypeCleanup:
		return "Cleanup"
	case WorkTypeStreamManagement:
		return "StreamManagement"
	default:
		return "Unknown"
	}
}

// GetPriority returns a string representation of the priority
func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "Low"
	case PriorityNormal:
		return "Normal"
	case PriorityHigh:
		return "High"
	case PriorityCritical:
		return "Critical"
	default:
		return "Unknown"
	}
}