package grpc

import (
	"log/slog"
	"os"
	"testing"

	"github.com/dungeongate/internal/games/infrastructure/pty"
	"github.com/stretchr/testify/assert"
)

func TestNewStreamHandler_Simple(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	ptyManager := pty.NewPTYManager(logger)

	handler := NewStreamHandler(ptyManager, logger)

	assert.NotNil(t, handler)
	assert.Equal(t, ptyManager, handler.ptyManager)
	assert.Equal(t, logger, handler.logger)
	assert.NotNil(t, handler.sessions)
}

func TestStreamSession_Close_Simple(t *testing.T) {
	session := &StreamSession{
		sessionID: "test-session",
		closeChan: make(chan struct{}),
	}

	// Should be able to close multiple times without panic
	session.Close()
	session.Close()

	// Verify channel is closed
	select {
	case <-session.closeChan:
		// OK, channel is closed
	default:
		t.Fatal("Close channel should be closed")
	}
}

func TestStreamHandler_Sessions_Management(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	ptyManager := pty.NewPTYManager(logger)
	handler := NewStreamHandler(ptyManager, logger)

	sessionID := "test-session"

	// Initially empty
	assert.Empty(t, handler.sessions)

	// Add a session
	streamSession := &StreamSession{
		sessionID: sessionID,
		closeChan: make(chan struct{}),
	}

	handler.mu.Lock()
	handler.sessions[sessionID] = streamSession
	handler.mu.Unlock()

	// Should have one session
	assert.Len(t, handler.sessions, 1)
	assert.Contains(t, handler.sessions, sessionID)

	// Remove session
	handler.mu.Lock()
	delete(handler.sessions, sessionID)
	handler.mu.Unlock()

	// Should be empty again
	assert.Empty(t, handler.sessions)
}
