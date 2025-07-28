package terminal

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

// InputType represents different types of input
type InputType int

const (
	InputTypeText InputType = iota
	InputTypePassword
	InputTypeOptional
)

// KeyCode represents special key codes
type KeyCode int

const (
	KeyNone KeyCode = iota
	KeyBackspace
	KeyDelete
	KeyEnter
	KeyCtrlC
	KeyCtrlD
	KeyCtrlZ
	KeyCtrlU // Clear line
	KeyCtrlK // Kill to end of line
	KeyCtrlA // Beginning of line
	KeyCtrlE // End of line
	KeyLeft
	KeyRight
	KeyUp
	KeyDown
	KeyHome
	KeyEnd
	KeyEscape
)

// InputEvent represents a keyboard input event
type InputEvent struct {
	Type      InputEventType
	Character rune
	KeyCode   KeyCode
	Data      []byte
}

type InputEventType int

const (
	EventCharacter InputEventType = iota
	EventKey
	EventSequence
)

// InputHandler handles terminal input with proper keyboard support
type InputHandler struct {
	channel ssh.Channel
}

// NewInputHandler creates a new input handler
func NewInputHandler(channel ssh.Channel) *InputHandler {
	return &InputHandler{
		channel: channel,
	}
}

// ReadInput reads and parses terminal input
func (h *InputHandler) ReadInput(ctx context.Context) (*InputEvent, error) {
	buffer := make([]byte, 1)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	n, err := h.channel.Read(buffer)
	if err != nil {
		return nil, err
	}

	if n == 0 {
		return &InputEvent{Type: EventCharacter, Character: 0}, nil
	}

	char := buffer[0]

	// Handle control characters
	switch char {
	case 3: // Ctrl+C
		return &InputEvent{Type: EventKey, KeyCode: KeyCtrlC}, nil
	case 4: // Ctrl+D
		return &InputEvent{Type: EventKey, KeyCode: KeyCtrlD}, nil
	case 8, 127: // Backspace/Delete
		return &InputEvent{Type: EventKey, KeyCode: KeyBackspace}, nil
	case 13, 10: // Enter/Return
		return &InputEvent{Type: EventKey, KeyCode: KeyEnter}, nil
	case 21: // Ctrl+U
		return &InputEvent{Type: EventKey, KeyCode: KeyCtrlU}, nil
	case 11: // Ctrl+K
		return &InputEvent{Type: EventKey, KeyCode: KeyCtrlK}, nil
	case 1: // Ctrl+A
		return &InputEvent{Type: EventKey, KeyCode: KeyCtrlA}, nil
	case 5: // Ctrl+E
		return &InputEvent{Type: EventKey, KeyCode: KeyCtrlE}, nil
	case 26: // Ctrl+Z
		return &InputEvent{Type: EventKey, KeyCode: KeyCtrlZ}, nil
	case 27: // ESC - might be start of escape sequence
		return h.handleEscapeSequence(ctx)
	default:
		// Regular character
		if char >= 32 && char <= 126 { // Printable ASCII
			return &InputEvent{Type: EventCharacter, Character: rune(char)}, nil
		}
		// Non-printable character
		return &InputEvent{Type: EventCharacter, Character: rune(char)}, nil
	}
}

// handleEscapeSequence handles ANSI escape sequences for arrow keys, etc.
func (h *InputHandler) handleEscapeSequence(ctx context.Context) (*InputEvent, error) {
	// Read the next character to see if this is an escape sequence
	buffer := make([]byte, 1)

	// Set a short timeout for escape sequence detection
	// In a real implementation, you might want non-blocking reads here
	n, err := h.channel.Read(buffer)
	if err != nil || n == 0 {
		// Just ESC key pressed
		return &InputEvent{Type: EventKey, KeyCode: KeyEscape}, nil
	}

	if buffer[0] == '[' {
		// This is likely an ANSI escape sequence
		return h.handleAnsiSequence(ctx)
	}

	// Not an escape sequence we recognize, treat as ESC
	return &InputEvent{Type: EventKey, KeyCode: KeyEscape}, nil
}

