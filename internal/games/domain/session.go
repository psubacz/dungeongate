package domain

import (
	"fmt"
	"time"
)

// GameSession aggregate root represents an active game session
type GameSession struct {
	// Identity
	id     SessionID
	userID UserID
	gameID GameID

	// Configuration
	username   string
	gameConfig GameConfig

	// Process information
	processInfo ProcessInfo

	// Session state
	status       SessionStatus
	startTime    time.Time
	endTime      *time.Time
	lastActivity time.Time

	// Terminal configuration
	terminalSize TerminalSize
	encoding     string

	// Features
	recording  *RecordingInfo
	streaming  *StreamingInfo
	spectators []SpectatorInfo

	// Audit
	createdAt time.Time
	updatedAt time.Time
}

// SessionID represents a unique session identifier
type SessionID struct {
	value string
}

// NewSessionID creates a new session ID
func NewSessionID(value string) SessionID {
	return SessionID{value: value}
}

// String returns the string representation of the session ID
func (id SessionID) String() string {
	return id.value
}

// UserID represents a user identifier
type UserID struct {
	value int
}

// NewUserID creates a new user ID
func NewUserID(value int) UserID {
	return UserID{value: value}
}

// Int returns the integer representation of the user ID
func (id UserID) Int() int {
	return id.value
}

// ProcessInfo contains information about the game process
type ProcessInfo struct {
	PID         int
	ContainerID string
	PodName     string
	ExitCode    *int
	Signal      *string
}

// SessionStatus represents the current status of a session
type SessionStatus string

const (
	SessionStatusStarting SessionStatus = "starting"
	SessionStatusActive   SessionStatus = "active"
	SessionStatusPaused   SessionStatus = "paused"
	SessionStatusEnding   SessionStatus = "ending"
	SessionStatusEnded    SessionStatus = "ended"
	SessionStatusFailed   SessionStatus = "failed"
)

// TerminalSize represents terminal dimensions
type TerminalSize struct {
	Width  int
	Height int
}

// String returns the terminal size as a string
func (ts TerminalSize) String() string {
	return fmt.Sprintf("%dx%d", ts.Width, ts.Height)
}

// RecordingInfo contains session recording information
type RecordingInfo struct {
	Enabled    bool
	FilePath   string
	Format     string
	StartTime  time.Time
	FileSize   int64
	Compressed bool
}

// StreamingInfo contains session streaming information
type StreamingInfo struct {
	Enabled       bool
	Protocol      string
	Encrypted     bool
	FrameCount    uint64
	BytesStreamed int64
}

// SpectatorInfo contains information about a spectator
type SpectatorInfo struct {
	UserID    UserID
	Username  string
	JoinTime  time.Time
	BytesSent int64
	IsActive  bool
}

// NewGameSession creates a new game session
func NewGameSession(
	id SessionID,
	userID UserID,
	username string,
	gameID GameID,
	gameConfig GameConfig,
	terminalSize TerminalSize,
) *GameSession {
	now := time.Now()
	return &GameSession{
		id:           id,
		userID:       userID,
		username:     username,
		gameID:       gameID,
		gameConfig:   gameConfig,
		status:       SessionStatusStarting,
		startTime:    now,
		lastActivity: now,
		terminalSize: terminalSize,
		encoding:     "utf-8",
		spectators:   make([]SpectatorInfo, 0),
		createdAt:    now,
		updatedAt:    now,
	}
}

// ID returns the session's ID
func (s *GameSession) ID() SessionID {
	return s.id
}

// UserID returns the session's user ID
func (s *GameSession) UserID() UserID {
	return s.userID
}

// Username returns the session's username
func (s *GameSession) Username() string {
	return s.username
}

// GameID returns the session's game ID
func (s *GameSession) GameID() GameID {
	return s.gameID
}

// Status returns the session's current status
func (s *GameSession) Status() SessionStatus {
	return s.status
}

// StartTime returns when the session started
func (s *GameSession) StartTime() time.Time {
	return s.startTime
}

// EndTime returns when the session ended (if it has ended)
func (s *GameSession) EndTime() *time.Time {
	return s.endTime
}

// Duration returns the session duration
func (s *GameSession) Duration() time.Duration {
	if s.endTime != nil {
		return s.endTime.Sub(s.startTime)
	}
	return time.Since(s.startTime)
}

// IsActive returns true if the session is active
func (s *GameSession) IsActive() bool {
	return s.status == SessionStatusActive
}

// CanSpectate returns true if the session allows spectators
func (s *GameSession) CanSpectate() bool {
	return s.status == SessionStatusActive && s.streaming != nil && s.streaming.Enabled
}

