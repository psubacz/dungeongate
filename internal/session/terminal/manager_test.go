package terminal

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	logger := slog.Default()
	manager := NewManager(logger)

	assert.NotNil(t, manager)
	assert.Equal(t, logger, manager.logger)
}

func TestManagerStartStop(t *testing.T) {
	logger := slog.Default()
	manager := NewManager(logger)
	ctx := context.Background()

	// Test start
	err := manager.Start(ctx)
	assert.NoError(t, err)

	// Test stop
	err = manager.Stop(ctx)
	assert.NoError(t, err)
}

func TestCreatePTY(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PTY not supported on Windows")
	}

	logger := slog.Default()
	manager := NewManager(logger)
	ctx := context.Background()

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	// Test creating PTY with echo command
	session, err := manager.CreatePTY("test-session", []string{"echo", "hello"}, 24, 80)
	
	assert.NoError(t, err)
	assert.NotNil(t, session)
	assert.Equal(t, "test-session", session.ID)
	assert.Equal(t, 24, session.Rows)
	assert.Equal(t, 80, session.Cols)
	assert.NotNil(t, session.PTY)
	assert.NotNil(t, session.Command)

	// Clean up
	manager.RemovePTY("test-session")
}

func TestCreatePTYEmptyCommand(t *testing.T) {
	logger := slog.Default()
	manager := NewManager(logger)
	ctx := context.Background()

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	// Test creating PTY with empty command
	session, err := manager.CreatePTY("test-session", []string{}, 24, 80)
	
	assert.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "command cannot be empty")
}

func TestCreatePTYInvalidCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PTY not supported on Windows")
	}

	logger := slog.Default()
	manager := NewManager(logger)
	ctx := context.Background()

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	// Test creating PTY with invalid command
	session, err := manager.CreatePTY("test-session", []string{"nonexistent-command"}, 24, 80)
	
	assert.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "failed to start PTY")
}

func TestGetPTY(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PTY not supported on Windows")
	}

	logger := slog.Default()
	manager := NewManager(logger)
	ctx := context.Background()

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	// Test getting non-existent PTY
	session, exists := manager.GetPTY("non-existent")
	assert.False(t, exists)
	assert.Nil(t, session)

	// Create a PTY
	createdSession, err := manager.CreatePTY("test-session", []string{"echo", "hello"}, 24, 80)
	require.NoError(t, err)
	defer manager.RemovePTY("test-session")

	// Test getting existing PTY
	session, exists = manager.GetPTY("test-session")
	assert.True(t, exists)
	assert.NotNil(t, session)
	assert.Equal(t, createdSession.ID, session.ID)
}

func TestRemovePTY(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PTY not supported on Windows")
	}

	logger := slog.Default()
	manager := NewManager(logger)
	ctx := context.Background()

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	// Create a PTY
	_, err = manager.CreatePTY("test-session", []string{"echo", "hello"}, 24, 80)
	require.NoError(t, err)

	// Verify it exists
	session, exists := manager.GetPTY("test-session")
	assert.True(t, exists)
	assert.NotNil(t, session)

	// Remove it
	manager.RemovePTY("test-session")

	// Verify it's gone
	session, exists = manager.GetPTY("test-session")
	assert.False(t, exists)
	assert.Nil(t, session)

	// Test removing non-existent PTY (should not panic)
	manager.RemovePTY("non-existent")
}

func TestResizePTY(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PTY not supported on Windows")
	}

	logger := slog.Default()
	manager := NewManager(logger)
	ctx := context.Background()

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	// Test resizing non-existent PTY
	err = manager.ResizePTY("non-existent", 30, 100)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PTY session not found")

	// Create a PTY
	_, err = manager.CreatePTY("test-session", []string{"echo", "hello"}, 24, 80)
	require.NoError(t, err)
	defer manager.RemovePTY("test-session")

	// Test resizing existing PTY
	err = manager.ResizePTY("test-session", 30, 100)
	assert.NoError(t, err)

	// Verify the size was updated
	session, exists := manager.GetPTY("test-session")
	assert.True(t, exists)
	assert.Equal(t, 30, session.Rows)
	assert.Equal(t, 100, session.Cols)
}

func TestPTYSessionReadWrite(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PTY not supported on Windows")
	}

	logger := slog.Default()
	manager := NewManager(logger)
	ctx := context.Background()

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	// Create a PTY with cat command (echoes input)
	session, err := manager.CreatePTY("test-session", []string{"cat"}, 24, 80)
	require.NoError(t, err)
	defer manager.RemovePTY("test-session")

	// Test write
	testData := []byte("hello\n")
	n, err := session.Write(testData)
	assert.NoError(t, err)
	assert.Equal(t, len(testData), n)

	// Test read (with timeout)
	readBuffer := make([]byte, 100)
	
	// Set a deadline to avoid hanging
	session.PTY.SetReadDeadline(time.Now().Add(2 * time.Second))
	
	n, err = session.Read(readBuffer)
	if err != nil {
		t.Logf("Read error (may be expected): %v", err)
	} else {
		assert.Greater(t, n, 0)
		t.Logf("Read %d bytes: %q", n, readBuffer[:n])
	}
}

