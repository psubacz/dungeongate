package pools

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// Priority defines the priority level for connection requests
type Priority int

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

// ConnectionState represents the state of a connection
type ConnectionState int

const (
	ConnectionStatePending ConnectionState = iota
	ConnectionStateActive
	ConnectionStateIdle
	ConnectionStateClosing
	ConnectionStateClosed
)

// Connection represents a managed connection
type Connection struct {
	ID            string
	SSHChannel    ssh.Channel
	SSHConn       *ssh.ServerConn
	UserID        string
	Username      string
	CreatedAt     time.Time
	LastActivity  time.Time
	State         ConnectionState
	ResourceQuota *ResourceQuota
	Context       context.Context
	Cancel        context.CancelFunc
}

// ResourceQuota defines resource limits for a connection
type ResourceQuota struct {
	MaxMemory    int64 // bytes
	MaxBandwidth int64 // bytes/sec
	MaxPTYs      int
	ExpiresAt    time.Time
}

// ConnectionRequest represents a request for a new connection
type ConnectionRequest struct {
	SSHChannel   ssh.Channel
	SSHConn      *ssh.ServerConn
	ResponseChan chan *ConnectionResponse
	Priority     Priority
	QueuedAt     time.Time
	Context      context.Context
}

// ConnectionResponse represents the response to a connection request
type ConnectionResponse struct {
	Connection *Connection
	Error      error
}

// ConnectionMetrics tracks connection pool metrics
type ConnectionMetrics struct {
	ActiveConnections   int64
	QueuedRequests      int64
	TotalConnections    int64
	RejectedConnections int64
	AverageQueueTime    time.Duration
}

// ConnectionPool manages a pool of connections with resource limits
type ConnectionPool struct {
	maxConnections    int
	queueSize         int
	queueTimeout      time.Duration
	idleTimeout       time.Duration
	drainTimeout      time.Duration
	
	activeConnections map[string]*Connection
	connectionQueue   chan *ConnectionRequest
	workerPool        *WorkerPool
	ptyPool          *PTYPool
	backpressure     *BackpressureManager
	
	metrics          *ConnectionMetrics
	logger           *slog.Logger
	
	shutdownChan     chan struct{}
	draining         bool
	
	mu               sync.RWMutex
	wg               sync.WaitGroup
}

// Config holds configuration for the connection pool
type Config struct {
	MaxConnections int
	QueueSize      int
	QueueTimeout   time.Duration
	IdleTimeout    time.Duration
	DrainTimeout   time.Duration
	WorkerPoolSize int
	MaxPTYs        int
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		MaxConnections: 1000,
		QueueSize:      100,
		QueueTimeout:   5 * time.Second,
		IdleTimeout:    30 * time.Minute,
		DrainTimeout:   30 * time.Second,
		WorkerPoolSize: 50,
		MaxPTYs:        500,
	}
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(config *Config, logger *slog.Logger) (*ConnectionPool, error) {
	if config == nil {
		config = DefaultConfig()
	}

	cp := &ConnectionPool{
		maxConnections:    config.MaxConnections,
		queueSize:         config.QueueSize,
		queueTimeout:      config.QueueTimeout,
		idleTimeout:       config.IdleTimeout,
		drainTimeout:      config.DrainTimeout,
		activeConnections: make(map[string]*Connection),
		connectionQueue:   make(chan *ConnectionRequest, config.QueueSize),
		metrics:          &ConnectionMetrics{},
		logger:           logger,
		shutdownChan:     make(chan struct{}),
	}

	// Create worker pool
	workerConfig := &WorkerConfig{
		PoolSize:        config.WorkerPoolSize,
		QueueSize:       1000,
		WorkerTimeout:   30 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}
	var err error
	cp.workerPool, err = NewWorkerPool(workerConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create worker pool: %w", err)
	}

	// Create PTY pool
	ptyConfig := &PTYConfig{
		MaxPTYs:         config.MaxPTYs,
		ReuseTimeout:    5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
		FDLimit:         1024,
	}
	cp.ptyPool, err = NewPTYPool(ptyConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create PTY pool: %w", err)
	}

	// Create backpressure manager
	backpressureConfig := &BackpressureConfig{
		Enabled:          true,
		CircuitBreaker:   true,
		LoadShedding:     true,
		FailureThreshold: 10,
		RecoveryTimeout:  60 * time.Second,
	}
	cp.backpressure, err = NewBackpressureManager(backpressureConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create backpressure manager: %w", err)
	}

	return cp, nil
}

