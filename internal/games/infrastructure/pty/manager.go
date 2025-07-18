package pty

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/dungeongate/internal/games"
	"github.com/dungeongate/internal/games/adapters"
	"github.com/dungeongate/internal/games/domain"
)

// PTYManager manages PTY instances for game sessions
type PTYManager struct {
	sessions map[string]*PTYSession
	mu       sync.RWMutex
	logger   *slog.Logger
	adapters *adapters.GameAdapterRegistry
}

// PTYSession represents a PTY session for a game
type PTYSession struct {
	SessionID     string
	PTY           *os.File
	Cmd           *exec.Cmd
	Size          *pty.Winsize
	inputChan     chan []byte
	outputChan    chan []byte
	errorChan     chan error
	closeChan     chan struct{}
	closeOnce     sync.Once
	mu            sync.Mutex
	adapter       adapters.GameAdapter
	session       *domain.GameSession
	onExit        ProcessExitCallback
	logger        *slog.Logger
	streamManager *games.StreamManager

	// Output subscribers for direct PTY streaming
	outputSubscribers map[string]chan []byte
	subscribersMu     sync.RWMutex
}

// NewPTYManager creates a new PTY manager
func NewPTYManager(logger *slog.Logger) *PTYManager {
	return &PTYManager{
		sessions: make(map[string]*PTYSession),
		logger:   logger,
		adapters: adapters.NewGameAdapterRegistry(),
	}
}

// NewPTYManagerWithAdapters creates a new PTY manager with configured adapters
func NewPTYManagerWithAdapters(logger *slog.Logger, adapterRegistry *adapters.GameAdapterRegistry) *PTYManager {
	return &PTYManager{
		sessions: make(map[string]*PTYSession),
		logger:   logger,
		adapters: adapterRegistry,
	}
}

// ProcessExitCallback is called when a game process exits
type ProcessExitCallback func(session *domain.GameSession, exitCode *int, err error)

// CreatePTY creates a new PTY for a game session
func (m *PTYManager) CreatePTY(ctx context.Context, session *domain.GameSession, gamePath string, args []string, env []string) (*PTYSession, error) {
	return m.CreatePTYWithCallback(ctx, session, gamePath, args, env, nil)
}

