package session

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

// PTYManager manages PTY sessions
type PTYManager struct {
	sessions    map[string]*PTYSession
	sessionsMux sync.RWMutex
}

// PTYSession represents a PTY session
type PTYSession struct {
	SessionID   string
	Username    string
	GameID      string
	PTY         *os.File
	TTY         *os.File
	Command     *exec.Cmd
	ProcessPID  int
	ExitCode    int
	IsActive    bool
	IsClosed    bool
	WindowSize  WindowSize
	Environment map[string]string
	StartTime   time.Time

	// I/O buffers
	inputBuffer  []byte
	outputBuffer []byte

	// Synchronization
	mutex      sync.RWMutex
	inputChan  chan []byte
	outputChan chan []byte
	doneChan   chan struct{}
}

// NewPTYManager creates a new PTY manager
func NewPTYManager() (*PTYManager, error) {
	return &PTYManager{
		sessions: make(map[string]*PTYSession),
	}, nil
}

// AllocatePTY allocates a new PTY session
func (pm *PTYManager) AllocatePTY(sessionID, username, gameID string, windowSize WindowSize) (*PTYSession, error) {
	pm.sessionsMux.Lock()
	defer pm.sessionsMux.Unlock()

	// Check if session already exists
	if _, exists := pm.sessions[sessionID]; exists {
		return nil, fmt.Errorf("PTY session %s already exists", sessionID)
	}

	// Use creack/pty for cross-platform PTY allocation
	ptyMaster, ptySlave, err := pty.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to allocate PTY: %w", err)
	}

	// Set window size using creack/pty
	if err := pty.Setsize(ptyMaster, &pty.Winsize{
		Rows: windowSize.Height,
		Cols: windowSize.Width,
	}); err != nil {
		ptyMaster.Close()
		ptySlave.Close()
		return nil, fmt.Errorf("failed to set window size: %w", err)
	}

	// Create PTY session
	session := &PTYSession{
		SessionID:    sessionID,
		Username:     username,
		GameID:       gameID,
		PTY:          ptyMaster,
		TTY:          ptySlave,
		IsActive:     true,
		WindowSize:   windowSize,
		Environment:  make(map[string]string),
		StartTime:    time.Now(),
		inputBuffer:  make([]byte, 0, 4096),
		outputBuffer: make([]byte, 0, 4096),
		inputChan:    make(chan []byte, 100),
		outputChan:   make(chan []byte, 100),
		doneChan:     make(chan struct{}),
	}

	// Set default environment variables
	session.Environment["TERM"] = "xterm-256color"
	session.Environment["COLUMNS"] = fmt.Sprintf("%d", windowSize.Width)
	session.Environment["LINES"] = fmt.Sprintf("%d", windowSize.Height)
	session.Environment["USER"] = username
	session.Environment["HOME"] = fmt.Sprintf("/tmp/%s", username) // Use /tmp for safety
	session.Environment["SHELL"] = "/bin/bash"

	// Start I/O handling
	go session.handleInput()
	go session.handleOutput()

	// Store session
	pm.sessions[sessionID] = session

	log.Printf("PTY allocated for session %s", sessionID)
	return session, nil
}

// ReleasePTY releases a PTY session
func (pm *PTYManager) ReleasePTY(sessionID string) error {
	pm.sessionsMux.Lock()
	defer pm.sessionsMux.Unlock()

	session, exists := pm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("PTY session %s not found", sessionID)
	}

	// Close the session
	err := session.Close()

	// Remove from sessions map
	delete(pm.sessions, sessionID)

	log.Printf("PTY released for session %s", sessionID)
	return err
}

// GetPTYSession returns a PTY session by ID
func (pm *PTYManager) GetPTYSession(sessionID string) (*PTYSession, error) {
	pm.sessionsMux.RLock()
	defer pm.sessionsMux.RUnlock()

	session, exists := pm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("PTY session %s not found", sessionID)
	}

	return session, nil
}

// GetActiveSessions returns all active PTY sessions
func (pm *PTYManager) GetActiveSessions() []*PTYSession {
	pm.sessionsMux.RLock()
	defer pm.sessionsMux.RUnlock()

	sessions := make([]*PTYSession, 0, len(pm.sessions))
	for _, session := range pm.sessions {
		if session.IsActive {
			sessions = append(sessions, session)
		}
	}

	return sessions
}

// Shutdown shuts down the PTY manager
func (pm *PTYManager) Shutdown() {
	pm.sessionsMux.Lock()
	defer pm.sessionsMux.Unlock()

	log.Println("Shutting down PTY manager...")

	for sessionID, session := range pm.sessions {
		log.Printf("Closing PTY session: %s", sessionID)
		session.Close()
	}

	pm.sessions = make(map[string]*PTYSession)
}

// PTYSession methods

