package streaming

import (
	"context"
	"log/slog"

	"github.com/dungeongate/internal/session/types"
)

// Manager manages streaming in a stateless manner
// All stream state is managed by Game Service
type Manager struct {
	logger *slog.Logger
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
func NewManager(logger *slog.Logger) *Manager {
	return &Manager{
		logger: logger,
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
func (m *Manager) CreateStream(sessionID, userID string) (*Stream, error) {
	streamID := sessionID + "-stream"

	// Stream creation delegated to Game Service
	// TODO: Add gRPC call to Game Service to create stream
	m.logger.Info("Stream creation delegated to Game Service",
		"stream_id", streamID,
		"session_id", sessionID,
		"user_id", userID)

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
func (m *Manager) GetStream(streamID string) (*Stream, bool) {
	// TODO: Query Game Service for stream state
	m.logger.Debug("Stream state query should use Game Service", "stream_id", streamID)
	return nil, false
}

// RemoveStream delegates to Game Service
func (m *Manager) RemoveStream(streamID string) {
	// TODO: Add gRPC call to Game Service to remove stream
	m.logger.Info("Stream removal delegated to Game Service", "stream_id", streamID)
}

// AddSpectator delegates to Game Service
func (m *Manager) AddSpectator(streamID, userID string) (*Spectator, error) {
	spectatorID := streamID + "-spectator-" + userID

	// TODO: Add gRPC call to Game Service to add spectator
	m.logger.Info("Spectator addition delegated to Game Service",
		"spectator_id", spectatorID,
		"stream_id", streamID,
		"user_id", userID)

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
func (m *Manager) RemoveSpectator(spectatorID string) {
	// TODO: Add gRPC call to Game Service to remove spectator
	m.logger.Info("Spectator removal delegated to Game Service", "spectator_id", spectatorID)
}

// GetStreamSpectators should query Game Service
func (m *Manager) GetStreamSpectators(streamID string) []*Spectator {
	// TODO: Query Game Service for spectator list
	m.logger.Debug("Spectator list query should use Game Service", "stream_id", streamID)
	return []*Spectator{}
}

// GetStats should query Game Service for streaming statistics
func (m *Manager) GetStats() *types.StreamingStats {
	// TODO: Query Game Service for streaming stats
	m.logger.Debug("Streaming stats query should use Game Service")

	// Return empty stats - real stats should come from Game Service
	stats := &types.StreamingStats{
		ActiveStreams:    0,
		TotalStreams:     0,
		ActiveSpectators: 0,
		TotalSpectators:  0,
		Streams:          make(map[string]*types.StreamInfo),
		Spectators:       make(map[string]*types.SpectatorInfo),
	}

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