// CreatePTYWithCallback creates a new PTY for a game session with a callback for process exit
func (m *PTYManager) CreatePTYWithCallback(ctx context.Context, session *domain.GameSession, gamePath string, args []string, env []string, onExit ProcessExitCallback) (*PTYSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessionID := session.ID().String()

	// Check if session already exists
	if _, exists := m.sessions[sessionID]; exists {
		return nil, fmt.Errorf("PTY already exists for session %s", sessionID)
	}

	// Get the appropriate game adapter
	gameID := session.GameID().String()
	adapter := m.adapters.GetAdapter(gameID)

	m.logger.Debug("Using adapter for game", "game_id", gameID, "adapter_type", fmt.Sprintf("%T", adapter))

	// Setup game environment using adapter
	if err := adapter.SetupGameEnvironment(session); err != nil {
		return nil, fmt.Errorf("failed to setup game environment: %w", err)
	}

	// Create command using adapter
	cmd, err := adapter.PrepareCommand(ctx, session, gamePath, args, env)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare command: %w", err)
	}

	// Set up PTY with enhanced terminal attributes
	size := &pty.Winsize{
		Rows: uint16(session.TerminalSize().Height),
		Cols: uint16(session.TerminalSize().Width),
	}

	// Note: Using standard pty.Start instead of StartWithAttrs
	// as it works better with NetHack on macOS

	m.logger.Debug("Starting PTY with command", "path", cmd.Path, "args", cmd.Args)
	m.logger.Debug("Working directory", "dir", cmd.Dir)

	// Look for NetHack-specific environment variables
	m.logger.Debug("Total environment variables", "count", len(cmd.Env))
	for _, env := range cmd.Env {
		if strings.HasPrefix(env, "TERM=") ||
			strings.HasPrefix(env, "USER=") ||
			strings.HasPrefix(env, "HOME=") ||
			strings.HasPrefix(env, "NETHACK") ||
			strings.HasPrefix(env, "HACKDIR") {
			m.logger.Debug("NetHack env", "variable", env)
		}
	}

	// Check if the binary exists
	if _, err := os.Stat(cmd.Path); err != nil {
		m.logger.Debug("Binary not found at path", "path", cmd.Path, "error", err)
		return nil, fmt.Errorf("game binary not found at %s: %w", cmd.Path, err)
	}

	// Try standard pty.Start first, which might work better on macOS
	startTime := time.Now()
	ptmx, err := pty.Start(cmd)
	if err != nil {
		m.logger.Error("Failed to start PTY", "error", err)
		return nil, fmt.Errorf("failed to start PTY: %w", err)
	}
	m.logger.Debug("PTY.Start took", "duration", time.Since(startTime))

	// Set the window size after starting
	if err := pty.Setsize(ptmx, size); err != nil {
		m.logger.Warn("Failed to set initial PTY size", "error", err)
	}

	m.logger.Debug("PTY started successfully", "pid", cmd.Process.Pid)

	// Check if process is still alive immediately after starting
	time.Sleep(100 * time.Millisecond)
	// Send signal 0 to check if process exists without affecting it
	err = cmd.Process.Signal(syscall.Signal(0))
	if err != nil {
		m.logger.Debug("Process not found after 100ms", "error", err)
	} else {
		m.logger.Debug("Process still running after 100ms")
	}

	// Create PTY session
	ptySession := &PTYSession{
		SessionID:         sessionID,
		PTY:               ptmx,
		Cmd:               cmd,
		Size:              size, // Use the size we already created
		inputChan:         make(chan []byte, 100),
		outputChan:        make(chan []byte, 100),
		errorChan:         make(chan error, 1),
		closeChan:         make(chan struct{}),
		adapter:           adapter,
		session:           session,
		onExit:            onExit,
		logger:            m.logger.With(slog.String("session_id", sessionID)),
		streamManager:     games.NewStreamManagerWithSize(int(size.Rows), int(size.Cols)),
		outputSubscribers: make(map[string]chan []byte),
	}

	// Set initial terminal size
	m.logger.Debug("Setting terminal size", "cols", ptySession.Size.Cols, "rows", ptySession.Size.Rows)
	if err := pty.Setsize(ptmx, ptySession.Size); err != nil {
		m.logger.Warn("Failed to set initial PTY size", "error", err, "session_id", sessionID)
	}

	// Start I/O handling goroutines
	go ptySession.handleInput()
	go ptySession.handleOutput()
	go ptySession.waitForExit()

	// Start the stream manager
	if ptySession.streamManager != nil {
		ptySession.streamManager.Start()
	}

	// Send initial input if adapter provides it
	go ptySession.sendInitialInput()

	// Store session
	m.sessions[sessionID] = ptySession

	m.logger.Info("Created PTY for session", "session_id", sessionID, "game_path", gamePath)

	return ptySession, nil
}

// GetPTY returns a PTY session by ID
func (m *PTYManager) GetPTY(sessionID string) (*PTYSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("PTY not found for session %s", sessionID)
	}

	return session, nil
}

// ClosePTY closes a PTY session
func (m *PTYManager) ClosePTY(sessionID string) error {
	m.logger.Debug("ClosePTY called for session", "session_id", sessionID)
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		m.logger.Debug("ClosePTY: PTY not found for session", "session_id", sessionID)
		return fmt.Errorf("PTY not found for session %s", sessionID)
	}

	m.logger.Debug("ClosePTY: Found PTY session", "session_id", sessionID, "pid", session.Cmd.Process.Pid)

	// Clean up game environment using adapter
	if err := session.adapter.CleanupGameEnvironment(session.session); err != nil {
		m.logger.Warn("Failed to cleanup game environment", "error", err, "session_id", sessionID)
	}

	// Close the session
	m.logger.Debug("ClosePTY: About to call session.Close()", "session_id", sessionID)
	session.Close()

	// Remove from map
	delete(m.sessions, sessionID)

	m.logger.Debug("ClosePTY completed for session", "session_id", sessionID)
	m.logger.Info("Closed PTY for session", "session_id", sessionID)

	return nil
}

// ResizePTY resizes a PTY
func (m *PTYManager) ResizePTY(sessionID string, rows, cols uint16) error {
	session, err := m.GetPTY(sessionID)
	if err != nil {
		return err
	}

	return session.Resize(rows, cols)
}

// handleInput reads from inputChan and writes to PTY
func (s *PTYSession) handleInput() {
	for {
		select {
		case data := <-s.inputChan:
			if _, err := s.PTY.Write(data); err != nil {
				select {
				case s.errorChan <- err:
				default:
				}
				return
			}
		case <-s.closeChan:
			return
		}
	}
}