// handleAnsiSequence handles ANSI escape sequences like arrow keys
func (h *InputHandler) handleAnsiSequence(ctx context.Context) (*InputEvent, error) {
	buffer := make([]byte, 1)
	n, err := h.channel.Read(buffer)
	if err != nil || n == 0 {
		return &InputEvent{Type: EventKey, KeyCode: KeyEscape}, nil
	}

	switch buffer[0] {
	case 'A': // Up arrow
		return &InputEvent{Type: EventKey, KeyCode: KeyUp}, nil
	case 'B': // Down arrow
		return &InputEvent{Type: EventKey, KeyCode: KeyDown}, nil
	case 'C': // Right arrow
		return &InputEvent{Type: EventKey, KeyCode: KeyRight}, nil
	case 'D': // Left arrow
		return &InputEvent{Type: EventKey, KeyCode: KeyLeft}, nil
	case 'H': // Home
		return &InputEvent{Type: EventKey, KeyCode: KeyHome}, nil
	case 'F': // End
		return &InputEvent{Type: EventKey, KeyCode: KeyEnd}, nil
	default:
		// Unknown sequence
		return &InputEvent{Type: EventSequence, Data: []byte{27, '[', buffer[0]}}, nil
	}
}

// LineEditor provides line editing capabilities
type LineEditor struct {
	handler   *InputHandler
	inputType InputType
	line      []rune
	cursor    int
}

// NewLineEditor creates a new line editor
func NewLineEditor(channel ssh.Channel, inputType InputType) *LineEditor {
	return &LineEditor{
		handler:   NewInputHandler(channel),
		inputType: inputType,
		line:      make([]rune, 0),
		cursor:    0,
	}
}

// ReadLine reads a complete line with editing support
func (e *LineEditor) ReadLine(ctx context.Context) (string, error) {
	e.line = e.line[:0] // Clear line
	e.cursor = 0

	for {
		event, err := e.handler.ReadInput(ctx)
		if err != nil {
			return "", err
		}

		switch event.Type {
		case EventCharacter:
			if event.Character >= 32 && event.Character <= 126 {
				// Insert character at cursor position
				e.insertCharacter(event.Character)
			}

		case EventKey:
			switch event.KeyCode {
			case KeyEnter:
				// Move to next line and return the input
				e.handler.channel.Write([]byte("\r\n"))
				return string(e.line), nil

			case KeyCtrlC:
				// User wants to cancel
				return "", fmt.Errorf("user cancelled")

			case KeyCtrlD:
				// EOF - if line is empty, treat as cancel
				if len(e.line) == 0 {
					return "", fmt.Errorf("user cancelled")
				}
				// Otherwise, delete character at cursor
				e.deleteCharacter()

			case KeyBackspace:
				e.backspace()

			case KeyCtrlU:
				// Clear entire line
				e.clearLine()

			case KeyCtrlK:
				// Kill to end of line
				e.killToEnd()

			case KeyCtrlA:
				// Move to beginning of line
				e.moveToBeginning()

			case KeyCtrlE:
				// Move to end of line
				e.moveToEnd()

			case KeyLeft:
				e.moveCursorLeft()

			case KeyRight:
				e.moveCursorRight()

			case KeyHome:
				e.moveToBeginning()

			case KeyEnd:
				e.moveToEnd()
			}
		}
	}
}

// insertCharacter inserts a character at the current cursor position
func (e *LineEditor) insertCharacter(ch rune) {
	// Insert character at cursor position
	newLine := make([]rune, len(e.line)+1)
	copy(newLine[:e.cursor], e.line[:e.cursor])
	newLine[e.cursor] = ch
	copy(newLine[e.cursor+1:], e.line[e.cursor:])
	e.line = newLine

	// Echo character (or asterisk for password)
	if e.inputType == InputTypePassword {
		e.handler.channel.Write([]byte("*"))
	} else {
		e.handler.channel.Write([]byte(string(ch)))
	}

	// If we inserted in the middle, redraw the rest of the line
	if e.cursor < len(e.line)-1 {
		e.redrawFromCursor()
	}

	e.cursor++
}