func TestPTYSessionClose(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PTY not supported on Windows")
	}

	logger := slog.Default()
	manager := NewManager(logger)
	ctx := context.Background()

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	// Create a PTY
	session, err := manager.CreatePTY("test-session", []string{"echo", "hello"}, 24, 80)
	require.NoError(t, err)

	// Test close
	err = session.Close()
	assert.NoError(t, err)

	// Test read/write after close
	_, err = session.Write([]byte("test"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PTY not available")

	_, err = session.Read(make([]byte, 10))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PTY not available")

	// Test Fd after close
	fd := session.Fd()
	assert.Equal(t, uintptr(0), fd)
}

func TestPTYSessionFd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PTY not supported on Windows")
	}

	logger := slog.Default()
	manager := NewManager(logger)
	ctx := context.Background()

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	// Create a PTY
	session, err := manager.CreatePTY("test-session", []string{"echo", "hello"}, 24, 80)
	require.NoError(t, err)
	defer manager.RemovePTY("test-session")

	// Test Fd
	fd := session.Fd()
	assert.Greater(t, fd, uintptr(0))
}

func TestGetStats(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PTY not supported on Windows")
	}

	logger := slog.Default()
	manager := NewManager(logger)
	ctx := context.Background()

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	// Test stats with no sessions
	stats := manager.GetStats()
	assert.Equal(t, 0, stats.ActiveSessions)
	assert.Equal(t, 0, stats.TotalSessions)
	assert.Equal(t, 0, len(stats.Sessions))

	// Create a PTY
	_, err = manager.CreatePTY("test-session", []string{"echo", "hello"}, 24, 80)
	require.NoError(t, err)
	defer manager.RemovePTY("test-session")

	// Test stats with one session
	stats = manager.GetStats()
	assert.Equal(t, 1, stats.ActiveSessions)
	assert.Equal(t, 1, stats.TotalSessions)
	assert.Equal(t, 1, len(stats.Sessions))
	
	sessionInfo, exists := stats.Sessions["test-session"]
	assert.True(t, exists)
	assert.Equal(t, "test-session", sessionInfo.ID)
	assert.Equal(t, []string{"echo", "hello"}, sessionInfo.Command)
	assert.Equal(t, 24, sessionInfo.Rows)
	assert.Equal(t, 80, sessionInfo.Cols)
}

func TestManagerStopWithActiveSessions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PTY not supported on Windows")
	}

	logger := slog.Default()
	manager := NewManager(logger)
	ctx := context.Background()

	err := manager.Start(ctx)
	require.NoError(t, err)

	// Create multiple PTYs
	for i := 0; i < 3; i++ {
		sessionID := fmt.Sprintf("test-session-%d", i)
		_, err = manager.CreatePTY(sessionID, []string{"echo", "hello"}, 24, 80)
		require.NoError(t, err)
	}

	// Verify sessions exist
	stats := manager.GetStats()
	assert.Equal(t, 3, stats.ActiveSessions)

	// Stop manager (should close all sessions)
	err = manager.Stop(ctx)
	assert.NoError(t, err)

	// Verify all sessions are cleaned up
	stats = manager.GetStats()
	assert.Equal(t, 0, stats.ActiveSessions)
}

func TestConcurrentOperations(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PTY not supported on Windows")
	}

	logger := slog.Default()
	manager := NewManager(logger)
	ctx := context.Background()

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	const numSessions = 10
	
	// Create sessions concurrently
	for i := 0; i < numSessions; i++ {
		go func(id int) {
			sessionID := fmt.Sprintf("session-%d", id)
			_, err := manager.CreatePTY(sessionID, []string{"echo", "hello"}, 24, 80)
			assert.NoError(t, err)
		}(i)
	}

	// Give some time for all sessions to be created
	time.Sleep(100 * time.Millisecond)

	// Verify sessions were created
	stats := manager.GetStats()
	assert.Equal(t, numSessions, stats.ActiveSessions)

	// Remove sessions concurrently
	for i := 0; i < numSessions; i++ {
		go func(id int) {
			sessionID := fmt.Sprintf("session-%d", id)
			manager.RemovePTY(sessionID)
		}(i)
	}

	// Give some time for all sessions to be removed
	time.Sleep(100 * time.Millisecond)

	// Verify sessions were removed
	stats = manager.GetStats()
	assert.Equal(t, 0, stats.ActiveSessions)
}

