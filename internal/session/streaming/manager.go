package streaming

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/dungeongate/internal/session/client"
	gamev2 "github.com/dungeongate/pkg/api/games/v2"
)

// Manager manages streaming in a stateless manner
// All stream state is managed by Game Service
type Manager struct {
	logger     *slog.Logger
	gameClient *client.GameClient
	// NOTE: No local state storage - truly stateless
	// Stream and spectator state managed by Game Service
}

// Stream represents a streaming session
type Stream struct {
	ID        string
	SessionID string
	UserID    string
	Active    bool
	logger    *slog.Logger
}

// Spectator represents a spectator connection
type Spectator struct {
	ID        string
	StreamID  string
	UserID    string
	Connected bool
	logger    *slog.Logger
}

// NewManager creates a new streaming manager
func NewManager(logger *slog.Logger, gameClient *client.GameClient) *Manager {
	return &Manager{
		logger:     logger,
		gameClient: gameClient,
	}
}

// Start starts the streaming manager
func (m *Manager) Start(ctx context.Context) error {
	if m.logger != nil {
		m.logger.Info("Streaming manager starting")
	}
	return nil
}

// Stop stops the streaming manager
func (m *Manager) Stop(ctx context.Context) error {
	if m.logger != nil {
		m.logger.Info("Streaming manager stopping")
	}

	// In stateless mode, no local streams/spectators to close
	// Stream cleanup handled by Game Service
	if m.logger != nil {
		m.logger.Info("Streaming manager stopped (stateless mode)")
	}

	return nil
}

