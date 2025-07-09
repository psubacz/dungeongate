package terminal

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"github.com/creack/pty"
	"github.com/dungeongate/internal/session/types"
)

// Manager manages terminal sessions in a stateless manner
type Manager struct {
	logger *slog.Logger
	ptys   sync.Map // map[string]*PTYSession
}

// PTYSession represents a pseudo-terminal session
type PTYSession struct {
	ID      string
	PTY     *os.File
	Command *exec.Cmd
	Rows    int
	Cols    int
	logger  *slog.Logger
}

// NewManager creates a new terminal manager
func NewManager(logger *slog.Logger) *Manager {
	return &Manager{
		logger: logger,
	}
}

// Start starts the terminal manager
func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("Terminal manager starting")
	return nil
}

// Stop stops the terminal manager
func (m *Manager) Stop(ctx context.Context) error {
	m.logger.Info("Terminal manager stopping")

	// Close all PTY sessions
	m.ptys.Range(func(key, value interface{}) bool {
		session := value.(*PTYSession)
		session.Close()
		return true
	})

	return nil
}

// CreatePTY creates a new pseudo-terminal session
func (m *Manager) CreatePTY(sessionID string, command []string, rows, cols int) (*PTYSession, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("command cannot be empty")
	}

	// Create command
	cmd := exec.Command(command[0], command[1:]...)

	// Start command with PTY
	ptyFile, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start PTY: %w", err)
	}

	// Set terminal size
	if err := m.setPtySize(ptyFile, rows, cols); err != nil {
		ptyFile.Close()
		return nil, fmt.Errorf("failed to set PTY size: %w", err)
	}

	session := &PTYSession{
		ID:      sessionID,
		PTY:     ptyFile,
		Command: cmd,
		Rows:    rows,
		Cols:    cols,
		logger:  m.logger,
	}

	// Store session
	m.ptys.Store(sessionID, session)

	m.logger.Info("PTY session created", "session_id", sessionID, "command", command[0])

	return session, nil
}

// GetPTY retrieves a PTY session
func (m *Manager) GetPTY(sessionID string) (*PTYSession, bool) {
	if value, exists := m.ptys.Load(sessionID); exists {
		return value.(*PTYSession), true
	}
	return nil, false
}

// RemovePTY removes a PTY session
func (m *Manager) RemovePTY(sessionID string) {
	if value, exists := m.ptys.LoadAndDelete(sessionID); exists {
		session := value.(*PTYSession)
		session.Close()
		m.logger.Info("PTY session removed", "session_id", sessionID)
	}
}

// ResizePTY resizes a PTY session
func (m *Manager) ResizePTY(sessionID string, rows, cols int) error {
	session, exists := m.GetPTY(sessionID)
	if !exists {
		return fmt.Errorf("PTY session not found: %s", sessionID)
	}

	if err := m.setPtySize(session.PTY, rows, cols); err != nil {
		return fmt.Errorf("failed to resize PTY: %w", err)
	}

	session.Rows = rows
	session.Cols = cols

	m.logger.Debug("PTY session resized", "session_id", sessionID, "rows", rows, "cols", cols)

	return nil
}

// setPtySize sets the size of a PTY
func (m *Manager) setPtySize(pty *os.File, rows, cols int) error {
	ws := &types.Winsize{
		Row: uint16(rows),
		Col: uint16(cols),
	}

	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		pty.Fd(),
		syscall.TIOCSWINSZ,
		uintptr(unsafe.Pointer(ws)),
	)

	if errno != 0 {
		return fmt.Errorf("failed to set PTY size: %v", errno)
	}

	return nil
}

// Close closes the PTY session
func (s *PTYSession) Close() error {
	if s.PTY != nil {
		s.logger.Debug("Closing PTY session", "session_id", s.ID)

		// Close PTY
		s.PTY.Close()

		// Kill process if still running
		if s.Command != nil && s.Command.Process != nil {
			s.Command.Process.Kill()
		}
	}

	return nil
}

// Read reads from the PTY
func (s *PTYSession) Read(p []byte) (int, error) {
	if s.PTY == nil {
		return 0, fmt.Errorf("PTY not available")
	}
	return s.PTY.Read(p)
}

// Write writes to the PTY
func (s *PTYSession) Write(p []byte) (int, error) {
	if s.PTY == nil {
		return 0, fmt.Errorf("PTY not available")
	}
	return s.PTY.Write(p)
}

// Fd returns the file descriptor of the PTY
func (s *PTYSession) Fd() uintptr {
	if s.PTY == nil {
		return 0
	}
	return s.PTY.Fd()
}

// GetStats returns terminal statistics
func (m *Manager) GetStats() *types.TerminalStats {
	stats := &types.TerminalStats{
		ActiveSessions: 0,
		TotalSessions:  0,
		Sessions:       make(map[string]*types.TerminalSessionInfo),
	}

	m.ptys.Range(func(key, value interface{}) bool {
		sessionID := key.(string)
		session := value.(*PTYSession)

		stats.ActiveSessions++
		stats.TotalSessions++

		stats.Sessions[sessionID] = &types.TerminalSessionInfo{
			ID:      session.ID,
			Command: session.Command.Args,
			Rows:    session.Rows,
			Cols:    session.Cols,
			Active:  session.Command.ProcessState == nil || !session.Command.ProcessState.Exited(),
		}

		return true
	})

	return stats
}
