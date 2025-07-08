package application

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/dungeongate/internal/games/domain"
)

// SessionService provides application-level session management operations
type SessionService struct {
	sessionRepo domain.SessionRepository
	gameRepo    domain.GameRepository
	saveRepo    domain.SaveRepository
	eventRepo   domain.EventRepository
	uow         domain.UnitOfWork
}

// NewSessionService creates a new session service
func NewSessionService(
	sessionRepo domain.SessionRepository,
	gameRepo domain.GameRepository,
	saveRepo domain.SaveRepository,
	eventRepo domain.EventRepository,
	uow domain.UnitOfWork,
) *SessionService {
	return &SessionService{
		sessionRepo: sessionRepo,
		gameRepo:    gameRepo,
		saveRepo:    saveRepo,
		eventRepo:   eventRepo,
		uow:         uow,
	}
}

// StartGameSession starts a new game session
func (s *SessionService) StartGameSession(ctx context.Context, req *StartSessionRequest) (*domain.GameSession, error) {
	// Validate request
	if err := s.validateStartSessionRequest(req); err != nil {
		return nil, fmt.Errorf("invalid start session request: %w", err)
	}

	// Get game configuration
	gameID := domain.NewGameID(req.GameID)
	game, err := s.gameRepo.FindByID(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("game not found: %w", err)
	}

	if !game.CanStart() {
		return nil, fmt.Errorf("game %s is not available for play", req.GameID)
	}

	// Check if user already has an active session for this game
	userID := domain.NewUserID(req.UserID)
	activeSessions, err := s.sessionRepo.FindActiveByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check active sessions: %w", err)
	}

	for _, session := range activeSessions {
		if session.GameID() == gameID {
			return nil, fmt.Errorf("user already has an active session for game %s", req.GameID)
		}
	}

	// Begin transaction
	if err := s.uow.Begin(ctx); err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			s.uow.Rollback(ctx)
		}
	}()

	// Create session
	sessionID := domain.NewSessionID(generateSessionID())
	terminalSize := domain.TerminalSize{
		Width:  req.TerminalWidth,
		Height: req.TerminalHeight,
	}

	session := domain.NewGameSession(
		sessionID,
		userID,
		req.Username,
		gameID,
		game.Config(),
		terminalSize,
	)

	// Enable recording if requested
	if req.EnableRecording {
		recordingPath := fmt.Sprintf("/var/lib/dungeongate/recordings/%s.ttyrec", sessionID.String())
		session.EnableRecording(recordingPath, "ttyrec")
	}

	// Enable streaming if requested
	if req.EnableStreaming {
		session.EnableStreaming("grpc", req.EnableEncryption)
	}

	// Start the game process
	processInfo, err := s.startGameProcess(ctx, session, game)
	if err != nil {
		return nil, fmt.Errorf("failed to start game process: %w", err)
	}

	session.Start(processInfo)

	// Save session
	if err := s.uow.Sessions().Save(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	// Update game statistics
	game.IncrementPlayCount()
	if err := s.uow.Games().Save(ctx, game); err != nil {
		return nil, fmt.Errorf("failed to update game statistics: %w", err)
	}

	// Publish session start event
	event := &domain.GameEvent{
		ID:        generateEventID(),
		Type:      domain.GameEventTypeSessionStart,
		GameID:    req.GameID,
		SessionID: sessionID.String(),
		UserID:    req.UserID,
		Data: map[string]interface{}{
			"username":      req.Username,
			"terminal_size": fmt.Sprintf("%dx%d", req.TerminalWidth, req.TerminalHeight),
		},
		Timestamp: time.Now(),
	}

	if err := s.uow.Events().SaveEvent(ctx, event); err != nil {
		// Log but don't fail the session start
		// log error
	}

	// Commit transaction
	if err := s.uow.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return session, nil
}

// StopGameSession stops a game session
func (s *SessionService) StopGameSession(ctx context.Context, sessionID string, reason string) error {
	id := domain.NewSessionID(sessionID)

	// Find the session
	session, err := s.sessionRepo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	if !session.IsActive() {
		return fmt.Errorf("session is not active")
	}

	// Begin transaction
	if err := s.uow.Begin(ctx); err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			s.uow.Rollback(ctx)
		}
	}()

	// Stop the game process
	if err := s.stopGameProcess(ctx, session); err != nil {
		// Log error but continue with session cleanup
	}

	// End the session
	session.End(nil, nil)

	// Save session
	if err := s.uow.Sessions().Save(ctx, session); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	// Publish session end event
	event := &domain.GameEvent{
		ID:        generateEventID(),
		Type:      domain.GameEventTypeSessionEnd,
		GameID:    session.GameID().String(),
		SessionID: sessionID,
		UserID:    session.UserID().Int(),
		Data: map[string]interface{}{
			"reason":   reason,
			"duration": session.Duration().String(),
		},
		Timestamp: time.Now(),
	}

	if err := s.uow.Events().SaveEvent(ctx, event); err != nil {
		// Log but don't fail
	}

	// Commit transaction
	return s.uow.Commit(ctx)
}

