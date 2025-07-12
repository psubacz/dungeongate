package pty

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPTYManager_Simple(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	manager := NewPTYManager(logger)

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.sessions)
	assert.Equal(t, logger, manager.logger)
}

func TestPTYManager_GetPTY_NotFound(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	manager := NewPTYManager(logger)

	// Test getting non-existent PTY
	ptySession, err := manager.GetPTY("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, ptySession)
	assert.Contains(t, err.Error(), "PTY not found")
}

func TestPTYManager_ResizePTY_NotFound(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	manager := NewPTYManager(logger)

	// Test resizing non-existent PTY
	err := manager.ResizePTY("nonexistent", 50, 100)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PTY not found")
}

func TestPTYSession_Channels(t *testing.T) {
	session := &PTYSession{
		SessionID:  "test-session",
		inputChan:  make(chan []byte, 1),
		outputChan: make(chan []byte, 1),
		errorChan:  make(chan error, 1),
		closeChan:  make(chan struct{}),
	}

	// Test channel access
	outputChan := session.GetOutput()
	errorChan := session.GetError()

	assert.NotNil(t, outputChan)
	assert.NotNil(t, errorChan)

	// Test sending to input channel
	testData := []byte("test")
	err := session.SendInput(testData)
	assert.NoError(t, err)

	// Should be able to receive from input channel (internal)
	select {
	case data := <-session.inputChan:
		assert.Equal(t, testData, data)
	default:
		t.Fatal("Expected data on input channel")
	}
}

func TestPTYSession_Close_Simple(t *testing.T) {
	session := &PTYSession{
		SessionID:  "test-session",
		inputChan:  make(chan []byte, 1),
		outputChan: make(chan []byte, 1),
		errorChan:  make(chan error, 1),
		closeChan:  make(chan struct{}),
		PTY:        nil, // This is OK, Close method checks for nil
		Cmd:        nil, // This is OK, Close method checks for nil
	}

	// Should be able to close multiple times without panic
	session.Close()
	session.Close()

	// Verify close channel is closed
	select {
	case <-session.closeChan:
		// OK, channel is closed
	default:
		t.Fatal("Close channel should be closed")
	}
}