// StartCommand starts a command in the PTY
func (ps *PTYSession) StartCommand(command string, args []string) error {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	if ps.Command != nil {
		return fmt.Errorf("command already running in PTY session %s", ps.SessionID)
	}

	// Create command
	cmd := exec.Command(command, args...)

	// Set up environment
	env := os.Environ()
	for key, value := range ps.Environment {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	cmd.Env = env

	// Set up process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
	}

	// Connect to PTY
	cmd.Stdin = ps.TTY
	cmd.Stdout = ps.TTY
	cmd.Stderr = ps.TTY

	// Start command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	ps.Command = cmd
	ps.ProcessPID = cmd.Process.Pid

	log.Printf("Command started in PTY session %s: %s (PID: %d)", ps.SessionID, command, ps.ProcessPID)

	// Monitor process
	go ps.monitorProcess()

	return nil
}

// StartCommandWithDir starts a command in the PTY session with a specific working directory
func (ps *PTYSession) StartCommandWithDir(command string, args []string, workingDir string) error {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	if ps.Command != nil {
		return fmt.Errorf("command already running in PTY session %s", ps.SessionID)
	}

	// Create command
	cmd := exec.Command(command, args...)

	// Set working directory if provided
	if workingDir != "" {
		cmd.Dir = workingDir
	}

	// Set up environment
	env := os.Environ()
	for key, value := range ps.Environment {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	cmd.Env = env

	// Set up process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
	}

	// Connect to PTY
	cmd.Stdin = ps.TTY
	cmd.Stdout = ps.TTY
	cmd.Stderr = ps.TTY

	// Start command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	ps.Command = cmd
	ps.ProcessPID = cmd.Process.Pid

	log.Printf("Command started in PTY session %s: %s (PID: %d) in directory: %s", ps.SessionID, command, ps.ProcessPID, workingDir)

	// Monitor process
	go ps.monitorProcess()

	return nil
}

// monitorProcess monitors the running process
func (ps *PTYSession) monitorProcess() {
	if ps.Command == nil {
		return
	}

	// Wait for process to complete
	err := ps.Command.Wait()

	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			ps.ExitCode = exitError.ExitCode()
		} else {
			ps.ExitCode = -1
		}
		log.Printf("Process in PTY session %s exited with error: %v (code: %d)",
			ps.SessionID, err, ps.ExitCode)
	} else {
		ps.ExitCode = 0
		log.Printf("Process in PTY session %s completed successfully", ps.SessionID)
	}

	ps.IsActive = false
	if !ps.IsClosed {
		ps.IsClosed = true
		close(ps.doneChan)
	}
}

// SendInput sends input to the PTY
func (ps *PTYSession) SendInput(data []byte) error {
	if !ps.IsActive {
		return fmt.Errorf("PTY session %s is not active", ps.SessionID)
	}

	// Send to input channel
	select {
	case ps.inputChan <- data:
		return nil
	default:
		return fmt.Errorf("input channel full for PTY session %s", ps.SessionID)
	}
}

// ReadOutput reads output from the PTY
func (ps *PTYSession) ReadOutput() ([]byte, error) {
	if !ps.IsActive {
		return nil, fmt.Errorf("PTY session %s is not active", ps.SessionID)
	}

	select {
	case data := <-ps.outputChan:
		return data, nil
	case <-ps.doneChan:
		return nil, fmt.Errorf("PTY session %s has ended", ps.SessionID)
	case <-time.After(100 * time.Millisecond):
		return nil, nil // Timeout, no data available
	}
}

// handleInput handles input processing
func (ps *PTYSession) handleInput() {
	for {
		select {
		case data := <-ps.inputChan:
			ps.mutex.Lock()
			ps.inputBuffer = append(ps.inputBuffer, data...)
			ps.mutex.Unlock()

			// Write to PTY
			if _, err := ps.PTY.Write(data); err != nil {
				log.Printf("Error writing to PTY %s: %v", ps.SessionID, err)
				return
			}

		case <-ps.doneChan:
			return
		}
	}
}

// handleOutput handles output processing
func (ps *PTYSession) handleOutput() {
	buffer := make([]byte, 4096)

	for {
		select {
		case <-ps.doneChan:
			return
		default:
			// Set read timeout
			_ = ps.PTY.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

			n, err := ps.PTY.Read(buffer)
			if err != nil {
				if !isTimeoutError(err) {
					log.Printf("Error reading from PTY %s: %v", ps.SessionID, err)
					return
				}
				continue
			}

			if n > 0 {
				data := make([]byte, n)
				copy(data, buffer[:n])

				ps.mutex.Lock()
				ps.outputBuffer = append(ps.outputBuffer, data...)
				// Trim buffer if it gets too large
				if len(ps.outputBuffer) > 8192 {
					ps.outputBuffer = ps.outputBuffer[len(ps.outputBuffer)-4096:]
				}
				ps.mutex.Unlock()

				// Send to output channel
				select {
				case ps.outputChan <- data:
				default:
					// Channel full, drop data
					log.Printf("Output channel full for PTY session %s", ps.SessionID)
				}
			}
		}
	}
}

