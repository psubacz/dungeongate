package pools

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/creack/pty"
)

// PTYResource represents a managed PTY resource
type PTYResource struct {
	ID          string
	FD          int
	Master      *os.File
	Slave       *os.File
	CreatedAt   time.Time
	LastUsed    time.Time
	InUse       bool
	SessionID   string
	ProcessPID  int
}

// PTYMetrics tracks PTY pool performance
type PTYMetrics struct {
	ActivePTYs    int64
	AvailablePTYs int64
	TotalPTYs     int64
	ReusedPTYs    int64
	FailedPTYs    int64
	CurrentFDs    int64
	MaxFDs        int64
}

// PTYPool manages a pool of PTY resources
type PTYPool struct {
	maxPTYs         int
	fdLimit         int
	reuseTimeout    time.Duration
	cleanupInterval time.Duration
	
	activePTYs      map[string]*PTYResource
	availablePTYs   chan *PTYResource
	currentFDs      int64
	
	metrics         *PTYMetrics
	logger          *slog.Logger
	
	shutdownChan    chan struct{}
	
	mu              sync.RWMutex
	wg              sync.WaitGroup
}

// PTYConfig holds configuration for the PTY pool
type PTYConfig struct {
	MaxPTYs         int
	ReuseTimeout    time.Duration
	CleanupInterval time.Duration
	FDLimit         int
}

// DefaultPTYConfig returns a default PTY configuration
func DefaultPTYConfig() *PTYConfig {
	return &PTYConfig{
		MaxPTYs:         500,
		ReuseTimeout:    5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
		FDLimit:         1024,
	}
}

// NewPTYPool creates a new PTY pool
func NewPTYPool(config *PTYConfig, logger *slog.Logger) (*PTYPool, error) {
	if config == nil {
		config = DefaultPTYConfig()
	}

	pp := &PTYPool{
		maxPTYs:         config.MaxPTYs,
		fdLimit:         config.FDLimit,
		reuseTimeout:    config.ReuseTimeout,
		cleanupInterval: config.CleanupInterval,
		activePTYs:      make(map[string]*PTYResource),
		availablePTYs:   make(chan *PTYResource, config.MaxPTYs),
		metrics: &PTYMetrics{
			MaxFDs: int64(config.FDLimit),
		},
		logger:       logger,
		shutdownChan: make(chan struct{}),
	}

	logger.Info("Created PTY pool", 
		"max_ptys", config.MaxPTYs,
		"fd_limit", config.FDLimit,
		"reuse_timeout", config.ReuseTimeout)

	return pp, nil
}

// Start starts the PTY pool
func (pp *PTYPool) Start(ctx context.Context) error {
	pp.logger.Info("Starting PTY pool")

	// Start cleanup routine
	pp.wg.Add(1)
	go pp.cleanupRoutine(ctx)

	// Start metrics collection
	pp.wg.Add(1)
	go pp.metricsRoutine(ctx)

	return nil
}

// Stop stops the PTY pool gracefully
func (pp *PTYPool) Stop(ctx context.Context) error {
	pp.logger.Info("Stopping PTY pool")

	// Signal shutdown
	close(pp.shutdownChan)

	// Wait for goroutines to finish
	done := make(chan struct{})
	go func() {
		pp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		pp.logger.Info("PTY pool stopped gracefully")
	case <-ctx.Done():
		pp.logger.Warn("PTY pool stop timeout exceeded")
	}

	// Clean up all PTYs
	pp.cleanupAllPTYs()

	return nil
}

// AcquirePTY acquires a PTY resource from the pool
func (pp *PTYPool) AcquirePTY(sessionID string) (*PTYResource, error) {
	pp.logger.Debug("Acquiring PTY", "session_id", sessionID)

	// Check FD limit
	if atomic.LoadInt64(&pp.currentFDs) >= int64(pp.fdLimit) {
		pp.logger.Warn("PTY acquisition failed: FD limit reached", 
			"current_fds", pp.currentFDs,
			"fd_limit", pp.fdLimit,
			"session_id", sessionID)
		return nil, fmt.Errorf("file descriptor limit reached")
	}

	// Try to get an available PTY first
	select {
	case ptyRes := <-pp.availablePTYs:
		if pp.isValidForReuse(ptyRes) {
			pp.logger.Debug("Reusing PTY", 
				"pty_id", ptyRes.ID,
				"session_id", sessionID,
				"age", time.Since(ptyRes.CreatedAt))
			
			ptyRes.SessionID = sessionID
			ptyRes.LastUsed = time.Now()
			ptyRes.InUse = true
			
			pp.mu.Lock()
			pp.activePTYs[ptyRes.ID] = ptyRes
			pp.mu.Unlock()
			
			atomic.AddInt64(&pp.metrics.ReusedPTYs, 1)
			return ptyRes, nil
		} else {
			// PTY is too old, clean it up and create a new one
			pp.logger.Debug("PTY too old for reuse, cleaning up", 
				"pty_id", ptyRes.ID,
				"age", time.Since(ptyRes.CreatedAt))
			pp.cleanupPTY(ptyRes)
		}
	default:
		// No available PTY, will create new one
		pp.logger.Debug("No available PTY for reuse, creating new one", "session_id", sessionID)
	}

	// Create new PTY
	return pp.createNewPTY(sessionID)
}

