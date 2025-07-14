package handlers

import (
	"context"
	"testing"
	"time"
	"log/slog"
	"os"
	"runtime"
	"sync"

	"github.com/stretchr/testify/require"
	"github.com/dungeongate/internal/session/pools"
	"github.com/dungeongate/internal/session/resources"
)

// createPerformanceTestHandler creates a SessionHandler optimized for performance testing
func createPerformanceTestHandler(b *testing.B) *SessionHandler {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	
	// Larger pool configurations for performance testing
	connectionPool, err := pools.NewConnectionPool(&pools.Config{
		MaxConnections: 5000,
		QueueSize:      1000,
		QueueTimeout:   30 * time.Second,
		IdleTimeout:    300 * time.Second,
		DrainTimeout:   60 * time.Second,
		WorkerPoolSize: 200,
		MaxPTYs:        2000,
	}, logger)
	require.NoError(b, err)

	workerPool, err := pools.NewWorkerPool(&pools.WorkerConfig{
		PoolSize:        200,
		QueueSize:       5000,
		WorkerTimeout:   60 * time.Second,
		ShutdownTimeout: 30 * time.Second,
	}, logger)
	require.NoError(b, err)

	ptyPool, err := pools.NewPTYPool(&pools.PTYConfig{
		MaxPTYs:         2000,
		ReuseTimeout:    5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
		FDLimit:         4096,
	}, logger)
	require.NoError(b, err)

	backpressure, err := pools.NewBackpressureManager(&pools.BackpressureConfig{
		Enabled:          true,
		CircuitBreaker:   true,
		LoadShedding:     true,
		FailureThreshold: 50,
		RecoveryTimeout:  60 * time.Second,
		QueueSize:        1000,
		CPUThreshold:     0.8,
		MemoryThreshold:  0.9,
	}, logger)
	require.NoError(b, err)

	resourceLimiter, err := resources.NewResourceLimiter(&resources.Config{}, logger)
	require.NoError(b, err)

	resourceTracker := resources.NewResourceTracker(logger)

	metricsRegistry := resources.NewMetricsRegistry(&resources.MetricsConfig{
		CollectionInterval: 5 * time.Second,
		ExportInterval:     30 * time.Second,
		RetentionPeriod:    24 * time.Hour,
		DefaultBuckets:     []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, logger)

	// Create handlers
	authHandler := NewAuthHandler(nil, resourceLimiter, workerPool, metricsRegistry, logger)
	gameHandler := NewGameHandler(nil, ptyPool, resourceTracker, workerPool, metricsRegistry, logger)
	streamHandler := NewStreamHandler(resourceTracker, workerPool, metricsRegistry, logger)

	// Create server config
	serverConfig := &ServerConfig{
		SSH: SSHServerConfig{
			Address:     "localhost",
			Port:        0,
			HostKeyPath: "",
			Banner:      "Performance Test",
		},
		HTTP: HTTPServerConfig{
			Address: "localhost",
			Port:    0,
		},
		GRPC: GRPCServerConfig{
			Address: "localhost",
			Port:    0,
		},
	}

	return NewSessionHandler(
		connectionPool, workerPool, ptyPool, backpressure,
		resourceLimiter, resourceTracker, metricsRegistry,
		authHandler, gameHandler, streamHandler, nil,
		serverConfig, logger)
}

// Performance test targets:
// - Connection Handling: 5000+ concurrent connections
// - Connection Setup: < 5ms average connection establishment  
// - Memory Efficiency: No memory leaks under load
// - Throughput: Handle 10000+ operations per minute

func BenchmarkConnectionPool_RequestRelease(b *testing.B) {
	sessionHandler := createPerformanceTestHandler(b)
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	// Start pools
	require.NoError(b, sessionHandler.connectionPool.Start(ctx))
	require.NoError(b, sessionHandler.workerPool.Start(ctx))
	defer func() {
		sessionHandler.connectionPool.Stop(ctx)
		sessionHandler.workerPool.Stop(ctx)
	}()
	
	b.ResetTimer()
	
	// Target: < 5ms average connection establishment
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			conn, err := sessionHandler.connectionPool.RequestConnection(ctx, nil, nil, pools.PriorityNormal)
			if err != nil {
				b.Error(err)
				continue
			}
			sessionHandler.connectionPool.ReleaseConnection(conn.ID)
		}
	})
}

