package application

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/dungeongate/internal/games/domain"
	"github.com/dungeongate/internal/games/infrastructure/pty"
	"github.com/google/uuid"
)

// SessionManager handles complete game session lifecycle including save management
type SessionManager struct {
	sessionRepo domain.SessionRepository
	saveRepo    domain.SaveRepository
	gameRepo    domain.GameRepository
	eventRepo   domain.EventRepository
	logger      *slog.Logger
	gameDataDir string
}

// NewSessionManager creates a new session manager
func NewSessionManager(
	sessionRepo domain.SessionRepository,
	saveRepo domain.SaveRepository,
	gameRepo domain.GameRepository,
	eventRepo domain.EventRepository,
	gameDataDir string,
	logger *slog.Logger,
) *SessionManager {
	return &SessionManager{
		sessionRepo: sessionRepo,
		saveRepo:    saveRepo,
		gameRepo:    gameRepo,
		eventRepo:   eventRepo,
		gameDataDir: gameDataDir,
		logger:      logger,
	}
}

// StartGameSession starts a new game session with automatic save loading
func (sm *SessionManager) StartGameSession(ctx context.Context, userID int, gameID string, terminalSize domain.TerminalSize) (*domain.GameSession, error) {
	userIDDomain := domain.NewUserID(userID)
	gameIDDomain := domain.NewGameID(gameID)

	// Find the game
	game, err := sm.gameRepo.FindByID(ctx, gameIDDomain)
	if err != nil {
		return nil, fmt.Errorf("game not found: %w", err)
	}

	// Create session ID
	sessionID := domain.NewSessionID(uuid.New().String())

	// Create session
	session := domain.NewGameSession(
		sessionID,
		userIDDomain,
		"username", // This would come from auth service
		gameIDDomain,
		domain.GameConfig{}, // This would be populated from game config
		terminalSize,
	)

	// Check for existing save
	existingSave, err := sm.saveRepo.FindByUserAndGame(ctx, userIDDomain, gameIDDomain)
	if err == nil && existingSave != nil {
		sm.logger.Info("Found existing save for user", "user_id", userID, "game_id", gameID)
		// Load save data into session working directory
		err = sm.prepareSaveEnvironment(session, existingSave)
		if err != nil {
			sm.logger.Error("Failed to prepare save environment", "error", err)
		}
	}

	// Create session working directory
	sessionDir := filepath.Join(sm.gameDataDir, "sessions", sessionID.String())
	err = os.MkdirAll(sessionDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	// Start the game process
	cmd, err := sm.startGameProcess(game, session, sessionDir)
	if err != nil {
		os.RemoveAll(sessionDir)
		return nil, fmt.Errorf("failed to start game process: %w", err)
	}

	// Update session with process info
	session.Start(domain.ProcessInfo{
		PID:         cmd.Process.Pid,
		ContainerID: "",
		PodName:     "",
	})

	// Save session to database
	err = sm.sessionRepo.Save(ctx, session)
	if err != nil {
		// Kill the process if we can't save the session
		cmd.Process.Kill()
		os.RemoveAll(sessionDir)
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	// Record session start event
	event := &domain.GameEvent{
		ID:        fmt.Sprintf("event_%d", time.Now().UnixNano()),
		Type:      domain.GameEventTypeSessionStart,
		GameID:    gameID,
		SessionID: sessionID.String(),
		UserID:    userID,
		Data: map[string]interface{}{
			"session_dir": sessionDir,
			"pid":         cmd.Process.Pid,
			"has_save":    existingSave != nil,
		},
		Timestamp: time.Now(),
	}
	sm.eventRepo.SaveEvent(ctx, event)

	// Process exit handling will be coordinated through PTY manager callback
	// to avoid race conditions with multiple Wait() calls

	sm.logger.Info("Started game session",
		"session_id", sessionID.String(),
		"user_id", userID,
		"game_id", gameID,
		"pid", cmd.Process.Pid)

	return session, nil
}

// EndGameSession gracefully ends a game session and handles save creation
func (sm *SessionManager) EndGameSession(ctx context.Context, sessionID string) error {
	sessionIDDomain := domain.NewSessionID(sessionID)

	// Find the session
	session, err := sm.sessionRepo.FindByID(ctx, sessionIDDomain)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	// Check if session is active
	if !session.IsActive() {
		return fmt.Errorf("session is not active")
	}

	// Send termination signal to process
	processInfo := session.ProcessInfo()
	if processInfo.PID != 0 {
		process, err := os.FindProcess(processInfo.PID)
		if err == nil {
			// Send SIGTERM first, then SIGKILL if needed
			err = process.Signal(syscall.SIGTERM)
			if err != nil {
				sm.logger.Error("Failed to send SIGTERM to process", "pid", processInfo.PID, "error", err)
				// Try SIGKILL
				process.Signal(syscall.SIGKILL)
			}
		}
	}

	// Create save from session data
	err = sm.createSaveFromSession(ctx, session)
	if err != nil {
		sm.logger.Error("Failed to create save from session", "session_id", sessionID, "error", err)
	}

	// Mark session as ended
	session.End(nil, nil)

	// Update session in database
	err = sm.sessionRepo.Save(ctx, session)
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	// Record session end event
	event := &domain.GameEvent{
		ID:        fmt.Sprintf("event_%d", time.Now().UnixNano()),
		Type:      domain.GameEventTypeSessionEnd,
		GameID:    session.GameID().String(),
		SessionID: sessionID,
		UserID:    session.UserID().Int(),
		Data: map[string]interface{}{
			"duration_seconds": session.Duration().Seconds(),
			"save_created":     true,
		},
		Timestamp: time.Now(),
	}
	sm.eventRepo.SaveEvent(ctx, event)

	sm.logger.Info("Ended game session", "session_id", sessionID)

	return nil
}

// prepareSaveEnvironment copies save data to session working directory
func (sm *SessionManager) prepareSaveEnvironment(session *domain.GameSession, save *domain.GameSave) error {
	sessionDir := filepath.Join(sm.gameDataDir, "sessions", session.ID().String())
	saveDir := filepath.Join(sessionDir, "save")

	// Create save directory
	err := os.MkdirAll(saveDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create save directory: %w", err)
	}

	// Copy save file to session directory
	if save.FilePath() != "" {
		sessionSavePath := filepath.Join(saveDir, "save.dat")
		err = sm.copyFile(save.FilePath(), sessionSavePath)
		if err != nil {
			return fmt.Errorf("failed to copy save file: %w", err)
		}
	}

	return nil
}

// startGameProcess starts the game executable
func (sm *SessionManager) startGameProcess(game *domain.Game, session *domain.GameSession, sessionDir string) (*exec.Cmd, error) {
	// This is a simplified implementation
	// In reality, you'd use the game's binary path, args, working directory, etc.

	gameExecutable := "/opt/homebrew/bin/nethack" // Default for now
	if game != nil {
		// Use game-specific executable path from game domain model
		gameExecutable = "/opt/homebrew/bin/nethack" // Would come from game.BinaryPath()
	}

	cmd := exec.Command(gameExecutable)
	cmd.Dir = sessionDir

	// Set up environment
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("COLUMNS=%d", session.TerminalSize().Width))
	cmd.Env = append(cmd.Env, fmt.Sprintf("LINES=%d", session.TerminalSize().Height))
	cmd.Env = append(cmd.Env, fmt.Sprintf("SESSION_ID=%s", session.ID().String()))

	// Start the process
	err := cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start game process: %w", err)
	}

	return cmd, nil
}