// handleOutput reads from PTY and writes to outputChan
func (s *PTYSession) handleOutput() {
	buffer := make([]byte, 4096)
	for {
		n, err := s.PTY.Read(buffer)
		if err != nil {
			if err != io.EOF {
				s.logger.Debug("PTY Read error for session", "session_id", s.SessionID, "error", err)
				select {
				case s.errorChan <- err:
				default:
				}
			}
			return
		}

		if n > 0 {
			s.logger.Debug("PTY received bytes for session", "session_id", s.SessionID, "bytes", n, "data", string(buffer[:n]))
			rawData := make([]byte, n)
			copy(rawData, buffer[:n])

			// Process output through adapter
			processedData := s.adapter.ProcessOutput(rawData)

			// Send to stream manager for spectating (non-blocking)
			// This ensures spectators don't interfere with player performance
			if s.streamManager != nil {
				// Create a copy of the data to avoid race conditions with buffer reuse
				streamData := make([]byte, len(processedData))
				copy(streamData, processedData)
				go func(data []byte) {
					s.streamManager.SendFrame(data)
				}(streamData)
			}

			// Broadcast to all output subscribers (player connections)
			s.subscribersMu.RLock()
			for subscriptionID, outputChan := range s.outputSubscribers {
				select {
				case outputChan <- processedData:
					// Successfully sent to subscriber
				default:
					// Channel is full, skip this subscriber to avoid blocking
					s.logger.Warn("Output subscriber channel full, skipping", "subscription_id", subscriptionID)
				}
			}
			s.subscribersMu.RUnlock()

			// Send to legacy output channel for backward compatibility
			select {
			case s.outputChan <- processedData:
			case <-s.closeChan:
				return
			}
		}
	}
}

// waitForExit waits for the command to exit
func (s *PTYSession) waitForExit() {
	s.logger.Debug("STARTING waitForExit for session", "session_id", s.SessionID, "pid", s.Cmd.Process.Pid)

	// Check process status before waiting
	err := s.Cmd.Process.Signal(syscall.Signal(0))
	if err != nil {
		s.logger.Error("CRITICAL: Process already dead before Wait()", "pid", s.Cmd.Process.Pid, "error", err)
	} else {
		s.logger.Debug("Process confirmed alive and healthy before Wait()", "pid", s.Cmd.Process.Pid)
	}

	// Add a small delay to see if the process gets killed immediately
	s.logger.Debug("Waiting 1 second to see if process stays alive...")
	time.Sleep(1 * time.Second)

	// Check again after delay
	err = s.Cmd.Process.Signal(syscall.Signal(0))
	if err != nil {
		s.logger.Error("CRITICAL: Process died during 1-second wait", "pid", s.Cmd.Process.Pid, "error", err)
	} else {
		s.logger.Debug("Process still alive after 1-second delay", "pid", s.Cmd.Process.Pid)
	}

	s.logger.Debug("About to call Cmd.Wait() for session", "session_id", s.SessionID, "pid", s.Cmd.Process.Pid)
	err = s.Cmd.Wait()
	s.logger.Debug("Cmd.Wait() returned for session", "session_id", s.SessionID, "pid", s.Cmd.Process.Pid, "error", err)

	var exitCode *int
	if err != nil {
		s.logger.Error("CRITICAL: Process exited with error for session", "session_id", s.SessionID, "error", err)
		// Check if it's an exec.ExitError to get more details
		if exitErr, ok := err.(*exec.ExitError); ok {
			s.logger.Debug("Exit code", "code", exitErr.ExitCode(), "system", exitErr.Sys())
			if exitErr.ExitCode() == -1 {
				s.logger.Error("CRITICAL: Process was killed by external signal (SIGKILL)")
			}
			code := exitErr.ExitCode()
			exitCode = &code
		}
		select {
		case s.errorChan <- err:
		default:
		}
	} else {
		s.logger.Debug("Process exited successfully for session", "session_id", s.SessionID)
		code := 0
		exitCode = &code
	}

	// Call the exit callback if provided
	if s.onExit != nil {
		s.logger.Debug("Calling onExit callback for session", "session_id", s.SessionID)
		s.onExit(s.session, exitCode, err)
	}

	s.logger.Debug("waitForExit completed for session", "session_id", s.SessionID, "process_state", s.Cmd.ProcessState)
}

// SendInput sends input to the PTY
func (s *PTYSession) SendInput(data []byte) error {
	select {
	case s.inputChan <- data:
		return nil
	case <-s.closeChan:
		return fmt.Errorf("PTY session is closed")
	}
}

// GetOutput returns the output channel
func (s *PTYSession) GetOutput() <-chan []byte {
	return s.outputChan
}

// GetError returns the error channel
func (s *PTYSession) GetError() <-chan error {
	return s.errorChan
}

// Resize resizes the PTY
func (s *PTYSession) Resize(rows, cols uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Size.Rows = rows
	s.Size.Cols = cols
	return pty.Setsize(s.PTY, s.Size)
}