func BenchmarkConnectionPool_ConcurrentConnections(b *testing.B) {
	sessionHandler := createPerformanceTestHandler(b)
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	require.NoError(b, sessionHandler.connectionPool.Start(ctx))
	require.NoError(b, sessionHandler.workerPool.Start(ctx))
	defer func() {
		sessionHandler.connectionPool.Stop(ctx)
		sessionHandler.workerPool.Stop(ctx)
	}()
	
	// Target: Handle 1000+ concurrent connections efficiently
	const targetConnections = 1000
	connections := make([]*pools.Connection, 0, targetConnections)
	var mu sync.Mutex
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// Acquire multiple connections concurrently
		var wg sync.WaitGroup
		errCh := make(chan error, targetConnections)
		
		for j := 0; j < targetConnections; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				conn, err := sessionHandler.connectionPool.RequestConnection(ctx, nil, nil, pools.PriorityNormal)
				if err != nil {
					errCh <- err
					return
				}
				
				mu.Lock()
				connections = append(connections, conn)
				mu.Unlock()
			}()
		}
		
		wg.Wait()
		close(errCh)
		
		// Check for errors
		for err := range errCh {
			if err != nil {
				b.Error(err)
			}
		}
		
		// Release all connections
		mu.Lock()
		for _, conn := range connections {
			sessionHandler.connectionPool.ReleaseConnection(conn.ID)
		}
		connections = connections[:0]
		mu.Unlock()
	}
}

func BenchmarkWorkerPool_TaskProcessing(b *testing.B) {
	sessionHandler := createPerformanceTestHandler(b)
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	require.NoError(b, sessionHandler.workerPool.Start(ctx))
	defer sessionHandler.workerPool.Stop(ctx)
	
	b.ResetTimer()
	
	// Target: Process 10000+ tasks per minute efficiently
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			workItem := &pools.WorkItem{
				Type:     pools.WorkTypeMenuAction,
				Handler:  func(ctx context.Context, conn *pools.Connection) error { return nil },
				Context:  ctx,
				Priority: pools.PriorityNormal,
				QueuedAt: time.Now(),
			}
			
			err := sessionHandler.workerPool.Submit(workItem)
			if err != nil {
				b.Error(err)
			}
		}
	})
}

func BenchmarkPTYPool_AllocationSpeed(b *testing.B) {
	sessionHandler := createPerformanceTestHandler(b)
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	require.NoError(b, sessionHandler.ptyPool.Start(ctx))
	defer sessionHandler.ptyPool.Stop(ctx)
	
	b.ResetTimer()
	
	// Target: Fast PTY allocation/deallocation
	for i := 0; i < b.N; i++ {
		pty, err := sessionHandler.ptyPool.AcquirePTY("test-session")
		if err != nil {
			b.Error(err)
			continue
		}
		
		sessionHandler.ptyPool.ReleasePTY(pty.ID)
	}
}

func BenchmarkFullPoolArchitecture_EndToEndFlow(b *testing.B) {
	sessionHandler := createPerformanceTestHandler(b)
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	// Start all pools
	require.NoError(b, sessionHandler.connectionPool.Start(ctx))
	require.NoError(b, sessionHandler.workerPool.Start(ctx))
	require.NoError(b, sessionHandler.ptyPool.Start(ctx))
	require.NoError(b, sessionHandler.backpressure.Start(ctx))
	require.NoError(b, sessionHandler.resourceTracker.Start(ctx))
	
	defer func() {
		sessionHandler.connectionPool.Stop(ctx)
		sessionHandler.workerPool.Stop(ctx)
		sessionHandler.ptyPool.Stop(ctx)
		sessionHandler.backpressure.Stop(ctx)
		sessionHandler.resourceTracker.Stop(ctx)
	}()
	
	b.ResetTimer()
	
	// Simulate complete session flow: connection -> work -> PTY -> release
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// 1. Get connection from pool
			conn, err := sessionHandler.connectionPool.RequestConnection(ctx, nil, nil, pools.PriorityNormal)
			if err != nil {
				b.Error(err)
				continue
			}
			
			// 2. Submit work to worker pool
			workItem := &pools.WorkItem{
				Type:       pools.WorkTypeMenuAction,
				Connection: conn,
				Handler: func(ctx context.Context, conn *pools.Connection) error {
					// 3. Allocate PTY
					pty, err := sessionHandler.ptyPool.AcquirePTY(conn.ID)
					if err != nil {
						return err
					}
					
					// 4. Simulate some work
					time.Sleep(1 * time.Millisecond)
					
					// 5. Release PTY
					sessionHandler.ptyPool.ReleasePTY(pty.ID)
					return nil
				},
				Context:  ctx,
				Priority: pools.PriorityNormal,
				QueuedAt: time.Now(),
			}
			
			err = sessionHandler.workerPool.Submit(workItem)
			if err != nil {
				b.Error(err)
			}
			
			// 6. Release connection
			sessionHandler.connectionPool.ReleaseConnection(conn.ID)
		}
	})
}

