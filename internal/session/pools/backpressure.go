package pools

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// DropPolicy defines how to handle queue overflow
type DropPolicy int

const (
	DropOldest DropPolicy = iota
	DropNewest
	DropLowestPriority
)

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	CircuitStateClosed CircuitState = iota
	CircuitStateOpen
	CircuitStateHalfOpen
)

// QueueItem represents an item in a queue
type QueueItem struct {
	Data     interface{}
	Priority Priority
	QueuedAt time.Time
}

// Queue represents a bounded queue with drop policies
type Queue struct {
	items       []QueueItem
	maxSize     int
	currentSize int
	dropPolicy  DropPolicy
	dropped     int64
	
	mu sync.RWMutex
}

// CircuitBreaker implements circuit breaker pattern
type CircuitBreaker struct {
	state           CircuitState
	failureCount    int64
	successCount    int64
	threshold       int64
	timeout         time.Duration
	lastFailure     time.Time
	consecutiveSuccesses int64
	
	mu sync.RWMutex
}

// LoadShedder implements load shedding functionality
type LoadShedder struct {
	enabled           bool
	cpuThreshold      float64
	memoryThreshold   float64
	queueThreshold    float64
	recentLoad        []float64
	maxSamples        int
	
	mu sync.RWMutex
}

// BackpressureMetrics tracks backpressure management metrics
type BackpressureMetrics struct {
	CircuitState        CircuitState
	FailureCount        int64
	SuccessCount        int64
	DroppedRequests     int64
	QueueUtilization    float64
	LoadSheddingActive  bool
	SystemLoad          float64
}

// BackpressureManager manages system backpressure and load
type BackpressureManager struct {
	connectionQueue   *Queue
	workQueue         *Queue
	circuitBreaker    *CircuitBreaker
	loadShedder       *LoadShedder
	
	enabled           bool
	metrics           *BackpressureMetrics
	logger            *slog.Logger
	
	shutdownChan      chan struct{}
	
	mu                sync.RWMutex
	wg                sync.WaitGroup
}

// BackpressureConfig holds configuration for backpressure management
type BackpressureConfig struct {
	Enabled           bool
	CircuitBreaker    bool
	LoadShedding      bool
	FailureThreshold  int64
	RecoveryTimeout   time.Duration
	QueueSize         int
	CPUThreshold      float64
	MemoryThreshold   float64
}

// DefaultBackpressureConfig returns a default backpressure configuration
func DefaultBackpressureConfig() *BackpressureConfig {
	return &BackpressureConfig{
		Enabled:          true,
		CircuitBreaker:   true,
		LoadShedding:     true,
		FailureThreshold: 10,
		RecoveryTimeout:  60 * time.Second,
		QueueSize:        1000,
		CPUThreshold:     0.8,
		MemoryThreshold:  0.9,
	}
}

// NewBackpressureManager creates a new backpressure manager
func NewBackpressureManager(config *BackpressureConfig, logger *slog.Logger) (*BackpressureManager, error) {
	if config == nil {
		config = DefaultBackpressureConfig()
	}

	bm := &BackpressureManager{
		enabled:      config.Enabled,
		metrics:      &BackpressureMetrics{},
		logger:       logger,
		shutdownChan: make(chan struct{}),
	}

	// Initialize connection queue
	bm.connectionQueue = &Queue{
		maxSize:    config.QueueSize,
		dropPolicy: DropOldest,
		items:      make([]QueueItem, 0, config.QueueSize),
	}

	// Initialize work queue
	bm.workQueue = &Queue{
		maxSize:    config.QueueSize * 2, // Larger work queue
		dropPolicy: DropLowestPriority,
		items:      make([]QueueItem, 0, config.QueueSize*2),
	}

	// Initialize circuit breaker
	if config.CircuitBreaker {
		bm.circuitBreaker = &CircuitBreaker{
			state:     CircuitStateClosed,
			threshold: config.FailureThreshold,
			timeout:   config.RecoveryTimeout,
		}
		logger.Info("Circuit breaker enabled", 
			"failure_threshold", config.FailureThreshold,
			"recovery_timeout", config.RecoveryTimeout)
	}

	// Initialize load shedder
	if config.LoadShedding {
		bm.loadShedder = &LoadShedder{
			enabled:         true,
			cpuThreshold:    config.CPUThreshold,
			memoryThreshold: config.MemoryThreshold,
			queueThreshold:  0.9, // 90% queue utilization
			recentLoad:      make([]float64, 0, 10),
			maxSamples:      10,
		}
		logger.Info("Load shedding enabled",
			"cpu_threshold", config.CPUThreshold,
			"memory_threshold", config.MemoryThreshold)
	}

	logger.Info("Created backpressure manager", 
		"enabled", config.Enabled,
		"circuit_breaker", config.CircuitBreaker,
		"load_shedding", config.LoadShedding)

	return bm, nil
}

