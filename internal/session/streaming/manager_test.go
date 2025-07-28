package streaming

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewManager(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	manager := NewManager(logger, nil)

	assert.NotNil(t, manager)
	assert.Equal(t, logger, manager.logger)
}

func TestManager_Start(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	manager := NewManager(logger, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := manager.Start(ctx)

	assert.NoError(t, err)
}

func TestManager_Stop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	manager := NewManager(logger, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := manager.Stop(ctx)

	assert.NoError(t, err)
}

func TestManager_StartStop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	manager := NewManager(logger, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Start the manager
	err := manager.Start(ctx)
	assert.NoError(t, err)

	// Stop the manager
	err = manager.Stop(ctx)
	assert.NoError(t, err)
}

func TestManager_ContextCancellation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	manager := NewManager(logger, nil)

	// Create a context that is already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Should still work since the manager doesn't do anything context-dependent yet
	err := manager.Start(ctx)
	assert.NoError(t, err)

	err = manager.Stop(ctx)
	assert.NoError(t, err)
}

// Test Stream struct
func TestStream_Creation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	stream := &Stream{
		ID:        "test-stream-1",
		SessionID: "session-123",
		UserID:    "user-456",
		Active:    true,
		logger:    logger,
	}

	assert.Equal(t, "test-stream-1", stream.ID)
	assert.Equal(t, "session-123", stream.SessionID)
	assert.Equal(t, "user-456", stream.UserID)
	assert.True(t, stream.Active)
	assert.Equal(t, logger, stream.logger)
}

// Test Spectator struct
func TestSpectator_Creation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	spectator := &Spectator{
		ID:        "spectator-1",
		StreamID:  "stream-123",
		UserID:    "user-789",
		Connected: true,
		logger:    logger,
	}

	assert.Equal(t, "spectator-1", spectator.ID)
	assert.Equal(t, "stream-123", spectator.StreamID)
	assert.Equal(t, "user-789", spectator.UserID)
	assert.True(t, spectator.Connected)
	assert.Equal(t, logger, spectator.logger)
}

// Test concurrent operations
func TestManager_ConcurrentOperations(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	manager := NewManager(logger, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Run start and stop operations concurrently
	done := make(chan error, 10)

	for i := 0; i < 5; i++ {
		go func() {
			done <- manager.Start(ctx)
		}()

		go func() {
			done <- manager.Stop(ctx)
		}()
	}

	// Collect all results
	for i := 0; i < 10; i++ {
		err := <-done
		assert.NoError(t, err)
	}
}

// Test that manager truly is stateless
func TestManager_Stateless(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	manager := NewManager(logger, nil)

	// Since the manager is stateless, we can call start and stop multiple times
	// without any issues
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		err := manager.Start(ctx)
		assert.NoError(t, err)

		err = manager.Stop(ctx)
		assert.NoError(t, err)
	}
}

// Test with different contexts
func TestManager_DifferentContexts(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	manager := NewManager(logger, nil)

	// Test with background context
	err := manager.Start(context.Background())
	assert.NoError(t, err)

	err = manager.Stop(context.Background())
	assert.NoError(t, err)

	// Test with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = manager.Start(ctx)
	assert.NoError(t, err)

	err = manager.Stop(ctx)
	assert.NoError(t, err)

	// Test with value context
	ctx = context.WithValue(context.Background(), "test", "value")
	err = manager.Start(ctx)
	assert.NoError(t, err)

	err = manager.Stop(ctx)
	assert.NoError(t, err)
}

// Benchmark tests
func BenchmarkManager_Start(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	manager := NewManager(logger, nil)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.Start(ctx)
	}
}

func BenchmarkManager_Stop(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	manager := NewManager(logger, nil)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.Stop(ctx)
	}
}

func BenchmarkManager_StartStop(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	manager := NewManager(logger, nil)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.Start(ctx)
		manager.Stop(ctx)
	}
}

// Test edge cases
func TestManager_NilLogger(t *testing.T) {
	// Test that manager can handle nil logger gracefully
	manager := NewManager(nil, nil)
	assert.NotNil(t, manager)
	assert.Nil(t, manager.logger)

	ctx := context.Background()

	// Should still work (though it might panic in real usage)
	// This tests the basic structure
	err := manager.Start(ctx)
	assert.NoError(t, err)

	err = manager.Stop(ctx)
	assert.NoError(t, err)
}

func TestStream_DefaultValues(t *testing.T) {
	stream := &Stream{}

	assert.Equal(t, "", stream.ID)
	assert.Equal(t, "", stream.SessionID)
	assert.Equal(t, "", stream.UserID)
	assert.False(t, stream.Active)
	assert.Nil(t, stream.logger)
}

func TestSpectator_DefaultValues(t *testing.T) {
	spectator := &Spectator{}

	assert.Equal(t, "", spectator.ID)
	assert.Equal(t, "", spectator.StreamID)
	assert.Equal(t, "", spectator.UserID)
	assert.False(t, spectator.Connected)
	assert.Nil(t, spectator.logger)
}

// Test with realistic scenarios
func TestManager_RealisticScenario(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	manager := NewManager(logger, nil)

	// Scenario: Starting streaming manager during service startup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start the manager
	err := manager.Start(ctx)
	assert.NoError(t, err, "Manager should start successfully")

	// Simulate some streaming activity (future implementation)
	// For now, just verify the manager can handle rapid operations
	for i := 0; i < 100; i++ {
		// In a real implementation, this might involve stream management
		// For now, we just verify stateless operation
		assert.NotNil(t, manager)
	}

	// Stop the manager during shutdown
	err = manager.Stop(ctx)
	assert.NoError(t, err, "Manager should stop successfully")
}

func TestManager_MultipleInstances(t *testing.T) {
	// Test that multiple manager instances can coexist (stateless design)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	manager1 := NewManager(logger, nil)
	manager2 := NewManager(logger, nil)
	manager3 := NewManager(logger, nil)

	ctx := context.Background()

	// All should work independently
	assert.NoError(t, manager1.Start(ctx))
	assert.NoError(t, manager2.Start(ctx))
	assert.NoError(t, manager3.Start(ctx))

	assert.NoError(t, manager1.Stop(ctx))
	assert.NoError(t, manager2.Stop(ctx))
	assert.NoError(t, manager3.Stop(ctx))
}