// Memory leak test - runs for extended time to detect leaks
func TestPoolBasedArchitecture_MemoryLeaks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}
	
	sessionHandler := createPerformanceTestHandler(&testing.B{})
	
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	
	// Start all pools
	require.NoError(t, sessionHandler.connectionPool.Start(ctx))
	require.NoError(t, sessionHandler.workerPool.Start(ctx))
	require.NoError(t, sessionHandler.ptyPool.Start(ctx))
	require.NoError(t, sessionHandler.backpressure.Start(ctx))
	require.NoError(t, sessionHandler.resourceTracker.Start(ctx))
	
	defer func() {
		sessionHandler.connectionPool.Stop(ctx)
		sessionHandler.workerPool.Stop(ctx)
		sessionHandler.ptyPool.Stop(ctx)
		sessionHandler.backpressure.Stop(ctx)
		sessionHandler.resourceTracker.Stop(ctx)
	}()
	
	// Record initial memory stats
	var initialMem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&initialMem)
	
	// Run operations for 2 minutes
	const testDuration = 60 * time.Second
	startTime := time.Now()
	operationCount := 0
	
	for time.Since(startTime) < testDuration {
		// Perform typical operations
		conn, err := sessionHandler.connectionPool.RequestConnection(ctx, nil, nil, pools.PriorityNormal)
		require.NoError(t, err)
		
		workItem := &pools.WorkItem{
			Type:       pools.WorkTypeMenuAction,
			Connection: conn,
			Handler: func(ctx context.Context, conn *pools.Connection) error {
				pty, err := sessionHandler.ptyPool.AcquirePTY(conn.ID)
				if err != nil {
					return err
				}
				time.Sleep(1 * time.Millisecond)
				sessionHandler.ptyPool.ReleasePTY(pty.ID)
				return nil
			},
			Context:  ctx,
			Priority: pools.PriorityNormal,
			QueuedAt: time.Now(),
		}
		
		err = sessionHandler.workerPool.Submit(workItem)
		require.NoError(t, err)
		
		sessionHandler.connectionPool.ReleaseConnection(conn.ID)
		operationCount++
		
		// Force GC periodically
		if operationCount%1000 == 0 {
			runtime.GC()
		}
	}
	
	// Force final GC and check memory
	runtime.GC()
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)
	
	// Log memory usage
	t.Logf("Operations completed: %d", operationCount)
	t.Logf("Initial memory: %d bytes", initialMem.Alloc)
	t.Logf("Final memory: %d bytes", finalMem.Alloc)
	t.Logf("Memory increase: %d bytes", finalMem.Alloc-initialMem.Alloc)
	t.Logf("Operations per second: %.2f", float64(operationCount)/testDuration.Seconds())
	
	// Memory should not increase significantly (< 50MB growth is acceptable)
	memoryIncrease := finalMem.Alloc - initialMem.Alloc
	maxAcceptableIncrease := uint64(50 * 1024 * 1024) // 50MB
	
	if memoryIncrease > maxAcceptableIncrease {
		t.Errorf("Potential memory leak detected: memory increased by %d bytes (> %d bytes)", 
			memoryIncrease, maxAcceptableIncrease)
	}
	
	// Should achieve reasonable throughput (> 1000 ops/sec)
	opsPerSecond := float64(operationCount) / testDuration.Seconds()
	if opsPerSecond < 1000 {
		t.Errorf("Performance below target: %.2f ops/sec (target: > 1000 ops/sec)", opsPerSecond)
	}
}

// Resource exhaustion test - verify graceful behavior at limits
func TestPoolBasedArchitecture_ResourceExhaustion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource exhaustion test in short mode")
	}
	
	sessionHandler := createPerformanceTestHandler(&testing.B{})
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	// Start pools
	require.NoError(t, sessionHandler.connectionPool.Start(ctx))
	require.NoError(t, sessionHandler.workerPool.Start(ctx))
	require.NoError(t, sessionHandler.ptyPool.Start(ctx))
	require.NoError(t, sessionHandler.backpressure.Start(ctx))
	require.NoError(t, sessionHandler.resourceTracker.Start(ctx))
	
	defer func() {
		sessionHandler.connectionPool.Stop(ctx)
		sessionHandler.workerPool.Stop(ctx)
		sessionHandler.ptyPool.Stop(ctx)
		sessionHandler.backpressure.Stop(ctx)
		sessionHandler.resourceTracker.Stop(ctx)
	}()
	
	// Try to exhaust connection pool
	const maxAttempts = 6000 // More than pool max (5000)
	connections := make([]*pools.Connection, 0, maxAttempts)
	successes := 0
	failures := 0
	
	for i := 0; i < maxAttempts; i++ {
		conn, err := sessionHandler.connectionPool.RequestConnection(ctx, nil, nil, pools.PriorityNormal)
		if err != nil {
			failures++
			break // Stop on first failure
		} else {
			connections = append(connections, conn)
			successes++
		}
	}
	
	t.Logf("Connection pool test: %d successes, %d failures", successes, failures)
	
	// Should hit pool limits and fail gracefully
	require.True(t, failures > 0, "Should hit pool limits")
	require.True(t, successes > 0, "Should handle some connections")
	
	// Clean up connections
	for _, conn := range connections {
		sessionHandler.connectionPool.ReleaseConnection(conn.ID)
	}
	
	// Verify pool recovers after releasing connections
	conn, err := sessionHandler.connectionPool.RequestConnection(ctx, nil, nil, pools.PriorityNormal)
	require.NoError(t, err, "Pool should recover after releasing connections")
	sessionHandler.connectionPool.ReleaseConnection(conn.ID)
}