// Close closes the PTY session WITHOUT terminating the process
// This allows games like NetHack to keep running for reconnection
func (s *PTYSession) Close() {
	s.logger.Debug("Close() called for session", "session_id", s.SessionID)

	s.closeOnce.Do(func() {
		s.logger.Debug("Executing Close() for session (first time)", "session_id", s.SessionID)
		close(s.closeChan)

		// FOR INTERACTIVE GAMES LIKE NETHACK: Do NOT terminate the process
		// The process should continue running even if streams disconnect
		// This enables reconnection and session persistence
		if s.Cmd != nil && s.Cmd.Process != nil && s.Cmd.ProcessState == nil {
			s.logger.Debug("Keeping process alive for session (interactive game)", "pid", s.Cmd.Process.Pid, "session_id", s.SessionID)
			s.logger.Debug("Process will continue running for potential reconnection")
		} else if s.Cmd != nil && s.Cmd.ProcessState != nil {
			s.logger.Debug("Process already exited for session", "session_id", s.SessionID, "state", s.Cmd.ProcessState)
		}

		// Close channels but do NOT close the PTY file descriptor yet
		// Closing the PTY would cause NetHack to exit due to lost terminal
		close(s.inputChan)
		close(s.outputChan)
		close(s.errorChan)

		// Stop the stream manager
		if s.streamManager != nil {
			s.streamManager.Stop()
		}

		s.logger.Debug("Close() completed for session - process and PTY kept alive", "session_id", s.SessionID)
	})
}

// ForceTerminate forcefully terminates the game process (for explicit user quit)
func (s *PTYSession) ForceTerminate() {
	s.logger.Debug("ForceTerminate() called for session", "session_id", s.SessionID)

	if s.Cmd != nil && s.Cmd.Process != nil && s.Cmd.ProcessState == nil {
		s.logger.Debug("Force terminating process for session", "pid", s.Cmd.Process.Pid, "session_id", s.SessionID)

		// Send SIGTERM first
		s.Cmd.Process.Signal(syscall.SIGTERM)

		// Give it 5 seconds to exit gracefully
		time.Sleep(5 * time.Second)

		// Force kill if still running
		if s.Cmd.ProcessState == nil {
			s.logger.Debug("Sending SIGKILL to process for session", "pid", s.Cmd.Process.Pid, "session_id", s.SessionID)
			s.Cmd.Process.Signal(syscall.SIGKILL)
		}
	}

	// Now close the PTY
	if s.PTY != nil {
		s.PTY.Close()
	}
}

// GetExitCode returns the exit code of the process
func (s *PTYSession) GetExitCode() (int, error) {
	if s.Cmd.ProcessState == nil {
		return 0, fmt.Errorf("process has not exited")
	}

	return s.Cmd.ProcessState.ExitCode(), nil
}

// GetStreamManager returns the stream manager for this PTY session
func (s *PTYSession) GetStreamManager() *games.StreamManager {
	return s.streamManager
}

// SubscribeToOutput subscribes to PTY output with a unique subscription ID
func (s *PTYSession) SubscribeToOutput(subscriptionID string) <-chan []byte {
	s.subscribersMu.Lock()
	defer s.subscribersMu.Unlock()

	outputChan := make(chan []byte, 100)
	s.outputSubscribers[subscriptionID] = outputChan
	s.logger.Debug("Added output subscriber", "subscription_id", subscriptionID, "total_subscribers", len(s.outputSubscribers))
	return outputChan
}

// UnsubscribeFromOutput removes a subscription to PTY output
func (s *PTYSession) UnsubscribeFromOutput(subscriptionID string) {
	s.subscribersMu.Lock()
	defer s.subscribersMu.Unlock()

	if outputChan, exists := s.outputSubscribers[subscriptionID]; exists {
		close(outputChan)
		delete(s.outputSubscribers, subscriptionID)
		s.logger.Debug("Removed output subscriber", "subscription_id", subscriptionID, "total_subscribers", len(s.outputSubscribers))
	}
}

// sendInitialInput sends any initial input required by the game
func (s *PTYSession) sendInitialInput() {
	// Wait a moment for the game to start
	select {
	case <-s.closeChan:
		return
	case <-time.After(time.Millisecond * 500): // Give game 500ms to start
	}

	// Get initial input from adapter
	if initialInput := s.adapter.GetInitialInput(); initialInput != nil {
		s.logger.Debug("Sending initial input for session", "session_id", s.SessionID, "input", string(initialInput))
		s.SendInput(initialInput)
	}
}
