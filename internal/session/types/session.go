package types

import (
	"time"
)

// SessionState represents the state of a session
type SessionState int

const (
	SessionStateCreated SessionState = iota
	SessionStateStarting
	SessionStateActive
	SessionStatePaused
	SessionStateEnding
	SessionStateEnded
)

// SessionInfo represents session information from Game Service
type SessionInfo struct {
	ID           string
	UserID       string
	GameID       string
	State        SessionState
	CreatedAt    time.Time
	LastActivity time.Time
	Metadata     map[string]interface{}
}

// PTYInfo represents PTY information
type PTYInfo struct {
	SessionID string
	Cols      int
	Rows      int
	Term      string
}

// Winsize represents terminal window size
type Winsize struct {
	Row uint16
	Col uint16
	X   uint16
	Y   uint16
}

// TerminalStats represents terminal statistics
type TerminalStats struct {
	ActiveSessions int
	TotalSessions  int
	Sessions       map[string]*TerminalSessionInfo
}

// TerminalSessionInfo represents terminal session information
type TerminalSessionInfo struct {
	ID      string
	Command []string
	Rows    int
	Cols    int
	Active  bool
}

// StreamingStats represents streaming statistics
type StreamingStats struct {
	ActiveStreams    int
	TotalStreams     int
	ActiveSpectators int
	TotalSpectators  int
	Streams          map[string]*StreamInfo
	Spectators       map[string]*SpectatorInfo
}

// StreamInfo represents stream information
type StreamInfo struct {
	ID        string
	SessionID string
	UserID    string
	Active    bool
}

// SpectatorInfo represents spectator information
type SpectatorInfo struct {
	ID        string
	StreamID  string
	UserID    string
	Connected bool
}
