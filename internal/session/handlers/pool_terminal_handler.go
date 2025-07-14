package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dungeongate/internal/session/terminal"
	"golang.org/x/crypto/ssh"
)

// PoolTerminalHandler provides pool-compatible terminal I/O operations
// It's designed to work efficiently within the worker pool architecture
type PoolTerminalHandler struct {
	// Buffer pools for efficient memory management
	inputBufferPool  sync.Pool
	outputBufferPool sync.Pool
	
	// Input handling
	inputHandlers map[string]*terminal.InputHandler
	handlersMu    sync.RWMutex
	
	// Metrics and logging
	logger *slog.Logger
}

// NewPoolTerminalHandler creates a new pool-compatible terminal handler
func NewPoolTerminalHandler(logger *slog.Logger) *PoolTerminalHandler {
	pth := &PoolTerminalHandler{
		inputHandlers: make(map[string]*terminal.InputHandler),
		logger:        logger,
	}

	// Initialize buffer pools for memory efficiency
	pth.inputBufferPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 1024) // 1KB input buffer
		},
	}

	pth.outputBufferPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 4096) // 4KB output buffer
		},
	}

	return pth
}

// InputEvent represents a processed input event from the terminal
type InputEvent struct {
	Type      terminal.InputEventType
	Character rune
	KeyCode   terminal.KeyCode
	Data      []byte
	Timestamp time.Time
}

// ReadInput reads and processes input from an SSH channel with context awareness
func (pth *PoolTerminalHandler) ReadInput(ctx context.Context, channel ssh.Channel, connectionID string) (*InputEvent, error) {
	// Get or create input handler for this connection
	inputHandler := pth.getInputHandler(connectionID, channel)
	
	// Read input with context support
	event, err := inputHandler.ReadInput(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}

	// Convert to pool-compatible event
	poolEvent := &InputEvent{
		Type:      event.Type,
		Character: event.Character,
		KeyCode:   event.KeyCode,
		Data:      event.Data,
		Timestamp: time.Now(),
	}

	pth.logger.Debug("Terminal input received",
		"connection_id", connectionID,
		"event_type", event.Type,
		"character", string(event.Character),
		"key_code", event.KeyCode)

	return poolEvent, nil
}

// WriteOutput writes data to an SSH channel with context awareness and buffering
func (pth *PoolTerminalHandler) WriteOutput(ctx context.Context, channel ssh.Channel, data []byte) error {
	if len(data) == 0 {
		return nil
	}

	// Check context before writing
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Write data to channel
	written := 0
	for written < len(data) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, err := channel.Write(data[written:])
		if err != nil {
			return fmt.Errorf("failed to write to channel: %w", err)
		}
		written += n
	}

	return nil
}

// ClearScreen clears the terminal screen
func (pth *PoolTerminalHandler) ClearScreen(ctx context.Context, channel ssh.Channel) error {
	return pth.WriteOutput(ctx, channel, []byte("\033[2J\033[H"))
}

// MoveCursor moves the cursor to the specified position
func (pth *PoolTerminalHandler) MoveCursor(ctx context.Context, channel ssh.Channel, row, col int) error {
	sequence := fmt.Sprintf("\033[%d;%dH", row, col)
	return pth.WriteOutput(ctx, channel, []byte(sequence))
}

// SetCursorPosition positions the cursor at the beginning of the current line
func (pth *PoolTerminalHandler) SetCursorPosition(ctx context.Context, channel ssh.Channel, position string) error {
	var sequence string
	switch position {
	case "home":
		sequence = "\033[H"
	case "beginning":
		sequence = "\r"
	case "end":
		sequence = "\033[E"
	default:
		return fmt.Errorf("unknown cursor position: %s", position)
	}
	return pth.WriteOutput(ctx, channel, []byte(sequence))
}

// WriteWithFormatting writes text with ANSI formatting codes
func (pth *PoolTerminalHandler) WriteWithFormatting(ctx context.Context, channel ssh.Channel, text string, formatting map[string]string) error {
	var formattedText string
	
	// Apply formatting if provided
	if formatting != nil {
		if color, exists := formatting["color"]; exists {
			switch color {
			case "red":
				formattedText = "\033[31m" + text + "\033[0m"
			case "green":
				formattedText = "\033[32m" + text + "\033[0m"
			case "yellow":
				formattedText = "\033[33m" + text + "\033[0m"
			case "blue":
				formattedText = "\033[34m" + text + "\033[0m"
			case "magenta":
				formattedText = "\033[35m" + text + "\033[0m"
			case "cyan":
				formattedText = "\033[36m" + text + "\033[0m"
			case "white":
				formattedText = "\033[37m" + text + "\033[0m"
			default:
				formattedText = text
			}
		} else {
			formattedText = text
		}
		
		if style, exists := formatting["style"]; exists {
			switch style {
			case "bold":
				formattedText = "\033[1m" + formattedText + "\033[0m"
			case "underline":
				formattedText = "\033[4m" + formattedText + "\033[0m"
			case "blink":
				formattedText = "\033[5m" + formattedText + "\033[0m"
			}
		}
	} else {
		formattedText = text
	}

	return pth.WriteOutput(ctx, channel, []byte(formattedText))
}