// Start starts the backpressure manager
func (bm *BackpressureManager) Start(ctx context.Context) error {
	if !bm.enabled {
		bm.logger.Info("Backpressure manager disabled, skipping start")
		return nil
	}

	bm.logger.Info("Starting backpressure manager")

	// Start monitoring routines
	bm.wg.Add(1)
	go bm.monitorSystemLoad(ctx)

	bm.wg.Add(1)
	go bm.circuitBreakerMonitor(ctx)

	bm.wg.Add(1)
	go bm.metricsRoutine(ctx)

	return nil
}

// Stop stops the backpressure manager gracefully
func (bm *BackpressureManager) Stop(ctx context.Context) error {
	if !bm.enabled {
		return nil
	}

	bm.logger.Info("Stopping backpressure manager")

	// Signal shutdown
	close(bm.shutdownChan)

	// Wait for goroutines to finish
	done := make(chan struct{})
	go func() {
		bm.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		bm.logger.Info("Backpressure manager stopped gracefully")
	case <-ctx.Done():
		bm.logger.Warn("Backpressure manager stop timeout exceeded")
	}

	return nil
}

// CanAccept determines if a new connection can be accepted
func (bm *BackpressureManager) CanAccept() bool {
	if !bm.enabled {
		return true
	}

	// Check circuit breaker
	if bm.circuitBreaker != nil && !bm.circuitBreaker.CanAccept() {
		bm.logger.Debug("Connection rejected by circuit breaker")
		return false
	}

	// Check load shedding
	if bm.loadShedder != nil && bm.loadShedder.ShouldShed() {
		bm.logger.Debug("Connection rejected by load shedder")
		return false
	}

	return true
}

// RecordSuccess records a successful operation
func (bm *BackpressureManager) RecordSuccess() {
	if bm.circuitBreaker != nil {
		bm.circuitBreaker.RecordSuccess()
	}
}

// RecordFailure records a failed operation
func (bm *BackpressureManager) RecordFailure() {
	if bm.circuitBreaker != nil {
		bm.circuitBreaker.RecordFailure()
	}
}

// GetMetrics returns current backpressure metrics
func (bm *BackpressureManager) GetMetrics() BackpressureMetrics {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	metrics := *bm.metrics

	// Update circuit breaker metrics
	if bm.circuitBreaker != nil {
		bm.circuitBreaker.mu.RLock()
		metrics.CircuitState = bm.circuitBreaker.state
		metrics.FailureCount = bm.circuitBreaker.failureCount
		metrics.SuccessCount = bm.circuitBreaker.successCount
		bm.circuitBreaker.mu.RUnlock()
	}

	// Update queue metrics
	if bm.connectionQueue != nil {
		bm.connectionQueue.mu.RLock()
		if bm.connectionQueue.maxSize > 0 {
			metrics.QueueUtilization = float64(bm.connectionQueue.currentSize) / float64(bm.connectionQueue.maxSize)
		}
		metrics.DroppedRequests = bm.connectionQueue.dropped
		bm.connectionQueue.mu.RUnlock()
	}

	// Update load shedding metrics
	if bm.loadShedder != nil {
		bm.loadShedder.mu.RLock()
		metrics.LoadSheddingActive = bm.loadShedder.ShouldShed()
		if len(bm.loadShedder.recentLoad) > 0 {
			var sum float64
			for _, load := range bm.loadShedder.recentLoad {
				sum += load
			}
			metrics.SystemLoad = sum / float64(len(bm.loadShedder.recentLoad))
		}
		bm.loadShedder.mu.RUnlock()
	}

	return metrics
}