// handleGameProcessExit handles cleanup when a game process exits (called via PTY manager callback)
func (sm *SessionManager) handleGameProcessExit(ctx context.Context, session *domain.GameSession, exitCode *int, processErr error) {
	var signal *string

	// Extract signal information if available
	if processErr != nil {
		if exitError, ok := processErr.(*exec.ExitError); ok {
			if sys := exitError.Sys(); sys != nil {
				// On Unix systems, extract signal information
				if ws, ok := sys.(syscall.WaitStatus); ok && ws.Signaled() {
					sig := ws.Signal().String()
					signal = &sig
				}
			}
		}
	}

	sm.logger.Info("Game process exited", "session_id", session.ID().String(), "exit_code", exitCode)

	// Create save from session data before marking as ended
	err := sm.createSaveFromSession(ctx, session)
	if err != nil {
		sm.logger.Error("Failed to create save from session", "session_id", session.ID().String(), "error", err)
	}

	// Mark session as ended
	session.End(exitCode, signal)

	// Update session in database
	err = sm.sessionRepo.Save(ctx, session)
	if err != nil {
		sm.logger.Error("Failed to update session after process exit", "error", err)
	}

	// Record process exit event
	event := &domain.GameEvent{
		ID:        fmt.Sprintf("event_%d", time.Now().UnixNano()),
		Type:      domain.GameEventTypeSessionEnd,
		GameID:    session.GameID().String(),
		SessionID: session.ID().String(),
		UserID:    session.UserID().Int(),
		Data: map[string]interface{}{
			"exit_code":        exitCode,
			"duration_seconds": session.Duration().Seconds(),
		},
		Timestamp: time.Now(),
	}
	sm.eventRepo.SaveEvent(ctx, event)
}

