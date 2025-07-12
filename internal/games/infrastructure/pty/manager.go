package pty

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/creack/pty"
	"github.com/dungeongate/internal/games/domain"
)

// PTYManager manages PTY instances for game sessions
type PTYManager struct {
	sessions map[string]*PTYSession
	mu       sync.RWMutex
	logger   *slog.Logger
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
}

// NewPTYManager creates a new PTY manager
func NewPTYManager(logger *slog.Logger) *PTYManager {
	return &PTYManager{
		sessions: make(map[string]*PTYSession),
		logger:   logger,
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

	// Create command
	cmd := exec.CommandContext(ctx, gamePath, args...)
	cmd.Env = env

	// Set up PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start PTY: %w", err)
	}

	// Create PTY session
	ptySession := &PTYSession{
		SessionID: sessionID,
		PTY:       ptmx,
		Cmd:       cmd,
		Size: &pty.Winsize{
			Rows: uint16(session.TerminalSize().Height),
			Cols: uint16(session.TerminalSize().Width),
		},
		inputChan:  make(chan []byte, 100),
		outputChan: make(chan []byte, 100),
		errorChan:  make(chan error, 1),
		closeChan:  make(chan struct{}),
	}

	// Set initial terminal size
	if err := pty.Setsize(ptmx, ptySession.Size); err != nil {
		m.logger.Warn("Failed to set initial PTY size", "error", err, "session_id", sessionID)
	}

	// Start I/O handling goroutines
	go ptySession.handleInput()
	go ptySession.handleOutput()
	go ptySession.waitForExit()

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
				select {
				case s.errorChan <- err:
				default:
				}
			}
			return
		}

		if n > 0 {
			data := make([]byte, n)
			copy(data, buffer[:n])
			select {
			case s.outputChan <- data:
			case <-s.closeChan:
				return
			}
		}
	}
}

// waitForExit waits for the command to exit
func (s *PTYSession) waitForExit() {
	err := s.Cmd.Wait()
	if err != nil {
		select {
		case s.errorChan <- err:
		default:
		}
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

		// Kill the process if it's still running
		if s.Cmd != nil && s.Cmd.Process != nil {
			s.Cmd.Process.Signal(syscall.SIGTERM)
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