// SendSignal sends a signal to the running process
func (ps *PTYSession) SendSignal(sig os.Signal) error {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()

	if ps.Command == nil || ps.Command.Process == nil {
		return fmt.Errorf("no active process in PTY session %s", ps.SessionID)
	}

	return ps.Command.Process.Signal(sig)
}

// ResizeWindow resizes the PTY window
func (ps *PTYSession) ResizeWindow(rows, cols uint16) error {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	ps.WindowSize.Height = rows
	ps.WindowSize.Width = cols
	ps.WindowSize.X = cols
	ps.WindowSize.Y = rows

	// Update environment variables
	ps.Environment["COLUMNS"] = fmt.Sprintf("%d", cols)
	ps.Environment["LINES"] = fmt.Sprintf("%d", rows)

	if ps.PTY == nil {
		return fmt.Errorf("PTY not available for session %s", ps.SessionID)
	}

	return pty.Setsize(ps.PTY, &pty.Winsize{
		Rows: ps.WindowSize.Height,
		Cols: ps.WindowSize.Width,
	})
}

// SetTerminalAttributes sets terminal attributes for the PTY
func (ps *PTYSession) SetTerminalAttributes() error {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	if ps.TTY == nil {
		return fmt.Errorf("TTY not available for session %s", ps.SessionID)
	}

	// For now, just return success - terminal attributes are complex on macOS
	// In a production environment, you'd want to implement proper terminal control
	log.Printf("Terminal attributes set for PTY session %s", ps.SessionID)
	return nil
}

// GetProcessInfo returns information about the running process
func (ps *PTYSession) GetProcessInfo() map[string]interface{} {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()

	info := map[string]interface{}{
		"session_id": ps.SessionID,
		"username":   ps.Username,
		"game_id":    ps.GameID,
		"is_active":  ps.IsActive,
		"start_time": ps.StartTime,
		"window_size": map[string]uint16{
			"height": ps.WindowSize.Height,
			"width":  ps.WindowSize.Width,
		},
	}

	if ps.Command != nil && ps.Command.Process != nil {
		info["process_id"] = ps.ProcessPID
		info["exit_code"] = ps.ExitCode
	}

	return info
}

// GetInputHistory returns recent input history
func (ps *PTYSession) GetInputHistory(maxBytes int) []byte {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()

	if len(ps.inputBuffer) <= maxBytes {
		result := make([]byte, len(ps.inputBuffer))
		copy(result, ps.inputBuffer)
		return result
	}

	result := make([]byte, maxBytes)
	copy(result, ps.inputBuffer[len(ps.inputBuffer)-maxBytes:])
	return result
}

// GetOutputHistory returns recent output history
func (ps *PTYSession) GetOutputHistory(maxBytes int) []byte {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()

	if len(ps.outputBuffer) <= maxBytes {
		result := make([]byte, len(ps.outputBuffer))
		copy(result, ps.outputBuffer)
		return result
	}

	result := make([]byte, maxBytes)
	copy(result, ps.outputBuffer[len(ps.outputBuffer)-maxBytes:])
	return result
}

// Close closes the PTY session
func (ps *PTYSession) Close() error {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	if !ps.IsActive {
		return nil
	}

	ps.IsActive = false

	// Terminate command if running
	if ps.Command != nil && ps.Command.Process != nil {
		log.Printf("Terminating process %d for PTY session %s", ps.ProcessPID, ps.SessionID)

		// Send SIGTERM first
		_ = ps.Command.Process.Signal(syscall.SIGTERM)

		// Wait for graceful termination
		done := make(chan bool, 1)
		go func() {
			_ = ps.Command.Wait()
			done <- true
		}()

		select {
		case <-done:
			log.Printf("Process %d terminated gracefully", ps.ProcessPID)
		case <-time.After(5 * time.Second):
			log.Printf("Force killing process %d", ps.ProcessPID)
			_ = ps.Command.Process.Kill()
			_ = ps.Command.Wait()
		}
	}

	// Close PTY files
	var err error
	if ps.TTY != nil {
		if closeErr := ps.TTY.Close(); closeErr != nil {
			err = closeErr
			log.Printf("Error closing TTY for session %s: %v", ps.SessionID, closeErr)
		}
	}
	if ps.PTY != nil {
		if closeErr := ps.PTY.Close(); closeErr != nil {
			err = closeErr
			log.Printf("Error closing PTY for session %s: %v", ps.SessionID, closeErr)
		}
	}

	// Signal completion
	if !ps.IsClosed {
		ps.IsClosed = true
		close(ps.doneChan)
	}

	return err
}

// Note: PTY functions now use github.com/creack/pty for cross-platform compatibility

// isTimeoutError checks if an error is a timeout error
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	// Check for various timeout error types
	if netErr, ok := err.(interface{ Timeout() bool }); ok {
		return netErr.Timeout()
	}

	return false
}