// Start starts the connection pool
func (cp *ConnectionPool) Start(ctx context.Context) error {
	cp.logger.Info("Starting connection pool", "max_connections", cp.maxConnections)

	// Start worker pool
	if err := cp.workerPool.Start(ctx); err != nil {
		return fmt.Errorf("failed to start worker pool: %w", err)
	}

	// Start PTY pool
	if err := cp.ptyPool.Start(ctx); err != nil {
		return fmt.Errorf("failed to start PTY pool: %w", err)
	}

	// Start backpressure manager
	if err := cp.backpressure.Start(ctx); err != nil {
		return fmt.Errorf("failed to start backpressure manager: %w", err)
	}

	// Start connection processor
	cp.wg.Add(1)
	go cp.processConnections(ctx)

	// Start cleanup routine
	cp.wg.Add(1)
	go cp.cleanupRoutine(ctx)

	return nil
}

// Stop stops the connection pool gracefully
func (cp *ConnectionPool) Stop(ctx context.Context) error {
	cp.logger.Info("Stopping connection pool")
	
	cp.mu.Lock()
	cp.draining = true
	cp.mu.Unlock()

	// Close shutdown channel
	close(cp.shutdownChan)

	// Stop accepting new connections
	close(cp.connectionQueue)

	// Wait for drain timeout or context cancellation
	drainCtx, cancel := context.WithTimeout(ctx, cp.drainTimeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		cp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		cp.logger.Info("Connection pool stopped gracefully")
	case <-drainCtx.Done():
		cp.logger.Warn("Connection pool drain timeout exceeded")
	}

	// Stop subsystems
	if err := cp.workerPool.Stop(ctx); err != nil {
		cp.logger.Error("Failed to stop worker pool", "error", err)
	}
	if err := cp.ptyPool.Stop(ctx); err != nil {
		cp.logger.Error("Failed to stop PTY pool", "error", err)
	}
	if err := cp.backpressure.Stop(ctx); err != nil {
		cp.logger.Error("Failed to stop backpressure manager", "error", err)
	}

	return nil
}