// ReleasePTY releases a PTY resource back to the pool
func (pp *PTYPool) ReleasePTY(ptyID string) error {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	ptyRes, exists := pp.activePTYs[ptyID]
	if !exists {
		pp.logger.Warn("Attempted to release non-existent PTY", "pty_id", ptyID)
		return fmt.Errorf("PTY not found: %s", ptyID)
	}

	pp.logger.Debug("Releasing PTY", 
		"pty_id", ptyID,
		"session_id", ptyRes.SessionID)

	// Mark as not in use
	ptyRes.InUse = false
	ptyRes.SessionID = ""
	ptyRes.LastUsed = time.Now()

	// Remove from active PTYs
	delete(pp.activePTYs, ptyID)

	// Try to add to available pool for reuse
	select {
	case pp.availablePTYs <- ptyRes:
		pp.logger.Debug("PTY returned to available pool", "pty_id", ptyID)
	default:
		// Available pool is full, clean up the PTY
		pp.logger.Debug("Available pool full, cleaning up PTY", "pty_id", ptyID)
		pp.cleanupPTY(ptyRes)
	}

	return nil
}

// GetMetrics returns current PTY pool metrics
func (pp *PTYPool) GetMetrics() PTYMetrics {
	pp.mu.RLock()
	defer pp.mu.RUnlock()

	metrics := *pp.metrics
	metrics.ActivePTYs = int64(len(pp.activePTYs))
	metrics.AvailablePTYs = int64(len(pp.availablePTYs))
	metrics.CurrentFDs = atomic.LoadInt64(&pp.currentFDs)

	return metrics
}

// createNewPTY creates a new PTY resource
func (pp *PTYPool) createNewPTY(sessionID string) (*PTYResource, error) {
	pp.logger.Debug("Creating new PTY", "session_id", sessionID)

	// Create PTY
	master, slave, err := pty.Open()
	if err != nil {
		pp.logger.Error("Failed to create PTY", "error", err, "session_id", sessionID)
		atomic.AddInt64(&pp.metrics.FailedPTYs, 1)
		return nil, fmt.Errorf("failed to create PTY: %w", err)
	}

	// Get file descriptors
	masterFD := int(master.Fd())
	slaveFD := int(slave.Fd())

	ptyID := pp.generatePTYID()
	ptyRes := &PTYResource{
		ID:         ptyID,
		FD:         masterFD,
		Master:     master,
		Slave:      slave,
		CreatedAt:  time.Now(),
		LastUsed:   time.Now(),
		InUse:      true,
		SessionID:  sessionID,
	}

	pp.mu.Lock()
	pp.activePTYs[ptyID] = ptyRes
	pp.mu.Unlock()

	// Update FD count
	atomic.AddInt64(&pp.currentFDs, 2) // master + slave
	atomic.AddInt64(&pp.metrics.TotalPTYs, 1)

	pp.logger.Info("Created new PTY", 
		"pty_id", ptyID,
		"session_id", sessionID,
		"master_fd", masterFD,
		"slave_fd", slaveFD,
		"current_fds", atomic.LoadInt64(&pp.currentFDs))

	return ptyRes, nil
}

// cleanupPTY cleans up a PTY resource
func (pp *PTYPool) cleanupPTY(ptyRes *PTYResource) {
	pp.logger.Debug("Cleaning up PTY", 
		"pty_id", ptyRes.ID,
		"session_id", ptyRes.SessionID)

	// Close master
	if ptyRes.Master != nil {
		if err := ptyRes.Master.Close(); err != nil {
			pp.logger.Error("Failed to close PTY master", 
				"pty_id", ptyRes.ID,
				"error", err)
		}
	}

	// Close slave
	if ptyRes.Slave != nil {
		if err := ptyRes.Slave.Close(); err != nil {
			pp.logger.Error("Failed to close PTY slave", 
				"pty_id", ptyRes.ID,
				"error", err)
		}
	}

	// Update FD count
	atomic.AddInt64(&pp.currentFDs, -2) // master + slave

	pp.logger.Debug("PTY cleaned up", 
		"pty_id", ptyRes.ID,
		"current_fds", atomic.LoadInt64(&pp.currentFDs))
}

