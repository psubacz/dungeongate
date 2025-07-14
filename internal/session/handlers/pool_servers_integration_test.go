package handlers

import (
	"context"
	"testing"
	"time"
	"log/slog"
	"os"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/dungeongate/internal/session/pools"
	"github.com/dungeongate/internal/session/resources"
)

// createTestSessionHandler creates a real SessionHandler for testing
func createTestSessionHandler(t *testing.T) *SessionHandler {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	
	// Create minimal pool configurations for testing
	connectionPool, err := pools.NewConnectionPool(&pools.Config{
		MaxConnections: 10,
		QueueSize:      5,
		QueueTimeout:   5 * time.Second,
		IdleTimeout:    30 * time.Second,
		DrainTimeout:   10 * time.Second,
		WorkerPoolSize: 5,
		MaxPTYs:        10,
	}, logger)
	require.NoError(t, err)

	workerPool, err := pools.NewWorkerPool(&pools.WorkerConfig{
		PoolSize:        5,
		QueueSize:       20,
		WorkerTimeout:   10 * time.Second,
		ShutdownTimeout: 5 * time.Second,
	}, logger)
	require.NoError(t, err)

	ptyPool, err := pools.NewPTYPool(&pools.PTYConfig{
		MaxPTYs:         10,
		ReuseTimeout:    1 * time.Minute,
		CleanupInterval: 30 * time.Second,
		FDLimit:         64,
	}, logger)
	require.NoError(t, err)

	backpressure, err := pools.NewBackpressureManager(&pools.BackpressureConfig{
		Enabled:          true,
		CircuitBreaker:   false, // Disable for testing
		LoadShedding:     false,
		FailureThreshold: 5,
		RecoveryTimeout:  30 * time.Second,
		QueueSize:        10,
		CPUThreshold:     0.9,
		MemoryThreshold:  0.95,
	}, logger)
	require.NoError(t, err)

	resourceLimiter, err := resources.NewResourceLimiter(&resources.Config{}, logger)
	require.NoError(t, err)

	resourceTracker := resources.NewResourceTracker(logger)

	metricsRegistry := resources.NewMetricsRegistry(&resources.MetricsConfig{
		CollectionInterval: 1 * time.Second,
		ExportInterval:     5 * time.Second,
		RetentionPeriod:    1 * time.Hour,
		DefaultBuckets:     []float64{0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
	}, logger)

	// Create minimal handlers
	authHandler := NewAuthHandler(nil, resourceLimiter, workerPool, metricsRegistry, logger)
	gameHandler := NewGameHandler(nil, ptyPool, resourceTracker, workerPool, metricsRegistry, logger)
	streamHandler := NewStreamHandler(resourceTracker, workerPool, metricsRegistry, logger)

	// Create server config
	serverConfig := &ServerConfig{
		SSH: SSHServerConfig{
			Address:     "localhost",
			Port:        0, // Random port for testing
			HostKeyPath: "",
			Banner:      "Test Banner",
		},
		HTTP: HTTPServerConfig{
			Address: "localhost",
			Port:    0, // Random port for testing
		},
		GRPC: GRPCServerConfig{
			Address: "localhost",
			Port:    0, // Random port for testing
		},
	}

	// Create session handler
	sessionHandler := NewSessionHandler(
		connectionPool, workerPool, ptyPool, backpressure,
		resourceLimiter, resourceTracker, metricsRegistry,
		authHandler, gameHandler, streamHandler, nil, // nil menu handler for testing
		serverConfig, logger)

	return sessionHandler
}

func TestPoolBasedServers_Lifecycle(t *testing.T) {
	sessionHandler := createTestSessionHandler(t)
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// Start all pools first
	require.NoError(t, sessionHandler.connectionPool.Start(ctx))
	require.NoError(t, sessionHandler.workerPool.Start(ctx))
	require.NoError(t, sessionHandler.ptyPool.Start(ctx))
	require.NoError(t, sessionHandler.backpressure.Start(ctx))
	require.NoError(t, sessionHandler.resourceTracker.Start(ctx))
	
	// Start servers
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- sessionHandler.StartServers(ctx)
	}()
	
	// Give servers time to start
	time.Sleep(300 * time.Millisecond)
	
	// Verify servers are created
	assert.Contains(t, sessionHandler.servers, "ssh")
	assert.Contains(t, sessionHandler.servers, "http")
	assert.Contains(t, sessionHandler.servers, "grpc")
	
	// Test that servers have names
	assert.Equal(t, "ssh", sessionHandler.servers["ssh"].Name())
	assert.Equal(t, "http", sessionHandler.servers["http"].Name())
	assert.Equal(t, "grpc", sessionHandler.servers["grpc"].Name())
	
	// Stop servers
	require.NoError(t, sessionHandler.StopServers(ctx))
	
	// Verify servers startup completes
	select {
	case err := <-serverDone:
		// Should complete without error or with context cancellation
		assert.True(t, err == nil || err == context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("Server startup did not complete after stop")
	}
	
	// Clean up pools
	sessionHandler.connectionPool.Stop(ctx)
	sessionHandler.workerPool.Stop(ctx)
	sessionHandler.ptyPool.Stop(ctx)
	sessionHandler.backpressure.Stop(ctx)
	sessionHandler.resourceTracker.Stop(ctx)
}

func TestPoolBasedHTTPServer_Integration(t *testing.T) {
	sessionHandler := createTestSessionHandler(t)
	
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	// Start pools
	require.NoError(t, sessionHandler.connectionPool.Start(ctx))
	require.NoError(t, sessionHandler.workerPool.Start(ctx))
	require.NoError(t, sessionHandler.ptyPool.Start(ctx))
	require.NoError(t, sessionHandler.backpressure.Start(ctx))
	require.NoError(t, sessionHandler.resourceTracker.Start(ctx))
	
	// Start HTTP server only
	httpServer := sessionHandler.servers["http"]
	require.NotNil(t, httpServer)
	
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- httpServer.Start(ctx)
	}()
	
	// Give server time to start
	time.Sleep(300 * time.Millisecond)
	
	// Get actual port - we need to add a method to get this
	// For now, let's test what we can
	
	// Stop server
	require.NoError(t, httpServer.Stop(ctx))
	
	// Verify server stops
	select {
	case err := <-serverDone:
		assert.True(t, err == nil || err == context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("HTTP server did not stop")
	}
	
	// Clean up
	sessionHandler.connectionPool.Stop(ctx)
	sessionHandler.workerPool.Stop(ctx)
	sessionHandler.ptyPool.Stop(ctx)
	sessionHandler.backpressure.Stop(ctx)
	sessionHandler.resourceTracker.Stop(ctx)
}

func TestPoolBasedSSHServer_Integration(t *testing.T) {
	sessionHandler := createTestSessionHandler(t)
	
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	// Start pools
	require.NoError(t, sessionHandler.connectionPool.Start(ctx))
	require.NoError(t, sessionHandler.workerPool.Start(ctx))
	require.NoError(t, sessionHandler.ptyPool.Start(ctx))
	require.NoError(t, sessionHandler.backpressure.Start(ctx))
	require.NoError(t, sessionHandler.resourceTracker.Start(ctx))
	
	// Start SSH server only
	sshServer := sessionHandler.servers["ssh"]
	require.NotNil(t, sshServer)
	
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- sshServer.Start(ctx)
	}()
	
	// Give server time to start
	time.Sleep(300 * time.Millisecond)
	
	// Try to connect to verify server is listening
	// Note: This will fail SSH handshake but should verify server is accepting connections
	// We can't easily test the full SSH flow without proper key setup
	
	// Stop server
	require.NoError(t, sshServer.Stop(ctx))
	
	// Verify server stops
	select {
	case err := <-serverDone:
		assert.True(t, err == nil || err == context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("SSH server did not stop")
	}
	
	// Clean up
	sessionHandler.connectionPool.Stop(ctx)
	sessionHandler.workerPool.Stop(ctx)
	sessionHandler.ptyPool.Stop(ctx)
	sessionHandler.backpressure.Stop(ctx)
	sessionHandler.resourceTracker.Stop(ctx)
}

func TestFullPoolBasedService_Integration(t *testing.T) {
	// This test verifies the complete integration as would be used in production
	sessionHandler := createTestSessionHandler(t)
	
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	
	// Start using the service integration function
	require.NoError(t, StartPoolBasedService(ctx, sessionHandler))
	
	// Give services time to stabilize
	time.Sleep(500 * time.Millisecond)
	
	// Verify all servers are running
	assert.Contains(t, sessionHandler.servers, "ssh")
	assert.Contains(t, sessionHandler.servers, "http")
	assert.Contains(t, sessionHandler.servers, "grpc")
	
	// Test graceful shutdown
	require.NoError(t, ShutdownPoolBasedService(ctx, sessionHandler))
}

// Benchmark test for complete pool-based architecture
func BenchmarkPoolBasedArchitecture_CompleteFlow(b *testing.B) {
	sessionHandler := createTestSessionHandler(&testing.T{})
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Start pools
	sessionHandler.connectionPool.Start(ctx)
	sessionHandler.workerPool.Start(ctx)
	sessionHandler.ptyPool.Start(ctx)
	sessionHandler.backpressure.Start(ctx)
	sessionHandler.resourceTracker.Start(ctx)
	
	defer func() {
		sessionHandler.connectionPool.Stop(ctx)
		sessionHandler.workerPool.Stop(ctx)
		sessionHandler.ptyPool.Stop(ctx)
		sessionHandler.backpressure.Stop(ctx)
		sessionHandler.resourceTracker.Stop(ctx)
	}()
	
	b.ResetTimer()
	
	// Benchmark connection pool operations
	b.Run("ConnectionPool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			conn, err := sessionHandler.connectionPool.RequestConnection(ctx, nil, nil, pools.PriorityNormal)
			if err != nil {
				b.Error(err)
				continue
			}
			sessionHandler.connectionPool.ReleaseConnection(conn.ID)
		}
	})
}