// Queue methods

// NewQueue creates a new bounded queue
func NewQueue(maxSize int, dropPolicy DropPolicy) *Queue {
	return &Queue{
		maxSize:    maxSize,
		dropPolicy: dropPolicy,
		items:      make([]QueueItem, 0, maxSize),
	}
}

// Enqueue adds an item to the queue
func (q *Queue) Enqueue(item QueueItem) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.currentSize >= q.maxSize {
		// Apply drop policy
		switch q.dropPolicy {
		case DropOldest:
			q.items = q.items[1:]
			q.currentSize--
		case DropNewest:
			atomic.AddInt64(&q.dropped, 1)
			return false
		case DropLowestPriority:
			q.dropLowestPriority()
		}
	}

	q.items = append(q.items, item)
	q.currentSize++
	return true
}

// Dequeue removes and returns the next item from the queue
func (q *Queue) Dequeue() (QueueItem, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.currentSize == 0 {
		return QueueItem{}, false
	}

	item := q.items[0]
	q.items = q.items[1:]
	q.currentSize--
	return item, true
}

// dropLowestPriority drops the item with the lowest priority
func (q *Queue) dropLowestPriority() {
	if len(q.items) == 0 {
		return
	}

	lowestIdx := 0
	lowestPriority := q.items[0].Priority

	for i, item := range q.items {
		if item.Priority < lowestPriority {
			lowestPriority = item.Priority
			lowestIdx = i
		}
	}

	// Remove the item with lowest priority
	q.items = append(q.items[:lowestIdx], q.items[lowestIdx+1:]...)
	q.currentSize--
	atomic.AddInt64(&q.dropped, 1)
}

// CircuitBreaker methods

// CanAccept checks if the circuit breaker allows new requests
func (cb *CircuitBreaker) CanAccept() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case CircuitStateClosed:
		return true
	case CircuitStateOpen:
		return time.Since(cb.lastFailure) > cb.timeout
	case CircuitStateHalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess records a successful operation
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	atomic.AddInt64(&cb.successCount, 1)

	switch cb.state {
	case CircuitStateHalfOpen:
		cb.consecutiveSuccesses++
		if cb.consecutiveSuccesses >= 3 { // Require 3 consecutive successes
			cb.state = CircuitStateClosed
			cb.failureCount = 0
			cb.consecutiveSuccesses = 0
		}
	case CircuitStateOpen:
		// Transition to half-open if timeout has passed
		if time.Since(cb.lastFailure) > cb.timeout {
			cb.state = CircuitStateHalfOpen
			cb.consecutiveSuccesses = 1
		}
	}
}

// RecordFailure records a failed operation
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	atomic.AddInt64(&cb.failureCount, 1)
	cb.lastFailure = time.Now()
	cb.consecutiveSuccesses = 0

	if cb.failureCount >= cb.threshold {
		cb.state = CircuitStateOpen
	}
}

// LoadShedder methods

// ShouldShed determines if load shedding should be active
func (ls *LoadShedder) ShouldShed() bool {
	if !ls.enabled {
		return false
	}

	ls.mu.RLock()
	defer ls.mu.RUnlock()

	if len(ls.recentLoad) == 0 {
		return false
	}

	// Calculate average load
	var sum float64
	for _, load := range ls.recentLoad {
		sum += load
	}
	avgLoad := sum / float64(len(ls.recentLoad))

	// Shed load if average exceeds threshold
	return avgLoad > ls.cpuThreshold
}

// UpdateLoad updates the current system load
func (ls *LoadShedder) UpdateLoad(cpuLoad, memoryLoad, queueLoad float64) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	// Use the highest load metric
	load := cpuLoad
	if memoryLoad > load {
		load = memoryLoad
	}
	if queueLoad > load {
		load = queueLoad
	}

	// Add to recent load samples
	ls.recentLoad = append(ls.recentLoad, load)

	// Keep only recent samples
	if len(ls.recentLoad) > ls.maxSamples {
		ls.recentLoad = ls.recentLoad[1:]
	}
}