// cleanupAllPTYs cleans up all PTY resources
func (pp *PTYPool) cleanupAllPTYs() {
	pp.logger.Info("Cleaning up all PTYs")

	pp.mu.Lock()
	defer pp.mu.Unlock()

	// Clean up active PTYs
	for _, ptyRes := range pp.activePTYs {
		pp.cleanupPTY(ptyRes)
	}
	pp.activePTYs = make(map[string]*PTYResource)

	// Clean up available PTYs
	for {
		select {
		case ptyRes := <-pp.availablePTYs:
			pp.cleanupPTY(ptyRes)
		default:
			goto done
		}
	}
done:

	pp.logger.Info("All PTYs cleaned up", "final_fd_count", atomic.LoadInt64(&pp.currentFDs))
}

// isValidForReuse checks if a PTY is valid for reuse
func (pp *PTYPool) isValidForReuse(ptyRes *PTYResource) bool {
	// Check if PTY is too old
	if time.Since(ptyRes.LastUsed) > pp.reuseTimeout {
		return false
	}

	// Check if master and slave are still valid
	if ptyRes.Master == nil || ptyRes.Slave == nil {
		return false
	}

	return true
}

// cleanupRoutine performs periodic cleanup of old PTYs
func (pp *PTYPool) cleanupRoutine(ctx context.Context) {
	defer pp.wg.Done()

	ticker := time.NewTicker(pp.cleanupInterval)
	defer ticker.Stop()

	pp.logger.Debug("PTY cleanup routine started", "interval", pp.cleanupInterval)

	for {
		select {
		case <-ticker.C:
			pp.performCleanup()
		case <-ctx.Done():
			pp.logger.Debug("PTY cleanup routine stopped due to context cancellation")
			return
		case <-pp.shutdownChan:
			pp.logger.Debug("PTY cleanup routine stopped")
			return
		}
	}
}

// performCleanup performs cleanup of old available PTYs
func (pp *PTYPool) performCleanup() {
	var cleaned int
	var toCleanup []*PTYResource

	// Collect PTYs that need cleanup from available pool
	for {
		select {
		case ptyRes := <-pp.availablePTYs:
			if !pp.isValidForReuse(ptyRes) {
				toCleanup = append(toCleanup, ptyRes)
			} else {
				// Put it back if it's still valid
				select {
				case pp.availablePTYs <- ptyRes:
				default:
					// Pool is full, clean it up anyway
					toCleanup = append(toCleanup, ptyRes)
				}
			}
		default:
			goto cleanup
		}
	}

cleanup:
	// Clean up old PTYs
	for _, ptyRes := range toCleanup {
		pp.cleanupPTY(ptyRes)
		cleaned++
	}

	if cleaned > 0 {
		pp.logger.Debug("Cleaned up old PTYs", 
			"cleaned_count", cleaned,
			"current_fds", atomic.LoadInt64(&pp.currentFDs))
	}
}

// metricsRoutine collects PTY pool metrics
func (pp *PTYPool) metricsRoutine(ctx context.Context) {
	defer pp.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	pp.logger.Debug("PTY metrics routine started")

	for {
		select {
		case <-ticker.C:
			pp.logMetrics()
		case <-ctx.Done():
			pp.logger.Debug("PTY metrics routine stopped due to context cancellation")
			return
		case <-pp.shutdownChan:
			pp.logger.Debug("PTY metrics routine stopped")
			return
		}
	}
}

// logMetrics logs current PTY pool metrics
func (pp *PTYPool) logMetrics() {
	metrics := pp.GetMetrics()
	
	pp.logger.Debug("PTY pool metrics",
		"active_ptys", metrics.ActivePTYs,
		"available_ptys", metrics.AvailablePTYs,
		"total_ptys", metrics.TotalPTYs,
		"reused_ptys", metrics.ReusedPTYs,
		"failed_ptys", metrics.FailedPTYs,
		"current_fds", metrics.CurrentFDs,
		"max_fds", metrics.MaxFDs,
		"fd_utilization", float64(metrics.CurrentFDs)/float64(metrics.MaxFDs)*100)
}

// generatePTYID generates a unique PTY ID
func (pp *PTYPool) generatePTYID() string {
	return fmt.Sprintf("pty_%d_%d", time.Now().UnixNano(), len(pp.activePTYs))
}