// CreateStream delegates stream creation to Game Service
// Returns stream info but doesn't store state locally
func (m *Manager) CreateStream(ctx context.Context, sessionID, userID string) (*Stream, error) {
	streamID := sessionID + "-stream"

	// Get session info to verify it exists and has streaming enabled
	sessionInfo, err := m.gameClient.GetGameSession(ctx, sessionID)
	if err != nil {
		m.logger.Error("Failed to get session for stream creation",
			"session_id", sessionID,
			"error", err)
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Check if session is active
	if sessionInfo.State != "active" {
		return nil, fmt.Errorf("session %s is not active (state: %s)", sessionID, sessionInfo.State)
	}

	m.logger.Info("Stream creation for active session",
		"stream_id", streamID,
		"session_id", sessionID,
		"user_id", userID,
		"session_state", sessionInfo.State)

	// Return stream info without storing locally
	stream := &Stream{
		ID:        streamID,
		SessionID: sessionID,
		UserID:    userID,
		Active:    true,
		logger:    m.logger,
	}

	return stream, nil
}

// GetStream should query Game Service for stream state
func (m *Manager) GetStream(ctx context.Context, streamID string) (*Stream, bool) {
	// Extract session ID from stream ID (format: sessionID-stream)
	sessionID := streamID
	if len(streamID) > 7 && streamID[len(streamID)-7:] == "-stream" {
		sessionID = streamID[:len(streamID)-7]
	}

	// Query Game Service for session state
	sessionInfo, err := m.gameClient.GetGameSession(ctx, sessionID)
	if err != nil {
		m.logger.Debug("Failed to get session for stream query",
			"stream_id", streamID,
			"session_id", sessionID,
			"error", err)
		return nil, false
	}

	// Check if session is active
	if sessionInfo.State != "active" {
		m.logger.Debug("Session is not active",
			"stream_id", streamID,
			"session_id", sessionID,
			"state", sessionInfo.State)
		return nil, false
	}

	stream := &Stream{
		ID:        streamID,
		SessionID: sessionID,
		UserID:    sessionInfo.UserID,
		Active:    true,
		logger:    m.logger,
	}

	return stream, true
}

// RemoveStream delegates to Game Service
func (m *Manager) RemoveStream(streamID string) {
	// TODO: Add gRPC call to Game Service to remove stream
	m.logger.Info("Stream removal delegated to Game Service", "stream_id", streamID)
}

// AddSpectator delegates to Game Service
func (m *Manager) AddSpectator(ctx context.Context, streamID, userID string, username string) (*Spectator, error) {
	spectatorID := streamID + "-spectator-" + userID

	// Extract session ID from stream ID
	sessionID := streamID
	if len(streamID) > 7 && streamID[len(streamID)-7:] == "-stream" {
		sessionID = streamID[:len(streamID)-7]
	}

	// Parse user ID to int32
	userIDInt, err := parseInt32(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Add spectator via Game Service
	err = m.gameClient.AddSpectator(ctx, sessionID, userIDInt, username)
	if err != nil {
		m.logger.Error("Failed to add spectator via Game Service",
			"spectator_id", spectatorID,
			"stream_id", streamID,
			"session_id", sessionID,
			"user_id", userID,
			"error", err)
		return nil, fmt.Errorf("failed to add spectator: %w", err)
	}

	m.logger.Info("Spectator addition successful",
		"spectator_id", spectatorID,
		"stream_id", streamID,
		"session_id", sessionID,
		"user_id", userID,
		"username", username)

	// Return spectator info without storing locally
	spectator := &Spectator{
		ID:        spectatorID,
		StreamID:  streamID,
		UserID:    userID,
		Connected: true,
		logger:    m.logger,
	}

	return spectator, nil
}

// RemoveSpectator delegates to Game Service
func (m *Manager) RemoveSpectator(ctx context.Context, spectatorID string) error {
	// Extract session ID and user ID from spectator ID
	// Format: streamID-spectator-userID
	parts := parseSpectatorID(spectatorID)
	if len(parts) < 3 {
		return fmt.Errorf("invalid spectator ID format: %s", spectatorID)
	}

	streamID := parts[0]
	userID := parts[2]

	// Extract session ID from stream ID
	sessionID := streamID
	if len(streamID) > 7 && streamID[len(streamID)-7:] == "-stream" {
		sessionID = streamID[:len(streamID)-7]
	}

	// Parse user ID to int32
	userIDInt, err := parseInt32(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID in spectator ID: %w", err)
	}

	// Remove spectator via Game Service
	err = m.gameClient.RemoveSpectator(ctx, sessionID, userIDInt)
	if err != nil {
		m.logger.Error("Failed to remove spectator via Game Service",
			"spectator_id", spectatorID,
			"session_id", sessionID,
			"user_id", userID,
			"error", err)
		return fmt.Errorf("failed to remove spectator: %w", err)
	}

	m.logger.Info("Spectator removal successful",
		"spectator_id", spectatorID,
		"session_id", sessionID,
		"user_id", userID)

	return nil
}

// GetStreamSpectators should query Game Service
func (m *Manager) GetStreamSpectators(ctx context.Context, streamID string) []*Spectator {
	// Extract session ID from stream ID
	sessionID := streamID
	if len(streamID) > 7 && streamID[len(streamID)-7:] == "-stream" {
		sessionID = streamID[:len(streamID)-7]
	}

	// Get session with spectators from Game Service
	session, err := m.gameClient.GetGameSessionWithSpectators(ctx, sessionID)
	if err != nil {
		m.logger.Debug("Failed to get session spectators",
			"stream_id", streamID,
			"session_id", sessionID,
			"error", err)
		return []*Spectator{}
	}

	// Convert proto spectators to local spectator objects
	spectators := make([]*Spectator, len(session.Spectators))
	for i, protoSpec := range session.Spectators {
		spectatorID := fmt.Sprintf("%s-spectator-%d", streamID, protoSpec.UserId)
		spectators[i] = &Spectator{
			ID:        spectatorID,
			StreamID:  streamID,
			UserID:    fmt.Sprintf("%d", protoSpec.UserId),
			Connected: protoSpec.IsActive,
			logger:    m.logger,
		}
	}

	return spectators
}

// StreamingStats represents basic streaming statistics for stateless mode
type StreamingStats struct {
	ActiveStreams    int `json:"active_streams"`
	TotalStreams     int `json:"total_streams"`
	ActiveSpectators int `json:"active_spectators"`
	TotalSpectators  int `json:"total_spectators"`
}

// GetStats should query Game Service for streaming statistics
func (m *Manager) GetStats(ctx context.Context) *StreamingStats {
	// Get all active sessions from Game Service
	sessions, err := m.gameClient.GetActiveGameSessions(ctx)
	if err != nil {
		m.logger.Debug("Failed to get streaming stats from Game Service", "error", err)
		return &StreamingStats{
			ActiveStreams:    0,
			TotalStreams:     0,
			ActiveSpectators: 0,
			TotalSpectators:  0,
		}
	}

	// Calculate statistics
	activeStreams := 0
	activeSpectators := 0

	for _, session := range sessions {
		if session.Status == gamev2.SessionStatus_SESSION_STATUS_ACTIVE {
			activeStreams++
			activeSpectators += len(session.Spectators)
		}
	}

	stats := &StreamingStats{
		ActiveStreams:    activeStreams,
		TotalStreams:     activeStreams, // For stateless mode, these are the same
		ActiveSpectators: activeSpectators,
		TotalSpectators:  activeSpectators, // For stateless mode, these are the same
	}

	m.logger.Debug("Streaming stats calculated",
		"active_streams", stats.ActiveStreams,
		"active_spectators", stats.ActiveSpectators)

	return stats
}

// Close closes the stream
func (s *Stream) Close() error {
	s.Active = false
	s.logger.Debug("Stream closed", "stream_id", s.ID)
	return nil
}

// Close closes the spectator connection
func (s *Spectator) Close() error {
	s.Connected = false
	s.logger.Debug("Spectator disconnected", "spectator_id", s.ID)
	return nil
}

// Helper functions

// parseInt32 parses a string to int32
func parseInt32(s string) (int32, error) {
	i, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(i), nil
}

// parseSpectatorID parses a spectator ID into its components
// Format: streamID-spectator-userID
func parseSpectatorID(spectatorID string) []string {
	// Simple split by "-spectator-"
	parts := []string{}
	spectatorIndex := -1

	// Find the last occurrence of "-spectator-"
	for i := len(spectatorID) - 11; i >= 0; i-- {
		if spectatorID[i:i+11] == "-spectator-" {
			spectatorIndex = i
			break
		}
	}

	if spectatorIndex == -1 {
		return parts
	}

	// Extract parts
	streamID := spectatorID[:spectatorIndex]
	userID := spectatorID[spectatorIndex+11:]

	parts = append(parts, streamID)
	parts = append(parts, "spectator")
	parts = append(parts, userID)

	return parts
}
