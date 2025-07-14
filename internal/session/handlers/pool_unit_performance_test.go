package handlers

import (
	"context"
	"testing"
	"time"
	"log/slog"
	"os"
	"runtime"

	"github.com/stretchr/testify/require"
	"github.com/dungeongate/internal/session/pools"
)

// Simplified performance tests focusing on individual pool components

func BenchmarkWorkerPool_SubmitWork(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	
	workerPool, err := pools.NewWorkerPool(&pools.WorkerConfig{
		PoolSize:        50,
		QueueSize:       1000,
		WorkerTimeout:   30 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}, logger)
	require.NoError(b, err)
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	require.NoError(b, workerPool.Start(ctx))
	defer workerPool.Stop(ctx)
	
	b.ResetTimer()
	
	// Test work submission throughput
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			workItem := &pools.WorkItem{
				Type: pools.WorkTypeMenuAction,
				Handler: func(ctx context.Context, conn *pools.Connection) error {
					// Minimal work simulation
					return nil
				},
				Context:  ctx,
				Priority: pools.PriorityNormal,
				QueuedAt: time.Now(),
			}
			
			err := workerPool.Submit(workItem)
			if err != nil {
				b.Error(err)
			}
		}
	})
}

func BenchmarkPTYPool_AcquireRelease(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	
	ptyPool, err := pools.NewPTYPool(&pools.PTYConfig{
		MaxPTYs:         100,
		ReuseTimeout:    5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
		FDLimit:         512,
	}, logger)
	require.NoError(b, err)
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	require.NoError(b, ptyPool.Start(ctx))
	defer ptyPool.Stop(ctx)
	
	b.ResetTimer()
	
	// Test PTY acquisition/release throughput
	for i := 0; i < b.N; i++ {
		sessionID := "test-session"
		pty, err := ptyPool.AcquirePTY(sessionID)
		if err != nil {
			b.Error(err)
			continue
		}
		
		err = ptyPool.ReleasePTY(pty.ID)
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkBackpressureManager_CanAccept(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	
	backpressure, err := pools.NewBackpressureManager(&pools.BackpressureConfig{
		Enabled:          true,
		CircuitBreaker:   true,
		LoadShedding:     true,
		FailureThreshold: 10,
		RecoveryTimeout:  30 * time.Second,
		QueueSize:        100,
		CPUThreshold:     0.8,
		MemoryThreshold:  0.9,
	}, logger)
	require.NoError(b, err)
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	require.NoError(b, backpressure.Start(ctx))
	defer backpressure.Stop(ctx)
	
	b.ResetTimer()
	
	// Test backpressure check performance
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = backpressure.CanAccept()
		}
	})
}

// Memory test for individual pools
func TestWorkerPool_MemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory test in short mode")
	}
	
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	
	workerPool, err := pools.NewWorkerPool(&pools.WorkerConfig{
		PoolSize:        20,
		QueueSize:       500,
		WorkerTimeout:   30 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}, logger)
	require.NoError(t, err)
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	require.NoError(t, workerPool.Start(ctx))
	defer workerPool.Stop(ctx)
	
	// Record initial memory
	var initialMem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&initialMem)
	
	// Submit many work items
	const numItems = 10000
	for i := 0; i < numItems; i++ {
		workItem := &pools.WorkItem{
			Type: pools.WorkTypeMenuAction,
			Handler: func(ctx context.Context, conn *pools.Connection) error {
				time.Sleep(1 * time.Millisecond)
				return nil
			},
			Context:  ctx,
			Priority: pools.PriorityNormal,
			QueuedAt: time.Now(),
		}
		
		err := workerPool.Submit(workItem)
		require.NoError(t, err)
	}
	
	// Wait for work to complete
	time.Sleep(5 * time.Second)
	
	// Check final memory
	runtime.GC()
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)
	
	t.Logf("Initial memory: %d bytes", initialMem.Alloc)
	t.Logf("Final memory: %d bytes", finalMem.Alloc)
	t.Logf("Memory increase: %d bytes", finalMem.Alloc-initialMem.Alloc)
	
	// Memory should not increase significantly
	memoryIncrease := finalMem.Alloc - initialMem.Alloc
	maxIncrease := uint64(10 * 1024 * 1024) // 10MB
	
	if memoryIncrease > maxIncrease {
		t.Errorf("Memory usage too high: %d bytes (max: %d bytes)", memoryIncrease, maxIncrease)
	}
}