// Monitor routines

// monitorSystemLoad monitors system resource usage
func (bm *BackpressureManager) monitorSystemLoad(ctx context.Context) {
	defer bm.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	bm.logger.Debug("System load monitoring started")

	for {
		select {
		case <-ticker.C:
			bm.updateSystemLoad()
		case <-ctx.Done():
			bm.logger.Debug("System load monitoring stopped due to context cancellation")
			return
		case <-bm.shutdownChan:
			bm.logger.Debug("System load monitoring stopped")
			return
		}
	}
}

// updateSystemLoad updates current system load metrics
func (bm *BackpressureManager) updateSystemLoad() {
	if bm.loadShedder == nil {
		return
	}

	// TODO: Implement actual system metrics collection
	// For now, use placeholder values
	cpuLoad := 0.5    // 50% CPU usage
	memoryLoad := 0.6 // 60% memory usage
	queueLoad := 0.3  // 30% queue utilization

	bm.loadShedder.UpdateLoad(cpuLoad, memoryLoad, queueLoad)

	if bm.loadShedder.ShouldShed() {
		bm.logger.Debug("Load shedding active",
			"cpu_load", cpuLoad,
			"memory_load", memoryLoad,
			"queue_load", queueLoad)
	}
}

// circuitBreakerMonitor monitors circuit breaker state
func (bm *BackpressureManager) circuitBreakerMonitor(ctx context.Context) {
	defer bm.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	bm.logger.Debug("Circuit breaker monitoring started")

	var lastState CircuitState = CircuitStateClosed

	for {
		select {
		case <-ticker.C:
			if bm.circuitBreaker != nil {
				bm.circuitBreaker.mu.RLock()
				currentState := bm.circuitBreaker.state
				failureCount := bm.circuitBreaker.failureCount
				successCount := bm.circuitBreaker.successCount
				bm.circuitBreaker.mu.RUnlock()

				if currentState != lastState {
					bm.logger.Info("Circuit breaker state changed",
						"old_state", lastState.String(),
						"new_state", currentState.String(),
						"failure_count", failureCount,
						"success_count", successCount)
					lastState = currentState
				}
			}
		case <-ctx.Done():
			bm.logger.Debug("Circuit breaker monitoring stopped due to context cancellation")
			return
		case <-bm.shutdownChan:
			bm.logger.Debug("Circuit breaker monitoring stopped")
			return
		}
	}
}

// metricsRoutine collects and logs backpressure metrics
func (bm *BackpressureManager) metricsRoutine(ctx context.Context) {
	defer bm.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	bm.logger.Debug("Backpressure metrics routine started")

	for {
		select {
		case <-ticker.C:
			bm.logMetrics()
		case <-ctx.Done():
			bm.logger.Debug("Backpressure metrics routine stopped due to context cancellation")
			return
		case <-bm.shutdownChan:
			bm.logger.Debug("Backpressure metrics routine stopped")
			return
		}
	}
}

// logMetrics logs current backpressure metrics
func (bm *BackpressureManager) logMetrics() {
	metrics := bm.GetMetrics()

	bm.logger.Info("Backpressure metrics",
		"circuit_state", metrics.CircuitState.String(),
		"failure_count", metrics.FailureCount,
		"success_count", metrics.SuccessCount,
		"dropped_requests", metrics.DroppedRequests,
		"queue_utilization", fmt.Sprintf("%.2f%%", metrics.QueueUtilization*100),
		"load_shedding_active", metrics.LoadSheddingActive,
		"system_load", fmt.Sprintf("%.2f%%", metrics.SystemLoad*100))
}

// String methods for enums

func (cs CircuitState) String() string {
	switch cs {
	case CircuitStateClosed:
		return "Closed"
	case CircuitStateOpen:
		return "Open"
	case CircuitStateHalfOpen:
		return "HalfOpen"
	default:
		return "Unknown"
	}
}

func (dp DropPolicy) String() string {
	switch dp {
	case DropOldest:
		return "DropOldest"
	case DropNewest:
		return "DropNewest"
	case DropLowestPriority:
		return "DropLowestPriority"
	default:
		return "Unknown"
	}
}