// GetProcessExitCallback returns a callback function for handling process exit
func (sm *SessionManager) GetProcessExitCallback(ctx context.Context) pty.ProcessExitCallback {
	return func(session *domain.GameSession, exitCode *int, err error) {
		sm.handleGameProcessExit(ctx, session, exitCode, err)
	}
}

// createSaveFromSession creates a save file from session data
func (sm *SessionManager) createSaveFromSession(ctx context.Context, session *domain.GameSession) error {
	sessionDir := filepath.Join(sm.gameDataDir, "sessions", session.ID().String())
	saveFile := filepath.Join(sessionDir, "save", "save.dat")

	// Check if save file exists
	if _, err := os.Stat(saveFile); os.IsNotExist(err) {
		// No save file found
		return nil
	}

	// Read save file data
	saveData, err := os.ReadFile(saveFile)
	if err != nil {
		return fmt.Errorf("failed to read save file: %w", err)
	}

	// Create permanent save directory
	userSaveDir := filepath.Join(sm.gameDataDir, "saves",
		fmt.Sprintf("user_%d", session.UserID().Int()),
		session.GameID().String())
	err = os.MkdirAll(userSaveDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create user save directory: %w", err)
	}

	// Copy save to permanent location
	permanentSavePath := filepath.Join(userSaveDir, fmt.Sprintf("save_%s.dat", session.ID().String()))
	err = os.WriteFile(permanentSavePath, saveData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write permanent save file: %w", err)
	}

	// Create save domain object
	saveID := domain.NewSaveID(uuid.New().String())
	save := domain.NewGameSave(
		saveID,
		session.UserID(),
		session.GameID(),
		saveData,
		permanentSavePath,
		domain.SaveMetadata{
			GameVersion: "1.0",    // Would come from game
			Character:   "Player", // Would parse from save data
			Level:       1,        // Would parse from save data
			Score:       0,        // Would parse from save data
			PlayTime:    session.Duration(),
			Location:    "Dungeon", // Would parse from save data
		},
	)

	// Save to database
	err = sm.saveRepo.Save(ctx, save)
	if err != nil {
		return fmt.Errorf("failed to save to database: %w", err)
	}

	sm.logger.Info("Created save for session", "save_id", saveID.String(), "session_id", session.ID().String())

	return nil
}

// copyFile copies a file from src to dst
func (sm *SessionManager) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Create destination directory if it doesn't exist
	destDir := filepath.Dir(dst)
	err = os.MkdirAll(destDir, 0755)
	if err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	// Copy file contents
	_, err = destFile.ReadFrom(sourceFile)
	return err
}