// Start activates the session
func (s *GameSession) Start(processInfo ProcessInfo) {
	s.status = SessionStatusActive
	s.processInfo = processInfo
	s.lastActivity = time.Now()
	s.updatedAt = time.Now()
}

// Pause pauses the session
func (s *GameSession) Pause() error {
	if s.status != SessionStatusActive {
		return fmt.Errorf("cannot pause session in status: %s", s.status)
	}
	s.status = SessionStatusPaused
	s.updatedAt = time.Now()
	return nil
}

// Resume resumes the session
func (s *GameSession) Resume() error {
	if s.status != SessionStatusPaused {
		return fmt.Errorf("cannot resume session in status: %s", s.status)
	}
	s.status = SessionStatusActive
	s.lastActivity = time.Now()
	s.updatedAt = time.Now()
	return nil
}

// End ends the session
func (s *GameSession) End(exitCode *int, signal *string) {
	s.status = SessionStatusEnded
	now := time.Now()
	s.endTime = &now
	if exitCode != nil {
		s.processInfo.ExitCode = exitCode
	}
	if signal != nil {
		s.processInfo.Signal = signal
	}
	s.updatedAt = now
}

// Fail marks the session as failed
func (s *GameSession) Fail(reason string) {
	s.status = SessionStatusFailed
	now := time.Now()
	s.endTime = &now
	s.updatedAt = now
}

// UpdateActivity updates the last activity time
func (s *GameSession) UpdateActivity() {
	s.lastActivity = time.Now()
	s.updatedAt = time.Now()
}

// EnableRecording enables session recording
func (s *GameSession) EnableRecording(filePath, format string) {
	s.recording = &RecordingInfo{
		Enabled:   true,
		FilePath:  filePath,
		Format:    format,
		StartTime: time.Now(),
	}
	s.updatedAt = time.Now()
}

// EnableStreaming enables session streaming
func (s *GameSession) EnableStreaming(protocol string, encrypted bool) {
	s.streaming = &StreamingInfo{
		Enabled:   true,
		Protocol:  protocol,
		Encrypted: encrypted,
	}
	s.updatedAt = time.Now()
}

// AddSpectator adds a spectator to the session
func (s *GameSession) AddSpectator(userID UserID, username string) error {
	if !s.CanSpectate() {
		return fmt.Errorf("session does not allow spectators")
	}

	// Check if spectator already exists
	for _, spectator := range s.spectators {
		if spectator.UserID == userID {
			return fmt.Errorf("user is already spectating this session")
		}
	}

	spectator := SpectatorInfo{
		UserID:   userID,
		Username: username,
		JoinTime: time.Now(),
		IsActive: true,
	}
	s.spectators = append(s.spectators, spectator)
	s.updatedAt = time.Now()
	return nil
}

// RemoveSpectator removes a spectator from the session
func (s *GameSession) RemoveSpectator(userID UserID) {
	for i, spectator := range s.spectators {
		if spectator.UserID == userID {
			s.spectators = append(s.spectators[:i], s.spectators[i+1:]...)
			break
		}
	}
	s.updatedAt = time.Now()
}

// Spectators returns the list of spectators
func (s *GameSession) Spectators() []SpectatorInfo {
	return s.spectators
}

// SpectatorCount returns the number of active spectators
func (s *GameSession) SpectatorCount() int {
	count := 0
	for _, spectator := range s.spectators {
		if spectator.IsActive {
			count++
		}
	}
	return count
}

// TerminalSize returns the terminal size
func (s *GameSession) TerminalSize() TerminalSize {
	return s.terminalSize
}

// UpdateTerminalSize updates the terminal size
func (s *GameSession) UpdateTerminalSize(size TerminalSize) {
	s.terminalSize = size
	s.updatedAt = time.Now()
}

// ProcessInfo returns the process information
func (s *GameSession) ProcessInfo() ProcessInfo {
	return s.processInfo
}

// RecordingInfo returns the recording information
func (s *GameSession) RecordingInfo() *RecordingInfo {
	return s.recording
}

// StreamingInfo returns the streaming information
func (s *GameSession) StreamingInfo() *StreamingInfo {
	return s.streaming
}

// CreatedAt returns when the session was created
func (s *GameSession) CreatedAt() time.Time {
	return s.createdAt
}

// UpdatedAt returns when the session was last updated
func (s *GameSession) UpdatedAt() time.Time {
	return s.updatedAt
}

// LastActivity returns when the session was last active
func (s *GameSession) LastActivity() time.Time {
	return s.lastActivity
}

// Encoding returns the session's encoding
func (s *GameSession) Encoding() string {
	return s.encoding
}