func TestPTYPool_MemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory test in short mode")
	}
	
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	
	ptyPool, err := pools.NewPTYPool(&pools.PTYConfig{
		MaxPTYs:         50,
		ReuseTimeout:    1 * time.Minute,
		CleanupInterval: 30 * time.Second,
		FDLimit:         256,
	}, logger)
	require.NoError(t, err)
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	require.NoError(t, ptyPool.Start(ctx))
	defer ptyPool.Stop(ctx)
	
	// Record initial memory
	var initialMem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&initialMem)
	
	// Allocate and release many PTYs
	const numOperations = 100
	for i := 0; i < numOperations; i++ {
		sessionID := "test-session"
		pty, err := ptyPool.AcquirePTY(sessionID)
		require.NoError(t, err)
		
		err = ptyPool.ReleasePTY(pty.ID)
		require.NoError(t, err)
		
		// Force GC periodically
		if i%20 == 0 {
			runtime.GC()
		}
	}
	
	// Final GC and memory check
	runtime.GC()
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)
	
	t.Logf("PTY operations: %d", numOperations)
	t.Logf("Initial memory: %d bytes", initialMem.Alloc)
	t.Logf("Final memory: %d bytes", finalMem.Alloc)
	t.Logf("Memory increase: %d bytes", finalMem.Alloc-initialMem.Alloc)
	
	// Memory should be reasonable
	memoryIncrease := finalMem.Alloc - initialMem.Alloc
	maxIncrease := uint64(20 * 1024 * 1024) // 20MB (PTYs use more memory)
	
	if memoryIncrease > maxIncrease {
		t.Errorf("PTY pool memory usage too high: %d bytes (max: %d bytes)", memoryIncrease, maxIncrease)
	}
}

// Concurrent stress test
func TestPoolArchitecture_ConcurrentStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}
	
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	
	// Create pools
	workerPool, err := pools.NewWorkerPool(&pools.WorkerConfig{
		PoolSize:        30,
		QueueSize:       1000,
		WorkerTimeout:   30 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}, logger)
	require.NoError(t, err)
	
	ptyPool, err := pools.NewPTYPool(&pools.PTYConfig{
		MaxPTYs:         100,
		ReuseTimeout:    2 * time.Minute,
		CleanupInterval: 30 * time.Second,
		FDLimit:         512,
	}, logger)
	require.NoError(t, err)
	
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	
	// Start pools
	require.NoError(t, workerPool.Start(ctx))
	require.NoError(t, ptyPool.Start(ctx))
	
	defer func() {
		workerPool.Stop(ctx)
		ptyPool.Stop(ctx)
	}()
	
	// Run concurrent operations
	const numGoroutines = 20
	const operationsPerGoroutine = 50
	
	done := make(chan error, numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < operationsPerGoroutine; j++ {
				// Submit work that uses PTY
				workItem := &pools.WorkItem{
					Type: pools.WorkTypeMenuAction,
					Handler: func(ctx context.Context, conn *pools.Connection) error {
						sessionID := "stress-test-session"
						pty, err := ptyPool.AcquirePTY(sessionID)
						if err != nil {
							return err
						}
						
						// Simulate work
						time.Sleep(10 * time.Millisecond)
						
						return ptyPool.ReleasePTY(pty.ID)
					},
					Context:  ctx,
					Priority: pools.PriorityNormal,
					QueuedAt: time.Now(),
				}
				
				if err := workerPool.Submit(workItem); err != nil {
					done <- err
					return
				}
			}
			done <- nil
		}(i)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(30 * time.Second):
			t.Fatal("Stress test timed out")
		}
	}
	
	t.Logf("Completed %d concurrent operations successfully", numGoroutines*operationsPerGoroutine)
}