// backspace removes the character before the cursor
func (e *LineEditor) backspace() {
	if e.cursor > 0 {
		// Remove character before cursor
		newLine := make([]rune, len(e.line)-1)
		copy(newLine[:e.cursor-1], e.line[:e.cursor-1])
		copy(newLine[e.cursor-1:], e.line[e.cursor:])
		e.line = newLine
		e.cursor--

		// Move cursor back and redraw
		e.handler.channel.Write([]byte("\b"))
		e.redrawFromCursor()
		e.handler.channel.Write([]byte(" \b")) // Clear the last character
	}
}

// deleteCharacter removes the character at the cursor
func (e *LineEditor) deleteCharacter() {
	if e.cursor < len(e.line) {
		// Remove character at cursor
		newLine := make([]rune, len(e.line)-1)
		copy(newLine[:e.cursor], e.line[:e.cursor])
		copy(newLine[e.cursor:], e.line[e.cursor+1:])
		e.line = newLine

		// Redraw from cursor
		e.redrawFromCursor()
		e.handler.channel.Write([]byte(" \b")) // Clear the last character
	}
}

// clearLine clears the entire line
func (e *LineEditor) clearLine() {
	// Move to beginning
	for e.cursor > 0 {
		e.handler.channel.Write([]byte("\b"))
		e.cursor--
	}

	// Clear the line
	spaces := strings.Repeat(" ", len(e.line))
	e.handler.channel.Write([]byte(spaces))

	// Move back to beginning
	for i := 0; i < len(e.line); i++ {
		e.handler.channel.Write([]byte("\b"))
	}

	e.line = e.line[:0]
	e.cursor = 0
}

// killToEnd removes from cursor to end of line
func (e *LineEditor) killToEnd() {
	if e.cursor < len(e.line) {
		// Clear from cursor to end
		remaining := len(e.line) - e.cursor
		spaces := strings.Repeat(" ", remaining)
		e.handler.channel.Write([]byte(spaces))

		// Move cursor back
		for i := 0; i < remaining; i++ {
			e.handler.channel.Write([]byte("\b"))
		}

		e.line = e.line[:e.cursor]
	}
}

// moveToBeginning moves cursor to beginning of line
func (e *LineEditor) moveToBeginning() {
	for e.cursor > 0 {
		e.handler.channel.Write([]byte("\b"))
		e.cursor--
	}
}

// moveToEnd moves cursor to end of line
func (e *LineEditor) moveToEnd() {
	for e.cursor < len(e.line) {
		if e.inputType == InputTypePassword {
			e.handler.channel.Write([]byte("*"))
		} else {
			e.handler.channel.Write([]byte(string(e.line[e.cursor])))
		}
		e.cursor++
	}
}

// moveCursorLeft moves cursor one position left
func (e *LineEditor) moveCursorLeft() {
	if e.cursor > 0 {
		e.handler.channel.Write([]byte("\b"))
		e.cursor--
	}
}

// moveCursorRight moves cursor one position right
func (e *LineEditor) moveCursorRight() {
	if e.cursor < len(e.line) {
		if e.inputType == InputTypePassword {
			e.handler.channel.Write([]byte("*"))
		} else {
			e.handler.channel.Write([]byte(string(e.line[e.cursor])))
		}
		e.cursor++
	}
}

// redrawFromCursor redraws the line from the current cursor position
func (e *LineEditor) redrawFromCursor() {
	oldCursor := e.cursor

	// Draw from cursor to end
	for i := e.cursor; i < len(e.line); i++ {
		if e.inputType == InputTypePassword {
			e.handler.channel.Write([]byte("*"))
		} else {
			e.handler.channel.Write([]byte(string(e.line[i])))
		}
	}

	// Move cursor back to original position
	for i := len(e.line); i > oldCursor; i-- {
		e.handler.channel.Write([]byte("\b"))
	}
}
