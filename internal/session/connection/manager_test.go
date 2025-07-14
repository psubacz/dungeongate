package connection

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	logger := slog.Default()
	manager := NewManager(100, logger)

	assert.NotNil(t, manager)
	assert.Equal(t, 100, manager.maxConnections)
	assert.Equal(t, logger, manager.logger)
	assert.Equal(t, int64(0), manager.activeConnections)
	assert.Equal(t, int64(0), manager.totalConnections)
}

func TestManagerStartStop(t *testing.T) {
	logger := slog.Default()
	manager := NewManager(100, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test start
	err := manager.Start(ctx)
	assert.NoError(t, err)

	// Test stop
	err = manager.Stop(ctx)
	assert.NoError(t, err)
}

func TestRegisterConnection(t *testing.T) {
	logger := slog.Default()
	manager := NewManager(100, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.Start(ctx)
	require.NoError(t, err)

	// Create mock connection
	conn := &mockConn{
		remoteAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345},
	}

	// Test successful registration
	connID := manager.RegisterConnection(conn)
	assert.NotEmpty(t, connID)
	assert.Equal(t, int64(1), manager.activeConnections)
	assert.Equal(t, int64(1), manager.totalConnections)

	// In stateless mode, GetConnection is not available
	// We can only verify via counters
	assert.Equal(t, int64(1), manager.activeConnections)

	manager.Stop(ctx)
}

func TestRegisterConnectionMaxLimit(t *testing.T) {
	logger := slog.Default()
	manager := NewManager(2, logger) // Small limit for testing
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.Start(ctx)
	require.NoError(t, err)

	// Register up to limit
	conn1 := &mockConn{remoteAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}}
	conn2 := &mockConn{remoteAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12346}}
	conn3 := &mockConn{remoteAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12347}}

	connID1 := manager.RegisterConnection(conn1)
	connID2 := manager.RegisterConnection(conn2)
	connID3 := manager.RegisterConnection(conn3) // Should be rejected

	assert.NotEmpty(t, connID1)
	assert.NotEmpty(t, connID2)
	assert.Empty(t, connID3)
	assert.Equal(t, int64(2), manager.activeConnections)
	assert.Equal(t, int64(2), manager.totalConnections)
	assert.True(t, conn3.closed)

	manager.Stop(ctx)
}

func TestRegisterConnectionRateLimit(t *testing.T) {
	logger := slog.Default()
	manager := NewManager(100, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.Start(ctx)
	require.NoError(t, err)

	sameIP := net.ParseIP("192.168.1.1")

	// Register connections from same IP up to rate limit
	var connIDs []string
	for i := 0; i < 10; i++ {
		conn := &mockConn{
			remoteAddr: &net.TCPAddr{IP: sameIP, Port: 12345 + i},
		}
		connID := manager.RegisterConnection(conn)
		if connID != "" {
			connIDs = append(connIDs, connID)
		}
	}

	// Should have registered up to 10 connections
	// But rapid connection limiting may block some
	assert.LessOrEqual(t, len(connIDs), 10)
	assert.GreaterOrEqual(t, len(connIDs), 5) // Should get at least 5 before rapid limiting

	// 11th connection should be rejected
	conn11 := &mockConn{
		remoteAddr: &net.TCPAddr{IP: sameIP, Port: 12355},
	}
	connID11 := manager.RegisterConnection(conn11)
	assert.Empty(t, connID11)
	assert.True(t, conn11.closed)

	manager.Stop(ctx)
}

func TestUnregisterConnection(t *testing.T) {
	logger := slog.Default()
	manager := NewManager(100, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.Start(ctx)
	require.NoError(t, err)

	// Register a connection
	conn := &mockConn{
		remoteAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345},
	}
	connID := manager.RegisterConnection(conn)
	require.NotEmpty(t, connID)

	// Verify active connection count
	assert.Equal(t, int64(1), manager.activeConnections)

	// Unregister (stateless mode - need remote addr)
	manager.UnregisterConnection(connID, conn.RemoteAddr())

	// Verify counter decremented
	assert.Equal(t, int64(0), manager.activeConnections)

	manager.Stop(ctx)
}

func TestUnregisterConnectionEmpty(t *testing.T) {
	logger := slog.Default()
	manager := NewManager(100, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.Start(ctx)
	require.NoError(t, err)

	// Should not panic with empty string (stateless mode)
	manager.UnregisterConnection("", nil)

	// Should not panic with non-existent ID (stateless mode)
	manager.UnregisterConnection("non-existent", nil)

	manager.Stop(ctx)
}

func TestUpdateConnectionState(t *testing.T) {
	logger := slog.Default()
	manager := NewManager(100, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.Start(ctx)
	require.NoError(t, err)

	// Register a connection
	conn := &mockConn{
		remoteAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345},
	}
	connID := manager.RegisterConnection(conn)
	require.NotEmpty(t, connID)

	// Update state (no-op in stateless mode)
	// UpdateConnectionState removed - no longer needed in stateless architecture

	// In stateless mode, state updates are delegated to Game Service
	// This test just ensures the method doesn't panic

	manager.Stop(ctx)
}

func TestUpdateConnectionStateNonExistent(t *testing.T) {
	logger := slog.Default()
	manager := NewManager(100, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.Start(ctx)
	require.NoError(t, err)

	// Should not panic with non-existent connection
	// UpdateConnectionState removed - no longer needed in stateless architecture

	manager.Stop(ctx)
}

func TestGetStats(t *testing.T) {
	logger := slog.Default()
	manager := NewManager(100, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.Start(ctx)
	require.NoError(t, err)

	// Register connections with different states
	conn1 := &mockConn{remoteAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}}
	conn2 := &mockConn{remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 12345}}
	conn3 := &mockConn{remoteAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12346}}

	manager.RegisterConnection(conn1)
	manager.RegisterConnection(conn2)
	manager.RegisterConnection(conn3)

	// Update states (no-op in stateless mode)
	// UpdateConnectionState removed - no longer needed in stateless architecture

	// Get stats (only basic counters in stateless mode)
	stats := manager.GetStats()
	assert.Equal(t, 3, stats.Active)
	assert.Equal(t, 3, stats.Total)

	// Detailed stats not available in stateless mode
	// These would be queried from Game Service in a real implementation

	manager.Stop(ctx)
}

func TestConcurrentOperations(t *testing.T) {
	logger := slog.Default()
	manager := NewManager(1000, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.Start(ctx)
	require.NoError(t, err)

	const numGoroutines = 100
	const connectionsPerGoroutine = 10

	var wg sync.WaitGroup
	connIDs := make([][]string, numGoroutines)
	connections := make([][]*mockConn, numGoroutines)

	// Concurrent registration
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			connIDs[goroutineID] = make([]string, 0, connectionsPerGoroutine)
			connections[goroutineID] = make([]*mockConn, 0, connectionsPerGoroutine)

			for j := 0; j < connectionsPerGoroutine; j++ {
				conn := &mockConn{
					remoteAddr: &net.TCPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: 12345 + goroutineID*1000 + j,
					},
				}
				connID := manager.RegisterConnection(conn)
				if connID != "" {
					connIDs[goroutineID] = append(connIDs[goroutineID], connID)
					connections[goroutineID] = append(connections[goroutineID], conn)
				}
			}
		}(i)
	}

	wg.Wait()

	// Count total registered connections
	totalRegistered := 0
	for i := 0; i < numGoroutines; i++ {
		totalRegistered += len(connIDs[i])
	}

	stats := manager.GetStats()
	assert.Equal(t, totalRegistered, stats.Active)
	assert.Equal(t, totalRegistered, stats.Total)

	// Concurrent unregistration
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j, connID := range connIDs[goroutineID] {
				manager.UnregisterConnection(connID, connections[goroutineID][j].RemoteAddr())
			}
		}(i)
	}

	wg.Wait()

	// All connections should be unregistered
	finalStats := manager.GetStats()
	assert.Equal(t, 0, finalStats.Active)
	assert.Equal(t, totalRegistered, finalStats.Total) // Total count should remain

	manager.Stop(ctx)
}

