package connection

import (
	"context"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dungeongate/internal/session/types"
	"github.com/google/uuid"
)

// Manager manages connections in a stateless manner
type Manager struct {
	maxConnections int
	logger         *slog.Logger

	// Atomic counters for stats
	activeConnections int64
	totalConnections  int64

	// Rate limiting
	connectionsByIP sync.Map // map[string]*ipConnectionTracker

	// Connection tracking for cleanup only
	connections sync.Map // map[string]*types.Connection
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

	// Close all connections
	m.connections.Range(func(key, value interface{}) bool {
		connID := key.(string)
		m.UnregisterConnection(connID)
		return true
	})

	return nil
}

// RegisterConnection registers a new connection and returns its ID
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

	// Create connection record
	connection := &types.Connection{
		ID:           connID,
		RemoteAddr:   conn.RemoteAddr(),
		State:        types.ConnectionStateConnected,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	// Store connection
	m.connections.Store(connID, connection)

	// Update counters
	atomic.AddInt64(&m.activeConnections, 1)
	atomic.AddInt64(&m.totalConnections, 1)

	m.logger.Info("Connection registered", "connection_id", connID, "remote_addr", conn.RemoteAddr())

	return connID
}

// UnregisterConnection removes a connection
func (m *Manager) UnregisterConnection(connID string) {
	if connID == "" {
		return
	}

	// Remove from storage
	if connInterface, exists := m.connections.LoadAndDelete(connID); exists {
		connection := connInterface.(*types.Connection)

		// Update IP tracker
		remoteIP := getIPFromAddr(connection.RemoteAddr)
		if tracker, exists := m.connectionsByIP.Load(remoteIP); exists {
			ipTracker := tracker.(*ipConnectionTracker)
			ipTracker.mu.Lock()
			if ipTracker.count > 0 {
				ipTracker.count--
			}
			ipTracker.mu.Unlock()
		}

		// Update counter
		atomic.AddInt64(&m.activeConnections, -1)

		m.logger.Info("Connection unregistered", "connection_id", connID)
	}
}

// UpdateConnectionState updates the state of a connection
func (m *Manager) UpdateConnectionState(connID string, state types.ConnectionState, userID string) {
	if connInterface, exists := m.connections.Load(connID); exists {
		connection := connInterface.(*types.Connection)
		connection.State = state
		connection.UserID = userID
		connection.LastActivity = time.Now()

		m.connections.Store(connID, connection)
	}
}

// GetConnection retrieves connection information
func (m *Manager) GetConnection(connID string) (*types.Connection, bool) {
	if connInterface, exists := m.connections.Load(connID); exists {
		connection := connInterface.(*types.Connection)
		return connection, true
	}
	return nil, false
}

// GetStats returns connection statistics
func (m *Manager) GetStats() *types.ConnectionStats {
	stats := &types.ConnectionStats{
		Active:     int(atomic.LoadInt64(&m.activeConnections)),
		Total:      int(atomic.LoadInt64(&m.totalConnections)),
		ByState:    make(map[types.ConnectionState]int),
		ByUserID:   make(map[string]int),
		ByRemoteIP: make(map[string]int),
	}

	// Count by state, user, and IP
	m.connections.Range(func(key, value interface{}) bool {
		connection := value.(*types.Connection)

		stats.ByState[connection.State]++

		if connection.UserID != "" {
			stats.ByUserID[connection.UserID]++
		}

		ip := getIPFromAddr(connection.RemoteAddr)
		stats.ByRemoteIP[ip]++

		return true
	})

	return stats
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

// cleanupLoop periodically cleans up expired connections
func (m *Manager) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
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

// cleanup removes expired connections and IP trackers
func (m *Manager) cleanup() {
	now := time.Now()
	expired := now.Add(-1 * time.Hour) // 1 hour timeout

	// Clean up expired connections
	m.connections.Range(func(key, value interface{}) bool {
		connection := value.(*types.Connection)
		if connection.LastActivity.Before(expired) {
			m.logger.Info("Cleaning up expired connection", "connection_id", connection.ID)
			m.UnregisterConnection(connection.ID)
		}
		return true
	})

	// Clean up IP trackers
	m.connectionsByIP.Range(func(key, value interface{}) bool {
		tracker := value.(*ipConnectionTracker)
		tracker.mu.RLock()
		shouldDelete := tracker.count == 0 && now.Sub(tracker.lastAttempt) > time.Hour
		tracker.mu.RUnlock()

		if shouldDelete {
			m.connectionsByIP.Delete(key)
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
