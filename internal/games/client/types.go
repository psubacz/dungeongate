package client

import (
	"time"
)

// StartGameGRPCRequest represents a gRPC request to start a game
type StartGameGRPCRequest struct {
	UserID          string            `json:"user_id"`
	Username        string            `json:"username"`
	GameID          string            `json:"game_id"`
	SessionID       string            `json:"session_id"`
	Environment     map[string]string `json:"environment"`
	EnableRecording bool              `json:"enable_recording"`
}

// StartGameRequest represents a game start request
type StartGameRequest struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	GameID   string `json:"game_id"`
}

// StartGameResponse represents the response from starting a game
type StartGameResponse struct {
	SessionID   string `json:"session_id"`
	ContainerID string `json:"container_id"`
	PodName     string `json:"pod_name"`
	Success     bool   `json:"success"`
	Error       string `json:"error,omitempty"`
}

// StopGameRequest represents a request to stop a game
type StopGameRequest struct {
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
	Force     bool   `json:"force"`
	Reason    string `json:"reason"`
}

// StopGameResponse represents the response from stopping a game
type StopGameResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// GameSessionInfo represents information about a game session
type GameSessionInfo struct {
	SessionID     string            `json:"session_id"`
	UserID        string            `json:"user_id"`
	Username      string            `json:"username"`
	GameID        string            `json:"game_id"`
	Status        string            `json:"status"`
	StartTime     time.Time         `json:"start_time"`
	LastActivity  time.Time         `json:"last_activity"`
	ContainerID   string            `json:"container_id"`
	PodName       string            `json:"pod_name"`
	RecordingPath string            `json:"recording_path"`
	Spectators    []string          `json:"spectators"`
	Metadata      map[string]string `json:"metadata"`
}
