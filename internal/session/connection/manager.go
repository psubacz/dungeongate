package connection

import (
	"context"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// Manager manages connections in a stateless manner
type Manager struct {
	maxConnections int
	logger         *slog.Logger

	// Atomic counters for stats only
	activeConnections int64
	totalConnections  int64

	// Rate limiting only (no connection state storage)
	connectionsByIP sync.Map // map[string]*ipConnectionTracker

	// NOTE: No connection storage - truly stateless
	// All connection state is managed by Game Service
}

// ipConnectionTracker tracks connections per IP for rate limiting
type ipConnectionTracker struct {
	count       int64
	lastAttempt time.Time
	mu          sync.RWMutex
}

// NewManager creates a new connection manager
func NewManager(maxConnections int, logger *slog.Logger) *Manager {
	return &Manager{
		maxConnections: maxConnections,
		logger:         logger,
	}
}

// Start starts the connection manager
func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("Starting connection manager", "max_connections", m.maxConnections)

	// Start cleanup goroutine
	go m.cleanupLoop(ctx)

	return nil
}

// Stop stops the connection manager
func (m *Manager) Stop(ctx context.Context) error {
	m.logger.Info("Connection manager stopping")

	// In stateless mode, no local connections to close
	// Connection cleanup is handled by SSH server shutdown
	m.logger.Info("Connection manager stopped (stateless mode)")

	return nil
}

// RegisterConnection validates and registers a new connection, returns ID
// Connection state is NOT stored locally - only rate limiting and counters
func (m *Manager) RegisterConnection(conn net.Conn) string {
	connID := uuid.New().String()

	// Check connection limits
	currentConnections := atomic.LoadInt64(&m.activeConnections)
	if currentConnections >= int64(m.maxConnections) {
		m.logger.Warn("Max connections reached", "current", currentConnections, "max", m.maxConnections)
		conn.Close()
		return ""
	}

	// Rate limiting by IP
	remoteIP := getIPFromAddr(conn.RemoteAddr())
	if !m.checkRateLimit(remoteIP) {
		m.logger.Warn("Rate limit exceeded", "ip", remoteIP)
		conn.Close()
		return ""
	}

	// Update counters only - no connection state storage
	atomic.AddInt64(&m.activeConnections, 1)
	atomic.AddInt64(&m.totalConnections, 1)

	m.logger.Info("Connection registered (stateless)",
		"connection_id", connID,
		"remote_addr", conn.RemoteAddr(),
		"active_count", atomic.LoadInt64(&m.activeConnections))

	return connID
}

// UnregisterConnection decrements counters (stateless)
// Takes remoteAddr since we don't store connection state
func (m *Manager) UnregisterConnection(connID string, remoteAddr net.Addr) {
	if connID == "" {
		return
	}

	// Update IP tracker if we have the address
	if remoteAddr != nil {
		remoteIP := getIPFromAddr(remoteAddr)
		if tracker, exists := m.connectionsByIP.Load(remoteIP); exists {
			ipTracker := tracker.(*ipConnectionTracker)
			ipTracker.mu.Lock()
			if ipTracker.count > 0 {
				ipTracker.count--
			}
			ipTracker.mu.Unlock()
		}
	}

	// Update counter
	atomic.AddInt64(&m.activeConnections, -1)

	m.logger.Info("Connection unregistered (stateless)",
		"connection_id", connID,
		"active_count", atomic.LoadInt64(&m.activeConnections))
}

// ConnectionStats represents basic connection statistics for stateless mode
type ConnectionStats struct {
	Active int `json:"active"`
	Total  int `json:"total"`
}

// GetStats returns basic connection statistics (counters only)
// Detailed stats should be queried from Game Service
func (m *Manager) GetStats() *ConnectionStats {
	return &ConnectionStats{
		Active: int(atomic.LoadInt64(&m.activeConnections)),
		Total:  int(atomic.LoadInt64(&m.totalConnections)),
	}
}

// checkRateLimit checks if connection is allowed based on rate limiting
func (m *Manager) checkRateLimit(remoteIP string) bool {
	now := time.Now()

	// Load or create IP tracker
	trackerInterface, _ := m.connectionsByIP.LoadOrStore(remoteIP, &ipConnectionTracker{
		count:       0,
		lastAttempt: now,
	})

	tracker := trackerInterface.(*ipConnectionTracker)

	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	// Simple rate limiting: max 10 connections per IP
	if tracker.count >= 10 {
		return false
	}

	// Check if too many attempts in short time
	if now.Sub(tracker.lastAttempt) < time.Second && tracker.count > 5 {
		return false
	}

	tracker.count++
	tracker.lastAttempt = now

	return true
}

// cleanupLoop periodically cleans up rate limiting data only
func (m *Manager) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute) // Less frequent in stateless mode
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.cleanup()
		}
	}
}

// cleanup removes expired IP trackers only (stateless mode)
func (m *Manager) cleanup() {
	now := time.Now()

	// Only clean up IP trackers in stateless mode
	// Connection cleanup is handled by Game Service
	m.connectionsByIP.Range(func(key, value interface{}) bool {
		tracker := value.(*ipConnectionTracker)
		tracker.mu.RLock()
		shouldDelete := tracker.count == 0 && now.Sub(tracker.lastAttempt) > time.Hour
		tracker.mu.RUnlock()

		if shouldDelete {
			m.connectionsByIP.Delete(key)
			m.logger.Debug("Cleaned up IP tracker", "ip", key)
		}
		return true
	})
}

// getIPFromAddr extracts IP address from net.Addr
func getIPFromAddr(addr net.Addr) string {
	switch v := addr.(type) {
	case *net.TCPAddr:
		return v.IP.String()
	case *net.UDPAddr:
		return v.IP.String()
	default:
		// For other types, extract IP from string representation
		addrStr := addr.String()
		if strings.Contains(addrStr, ":") {
			host, _, _ := net.SplitHostPort(addrStr)
			return host
		}
		return addrStr
	}
}