// GetGameSession retrieves a game session
func (s *SessionService) GetGameSession(ctx context.Context, sessionID string) (*domain.GameSession, error) {
	id := domain.NewSessionID(sessionID)
	return s.sessionRepo.FindByID(ctx, id)
}

// ListActiveSessions lists all active sessions
func (s *SessionService) ListActiveSessions(ctx context.Context) ([]*domain.GameSession, error) {
	return s.sessionRepo.FindActive(ctx)
}

// ListUserSessions lists sessions for a specific user
func (s *SessionService) ListUserSessions(ctx context.Context, userID int) ([]*domain.GameSession, error) {
	id := domain.NewUserID(userID)
	return s.sessionRepo.FindByUserID(ctx, id)
}

// AddSpectator adds a spectator to a session
func (s *SessionService) AddSpectator(ctx context.Context, sessionID string, spectatorUserID int, spectatorUsername string) error {
	id := domain.NewSessionID(sessionID)

	session, err := s.sessionRepo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	spectatorID := domain.NewUserID(spectatorUserID)
	if err := session.AddSpectator(spectatorID, spectatorUsername); err != nil {
		return err
	}

	// Save session
	if err := s.sessionRepo.Save(ctx, session); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	// Publish spectator join event
	event := &domain.GameEvent{
		ID:        generateEventID(),
		Type:      domain.GameEventTypeSpectatorJoin,
		GameID:    session.GameID().String(),
		SessionID: sessionID,
		UserID:    spectatorUserID,
		Data: map[string]interface{}{
			"spectator_username": spectatorUsername,
		},
		Timestamp: time.Now(),
	}

	s.eventRepo.SaveEvent(ctx, event)

	return nil
}

// RemoveSpectator removes a spectator from a session
func (s *SessionService) RemoveSpectator(ctx context.Context, sessionID string, spectatorUserID int) error {
	id := domain.NewSessionID(sessionID)

	session, err := s.sessionRepo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	spectatorID := domain.NewUserID(spectatorUserID)
	session.RemoveSpectator(spectatorID)

	return s.sessionRepo.Save(ctx, session)
}

// startGameProcess starts the actual game process
func (s *SessionService) startGameProcess(ctx context.Context, session *domain.GameSession, game *domain.Game) (domain.ProcessInfo, error) {
	config := game.Config()

	// Create the command
	cmd := exec.CommandContext(ctx, config.Binary.Path, config.Binary.Args...)

	if config.Binary.WorkingDirectory != "" {
		cmd.Dir = config.Binary.WorkingDirectory
	}

	// Set environment variables
	cmd.Env = []string{}
	for key, value := range config.Environment {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return domain.ProcessInfo{}, fmt.Errorf("failed to start process: %w", err)
	}

	return domain.ProcessInfo{
		PID: cmd.Process.Pid,
	}, nil
}

// stopGameProcess stops a game process
func (s *SessionService) stopGameProcess(ctx context.Context, session *domain.GameSession) error {
	// Implementation would send appropriate signals to terminate the process
	// This is a simplified version
	return nil
}

// validateStartSessionRequest validates a start session request
func (s *SessionService) validateStartSessionRequest(req *StartSessionRequest) error {
	if req.UserID <= 0 {
		return fmt.Errorf("user ID is required")
	}
	if req.Username == "" {
		return fmt.Errorf("username is required")
	}
	if req.GameID == "" {
		return fmt.Errorf("game ID is required")
	}
	if req.TerminalWidth <= 0 || req.TerminalHeight <= 0 {
		return fmt.Errorf("valid terminal dimensions are required")
	}
	return nil
}

// generateSessionID generates a unique session ID
func generateSessionID() string {
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}

// generateEventID generates a unique event ID
func generateEventID() string {
	return fmt.Sprintf("event_%d", time.Now().UnixNano())
}