// ReadLine reads a complete line of input with editing support
func (pth *PoolTerminalHandler) ReadLine(ctx context.Context, channel ssh.Channel, connectionID string, inputType terminal.InputType) (string, error) {
	// Create line editor for this input
	lineEditor := terminal.NewLineEditor(channel, inputType)
	
	pth.logger.Debug("Starting line input",
		"connection_id", connectionID,
		"input_type", inputType)

	// Read line with context support
	line, err := lineEditor.ReadLine(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read line: %w", err)
	}

	pth.logger.Debug("Line input completed",
		"connection_id", connectionID,
		"line_length", len(line))

	return line, nil
}

// ProcessControlSequences processes ANSI control sequences in input
func (pth *PoolTerminalHandler) ProcessControlSequences(input []byte) (*InputEvent, error) {
	if len(input) == 0 {
		return nil, fmt.Errorf("empty input")
	}

	// Handle basic control characters
	char := input[0]
	switch char {
	case 3: // Ctrl+C
		return &InputEvent{
			Type:      terminal.EventKey,
			KeyCode:   terminal.KeyCtrlC,
			Timestamp: time.Now(),
		}, nil
	case 4: // Ctrl+D
		return &InputEvent{
			Type:      terminal.EventKey,
			KeyCode:   terminal.KeyCtrlD,
			Timestamp: time.Now(),
		}, nil
	case 13, 10: // Enter/Return
		return &InputEvent{
			Type:      terminal.EventKey,
			KeyCode:   terminal.KeyEnter,
			Timestamp: time.Now(),
		}, nil
	case 27: // ESC - could be escape sequence
		if len(input) > 1 {
			return pth.processEscapeSequence(input)
		}
		return &InputEvent{
			Type:      terminal.EventKey,
			KeyCode:   terminal.KeyEscape,
			Timestamp: time.Now(),
		}, nil
	default:
		// Regular character
		if char >= 32 && char <= 126 { // Printable ASCII
			return &InputEvent{
				Type:      terminal.EventCharacter,
				Character: rune(char),
				Timestamp: time.Now(),
			}, nil
		}
		// Non-printable character
		return &InputEvent{
			Type:      terminal.EventSequence,
			Data:      input,
			Timestamp: time.Now(),
		}, nil
	}
}

// processEscapeSequence handles ANSI escape sequences
func (pth *PoolTerminalHandler) processEscapeSequence(input []byte) (*InputEvent, error) {
	if len(input) < 3 {
		return &InputEvent{
			Type:      terminal.EventKey,
			KeyCode:   terminal.KeyEscape,
			Timestamp: time.Now(),
		}, nil
	}

	// Check for ANSI escape sequence pattern: ESC[X
	if input[1] == '[' {
		switch input[2] {
		case 'A': // Up arrow
			return &InputEvent{
				Type:      terminal.EventKey,
				KeyCode:   terminal.KeyUp,
				Timestamp: time.Now(),
			}, nil
		case 'B': // Down arrow
			return &InputEvent{
				Type:      terminal.EventKey,
				KeyCode:   terminal.KeyDown,
				Timestamp: time.Now(),
			}, nil
		case 'C': // Right arrow
			return &InputEvent{
				Type:      terminal.EventKey,
				KeyCode:   terminal.KeyRight,
				Timestamp: time.Now(),
			}, nil
		case 'D': // Left arrow
			return &InputEvent{
				Type:      terminal.EventKey,
				KeyCode:   terminal.KeyLeft,
				Timestamp: time.Now(),
			}, nil
		case 'H': // Home
			return &InputEvent{
				Type:      terminal.EventKey,
				KeyCode:   terminal.KeyHome,
				Timestamp: time.Now(),
			}, nil
		case 'F': // End
			return &InputEvent{
				Type:      terminal.EventKey,
				KeyCode:   terminal.KeyEnd,
				Timestamp: time.Now(),
			}, nil
		}
	}

	// Unknown escape sequence
	return &InputEvent{
		Type:      terminal.EventSequence,
		Data:      input,
		Timestamp: time.Now(),
	}, nil
}

// getInputHandler gets or creates an input handler for a connection
func (pth *PoolTerminalHandler) getInputHandler(connectionID string, channel ssh.Channel) *terminal.InputHandler {
	pth.handlersMu.RLock()
	handler, exists := pth.inputHandlers[connectionID]
	pth.handlersMu.RUnlock()

	if exists {
		return handler
	}

	// Create new handler
	pth.handlersMu.Lock()
	defer pth.handlersMu.Unlock()

	// Double-check after acquiring write lock
	if handler, exists := pth.inputHandlers[connectionID]; exists {
		return handler
	}

	// Create and store new handler
	handler = terminal.NewInputHandler(channel)
	pth.inputHandlers[connectionID] = handler

	pth.logger.Debug("Created new input handler",
		"connection_id", connectionID)

	return handler
}

// CleanupConnection removes the input handler for a connection
func (pth *PoolTerminalHandler) CleanupConnection(connectionID string) {
	pth.handlersMu.Lock()
	defer pth.handlersMu.Unlock()

	delete(pth.inputHandlers, connectionID)
	
	pth.logger.Debug("Cleaned up input handler",
		"connection_id", connectionID)
}

// GetActiveConnections returns the number of active terminal connections
func (pth *PoolTerminalHandler) GetActiveConnections() int {
	pth.handlersMu.RLock()
	defer pth.handlersMu.RUnlock()
	
	return len(pth.inputHandlers)
}