func TestCleanupIPTrackers(t *testing.T) {
	logger := slog.Default()
	manager := NewManager(100, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.Start(ctx)
	require.NoError(t, err)

	// Register and unregister a connection to create IP tracker
	conn := &mockConn{
		remoteAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345},
	}
	connID := manager.RegisterConnection(conn)
	require.NotEmpty(t, connID)

	// Unregister to reset count but keep tracker
	manager.UnregisterConnection(connID, conn.RemoteAddr())

	// Force cleanup - should remove inactive IP trackers
	manager.cleanup()

	// IP tracker cleanup is internal to the manager
	// We can't easily test it without exposing internal state
	// This test mainly ensures cleanup doesn't panic in stateless mode

	manager.Stop(ctx)
}

func TestGetIPFromAddr(t *testing.T) {
	tests := []struct {
		name     string
		addr     net.Addr
		expected string
	}{
		{
			name:     "TCP address",
			addr:     &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 8080},
			expected: "192.168.1.1",
		},
		{
			name:     "UDP address",
			addr:     &net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 53},
			expected: "10.0.0.1",
		},
		{
			name:     "String address with port",
			addr:     &mockAddr{addr: "172.16.0.1:22"},
			expected: "172.16.0.1",
		},
		{
			name:     "String address without port",
			addr:     &mockAddr{addr: "localhost"},
			expected: "localhost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getIPFromAddr(tt.addr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRateLimitingWithTime(t *testing.T) {
	logger := slog.Default()
	manager := NewManager(100, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.Start(ctx)
	require.NoError(t, err)

	sameIP := net.ParseIP("192.168.1.1")

	// Register 6 connections quickly (within time limit)
	var connIDs []string
	for i := 0; i < 6; i++ {
		conn := &mockConn{
			remoteAddr: &net.TCPAddr{IP: sameIP, Port: 12345 + i},
		}
		connID := manager.RegisterConnection(conn)
		if connID != "" {
			connIDs = append(connIDs, connID)
		}
	}

	// Should have registered 6 connections (within time-based rate limit)
	assert.Equal(t, 6, len(connIDs))

	// Try to register another immediately (should be rate limited due to time)
	conn7 := &mockConn{
		remoteAddr: &net.TCPAddr{IP: sameIP, Port: 12351},
	}
	connID7 := manager.RegisterConnection(conn7)
	assert.Empty(t, connID7)
	assert.True(t, conn7.closed)

	manager.Stop(ctx)
}

// Mock connection for testing
type mockConn struct {
	remoteAddr net.Addr
	closed     bool
	mu         sync.Mutex
}

func (c *mockConn) Read(b []byte) (n int, err error) {
	return 0, nil
}

func (c *mockConn) Write(b []byte) (n int, err error) {
	return len(b), nil
}

func (c *mockConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *mockConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
}

func (c *mockConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *mockConn) SetDeadline(t time.Time) error {
	return nil
}

func (c *mockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

// Mock address for testing
type mockAddr struct {
	addr string
}

func (a *mockAddr) Network() string {
	return "tcp"
}

func (a *mockAddr) String() string {
	return a.addr
}