// RequestConnection requests a new connection from the pool
func (cp *ConnectionPool) RequestConnection(ctx context.Context, channel ssh.Channel, conn *ssh.ServerConn, priority Priority) (*Connection, error) {
	cp.mu.RLock()
	if cp.draining {
		cp.mu.RUnlock()
		return nil, fmt.Errorf("connection pool is shutting down")
	}
	cp.mu.RUnlock()

	// Check if we can accept the connection
	if !cp.backpressure.CanAccept() {
		cp.metrics.RejectedConnections++
		return nil, fmt.Errorf("server overloaded")
	}

	req := &ConnectionRequest{
		SSHChannel:   channel,
		SSHConn:      conn,
		ResponseChan: make(chan *ConnectionResponse, 1),
		Priority:     priority,
		QueuedAt:     time.Now(),
		Context:      ctx,
	}

	// Try to queue the request
	select {
	case cp.connectionQueue <- req:
		cp.metrics.QueuedRequests++
	case <-time.After(cp.queueTimeout):
		cp.metrics.RejectedConnections++
		return nil, fmt.Errorf("request queue timeout")
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Wait for response
	select {
	case resp := <-req.ResponseChan:
		queueTime := time.Since(req.QueuedAt)
		cp.updateAverageQueueTime(queueTime)
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Connection, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ReleaseConnection releases a connection back to the pool
func (cp *ConnectionPool) ReleaseConnection(connID string) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	conn, exists := cp.activeConnections[connID]
	if !exists {
		return
	}

	// Cancel the connection context
	if conn.Cancel != nil {
		conn.Cancel()
	}

	// Update state
	conn.State = ConnectionStateClosed

	// Remove from active connections
	delete(cp.activeConnections, connID)
	cp.metrics.ActiveConnections--

	cp.logger.Debug("Released connection", "connection_id", connID, "active_connections", len(cp.activeConnections))
}

// GetConnection retrieves a connection by ID
func (cp *ConnectionPool) GetConnection(connID string) (*Connection, bool) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	
	conn, exists := cp.activeConnections[connID]
	return conn, exists
}

// GetMetrics returns current pool metrics
func (cp *ConnectionPool) GetMetrics() ConnectionMetrics {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	
	metrics := *cp.metrics
	metrics.ActiveConnections = int64(len(cp.activeConnections))
	return metrics
}

// RequestQueue returns the request queue channel for external monitoring
func (cp *ConnectionPool) RequestQueue() chan<- *ConnectionRequest {
	return cp.connectionQueue
}

// QueueTimeout returns the configured queue timeout
func (cp *ConnectionPool) QueueTimeout() time.Duration {
	return cp.queueTimeout
}

// processConnections processes incoming connection requests
func (cp *ConnectionPool) processConnections(ctx context.Context) {
	defer cp.wg.Done()

	for {
		select {
		case req, ok := <-cp.connectionQueue:
			if !ok {
				return // Channel closed, shutting down
			}
			cp.handleConnectionRequest(ctx, req)
		case <-ctx.Done():
			return
		case <-cp.shutdownChan:
			return
		}
	}
}

// handleConnectionRequest handles a single connection request
func (cp *ConnectionPool) handleConnectionRequest(ctx context.Context, req *ConnectionRequest) {
	cp.mu.Lock()
	
	// Check if we're at capacity
	if len(cp.activeConnections) >= cp.maxConnections {
		cp.mu.Unlock()
		req.ResponseChan <- &ConnectionResponse{
			Error: fmt.Errorf("connection pool at capacity"),
		}
		cp.metrics.RejectedConnections++
		return
	}

	// Create new connection
	connCtx, cancel := context.WithCancel(ctx)
	connID := cp.generateConnectionID()
	
	// Get username safely
	var username string
	if req.SSHConn != nil {
		username = req.SSHConn.User()
	} else {
		username = "test-user" // Default for testing
	}

	conn := &Connection{
		ID:           connID,
		SSHChannel:   req.SSHChannel,
		SSHConn:      req.SSHConn,
		Username:     username,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		State:        ConnectionStateActive,
		Context:      connCtx,
		Cancel:       cancel,
		ResourceQuota: &ResourceQuota{
			MaxMemory:    256 * 1024 * 1024, // 256MB default
			MaxBandwidth: 10 * 1024 * 1024,  // 10MB/s default
			MaxPTYs:      5,                  // 5 PTYs default
			ExpiresAt:    time.Now().Add(24 * time.Hour),
		},
	}

	// Add to active connections
	cp.activeConnections[connID] = conn
	cp.metrics.ActiveConnections++
	cp.metrics.TotalConnections++
	cp.metrics.QueuedRequests--

	cp.mu.Unlock()

	cp.logger.Info("Accepted new connection", 
		"connection_id", connID, 
		"username", conn.Username,
		"active_connections", len(cp.activeConnections))

	// Send response
	req.ResponseChan <- &ConnectionResponse{
		Connection: conn,
	}
}

// cleanupRoutine performs periodic cleanup of idle connections
func (cp *ConnectionPool) cleanupRoutine(ctx context.Context) {
	defer cp.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cp.cleanupIdleConnections()
		case <-ctx.Done():
			return
		case <-cp.shutdownChan:
			return
		}
	}
}

// cleanupIdleConnections removes connections that have been idle too long
func (cp *ConnectionPool) cleanupIdleConnections() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	now := time.Now()
	var toRemove []string

	for connID, conn := range cp.activeConnections {
		if now.Sub(conn.LastActivity) > cp.idleTimeout {
			toRemove = append(toRemove, connID)
		}
	}

	for _, connID := range toRemove {
		conn := cp.activeConnections[connID]
		if conn.Cancel != nil {
			conn.Cancel()
		}
		delete(cp.activeConnections, connID)
		cp.metrics.ActiveConnections--
		
		cp.logger.Info("Cleaned up idle connection", "connection_id", connID)
	}
}

// generateConnectionID generates a unique connection ID
func (cp *ConnectionPool) generateConnectionID() string {
	return fmt.Sprintf("conn_%d_%d", time.Now().UnixNano(), len(cp.activeConnections))
}

// updateAverageQueueTime updates the average queue time metric
func (cp *ConnectionPool) updateAverageQueueTime(queueTime time.Duration) {
	// Simple moving average - in production, use a more sophisticated approach
	if cp.metrics.AverageQueueTime == 0 {
		cp.metrics.AverageQueueTime = queueTime
	} else {
		cp.metrics.AverageQueueTime = (cp.metrics.AverageQueueTime + queueTime) / 2
	}
}