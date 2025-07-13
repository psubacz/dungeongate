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
	SessionID  string
	PTY        *os.File
	Cmd        *exec.Cmd
	Size       *pty.Winsize
	inputChan  chan []byte
	outputChan chan []byte
	errorChan  chan error
	closeChan  chan struct{}
	closeOnce  sync.Once
	mu         sync.Mutex
	adapter    adapters.GameAdapter
	session    *domain.GameSession
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

// CreatePTY creates a new PTY for a game session
func (m *PTYManager) CreatePTY(ctx context.Context, session *domain.GameSession, gamePath string, args []string, env []string) (*PTYSession, error) {
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

	fmt.Printf("DEBUG: Using adapter for game %s: %T\n", gameID, adapter)

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
	
	fmt.Printf("DEBUG: Starting PTY with command: %s %v\n", cmd.Path, cmd.Args)
	fmt.Printf("DEBUG: Working directory: %s\n", cmd.Dir)
	
	// Look for NetHack-specific environment variables
	fmt.Printf("DEBUG: Total environment variables: %d\n", len(cmd.Env))
	for _, env := range cmd.Env {
		if strings.HasPrefix(env, "TERM=") || 
		   strings.HasPrefix(env, "USER=") || 
		   strings.HasPrefix(env, "HOME=") ||
		   strings.HasPrefix(env, "NETHACK") ||
		   strings.HasPrefix(env, "HACKDIR") {
			fmt.Printf("DEBUG: NetHack env: %s\n", env)
		}
	}
	
	// Check if the binary exists
	if _, err := os.Stat(cmd.Path); err != nil {
		fmt.Printf("DEBUG: Binary not found at path %s: %v\n", cmd.Path, err)
		return nil, fmt.Errorf("game binary not found at %s: %w", cmd.Path, err)
	}
	
	// Try standard pty.Start first, which might work better on macOS
	startTime := time.Now()
	ptmx, err := pty.Start(cmd)
	if err != nil {
		fmt.Printf("DEBUG: Failed to start PTY: %v\n", err)
		return nil, fmt.Errorf("failed to start PTY: %w", err)
	}
	fmt.Printf("DEBUG: PTY.Start took %v\n", time.Since(startTime))
	
	// Set the window size after starting
	if err := pty.Setsize(ptmx, size); err != nil {
		fmt.Printf("DEBUG: Warning: Failed to set initial PTY size: %v\n", err)
	}
	
	fmt.Printf("DEBUG: PTY started successfully, PID: %d\n", cmd.Process.Pid)
	
	// Check if process is still alive immediately after starting
	time.Sleep(100 * time.Millisecond)
	// Send signal 0 to check if process exists without affecting it
	err = cmd.Process.Signal(syscall.Signal(0))
	if err != nil {
		fmt.Printf("DEBUG: Process not found after 100ms: %v\n", err)
	} else {
		fmt.Printf("DEBUG: Process still running after 100ms\n")
	}

	// Create PTY session
	ptySession := &PTYSession{
		SessionID: sessionID,
		PTY:       ptmx,
		Cmd:       cmd,
		Size:      size, // Use the size we already created
		inputChan:  make(chan []byte, 100),
		outputChan: make(chan []byte, 100),
		errorChan:  make(chan error, 1),
		closeChan:  make(chan struct{}),
		adapter:    adapter,
		session:    session,
	}

	// Set initial terminal size
	fmt.Printf("DEBUG: Setting terminal size: %dx%d\n", ptySession.Size.Cols, ptySession.Size.Rows)
	if err := pty.Setsize(ptmx, ptySession.Size); err != nil {
		fmt.Printf("DEBUG: Failed to set PTY size: %v\n", err)
		m.logger.Warn("Failed to set initial PTY size", "error", err, "session_id", sessionID)
	}

	// Start I/O handling goroutines
	go ptySession.handleInput()
	go ptySession.handleOutput()
	go ptySession.waitForExit()

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
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("PTY not found for session %s", sessionID)
	}

	// Clean up game environment using adapter
	if err := session.adapter.CleanupGameEnvironment(session.session); err != nil {
		m.logger.Warn("Failed to cleanup game environment", "error", err, "session_id", sessionID)
	}

	// Close the session
	session.Close()

	// Remove from map
	delete(m.sessions, sessionID)

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
				fmt.Printf("DEBUG: PTY Read error for session %s: %v\n", s.SessionID, err)
				select {
				case s.errorChan <- err:
				default:
				}
			}
			return
		}

		if n > 0 {
			fmt.Printf("DEBUG: PTY received %d bytes for session %s: %q\n", n, s.SessionID, string(buffer[:n]))
			rawData := make([]byte, n)
			copy(rawData, buffer[:n])

			// Process output through adapter
			processedData := s.adapter.ProcessOutput(rawData)

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
	fmt.Printf("DEBUG: Waiting for process to exit for session %s, PID: %d\n", s.SessionID, s.Cmd.Process.Pid)
	err := s.Cmd.Wait()
	if err != nil {
		fmt.Printf("DEBUG: Process exited with error for session %s: %v\n", s.SessionID, err)
		// Check if it's an exec.ExitError to get more details
		if exitErr, ok := err.(*exec.ExitError); ok {
			fmt.Printf("DEBUG: Exit code: %d, System: %v\n", exitErr.ExitCode(), exitErr.Sys())
			if exitErr.ExitCode() == -1 {
				fmt.Printf("DEBUG: Process was killed by signal\n")
			}
		}
		select {
		case s.errorChan <- err:
		default:
		}
	} else {
		fmt.Printf("DEBUG: Process exited successfully for session %s\n", s.SessionID)
	}
	s.Close()
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

// Close closes the PTY session
func (s *PTYSession) Close() {
	s.closeOnce.Do(func() {
		close(s.closeChan)

		// Gracefully terminate the process if it's still running
		if s.Cmd != nil && s.Cmd.Process != nil && s.Cmd.ProcessState == nil {
			// Only terminate if process hasn't already exited
			fmt.Printf("DEBUG: Attempting graceful termination of process %d for session %s\n", s.Cmd.Process.Pid, s.SessionID)
			// First try SIGTERM with a grace period
			s.Cmd.Process.Signal(syscall.SIGTERM)
			
			// Give the process 3 seconds to terminate gracefully
			done := make(chan struct{})
			go func() {
				s.Cmd.Wait()
				close(done)
			}()
			
			select {
			case <-done:
				// Process terminated gracefully
				fmt.Printf("DEBUG: Process %d terminated gracefully for session %s\n", s.Cmd.Process.Pid, s.SessionID)
			case <-time.After(3 * time.Second):
				// Process didn't terminate, force kill
				fmt.Printf("DEBUG: Force killing process %d for session %s\n", s.Cmd.Process.Pid, s.SessionID)
				s.Cmd.Process.Signal(syscall.SIGKILL)
			}
		} else if s.Cmd != nil && s.Cmd.ProcessState != nil {
			fmt.Printf("DEBUG: Process already exited for session %s with state: %v\n", s.SessionID, s.Cmd.ProcessState)
		}

		// Close the PTY
		if s.PTY != nil {
			s.PTY.Close()
		}

		// Close channels
		close(s.inputChan)
		close(s.outputChan)
		close(s.errorChan)
	})
}

// GetExitCode returns the exit code of the process
func (s *PTYSession) GetExitCode() (int, error) {
	if s.Cmd.ProcessState == nil {
		return 0, fmt.Errorf("process has not exited")
	}

	return s.Cmd.ProcessState.ExitCode(), nil
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
		fmt.Printf("DEBUG: Sending initial input for session %s: %q\n", s.SessionID, string(initialInput))
		s.SendInput(initialInput)
	}
}
