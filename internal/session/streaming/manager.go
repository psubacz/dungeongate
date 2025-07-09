package streaming

import (
	"context"
	"log/slog"
	"sync"

	"github.com/dungeongate/internal/session/types"
)

// Manager manages streaming in a stateless manner
type Manager struct {
	logger     *slog.Logger
	streams    sync.Map // map[string]*Stream
	spectators sync.Map // map[string]*Spectator
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
	m.logger.Info("Streaming manager starting")
	return nil
}

// Stop stops the streaming manager
func (m *Manager) Stop(ctx context.Context) error {
	m.logger.Info("Streaming manager stopping")

	// Close all streams
	m.streams.Range(func(key, value interface{}) bool {
		stream := value.(*Stream)
		stream.Close()
		return true
	})

	// Close all spectators
	m.spectators.Range(func(key, value interface{}) bool {
		spectator := value.(*Spectator)
		spectator.Close()
		return true
	})

	return nil
}

// CreateStream creates a new streaming session
func (m *Manager) CreateStream(sessionID, userID string) (*Stream, error) {
	streamID := sessionID + "-stream"

	stream := &Stream{
		ID:        streamID,
		SessionID: sessionID,
		UserID:    userID,
		Active:    true,
		logger:    m.logger,
	}

	m.streams.Store(streamID, stream)

	m.logger.Info("Stream created", "stream_id", streamID, "session_id", sessionID, "user_id", userID)

	return stream, nil
}

// GetStream retrieves a stream
func (m *Manager) GetStream(streamID string) (*Stream, bool) {
	if value, exists := m.streams.Load(streamID); exists {
		return value.(*Stream), true
	}
	return nil, false
}

// RemoveStream removes a stream
func (m *Manager) RemoveStream(streamID string) {
	if value, exists := m.streams.LoadAndDelete(streamID); exists {
		stream := value.(*Stream)
		stream.Close()
		m.logger.Info("Stream removed", "stream_id", streamID)
	}
}

// AddSpectator adds a spectator to a stream
func (m *Manager) AddSpectator(streamID, userID string) (*Spectator, error) {
	spectatorID := streamID + "-spectator-" + userID

	spectator := &Spectator{
		ID:        spectatorID,
		StreamID:  streamID,
		UserID:    userID,
		Connected: true,
		logger:    m.logger,
	}

	m.spectators.Store(spectatorID, spectator)

	m.logger.Info("Spectator added", "spectator_id", spectatorID, "stream_id", streamID, "user_id", userID)

	return spectator, nil
}

// RemoveSpectator removes a spectator
func (m *Manager) RemoveSpectator(spectatorID string) {
	if value, exists := m.spectators.LoadAndDelete(spectatorID); exists {
		spectator := value.(*Spectator)
		spectator.Close()
		m.logger.Info("Spectator removed", "spectator_id", spectatorID)
	}
}

// GetStreamSpectators gets all spectators for a stream
func (m *Manager) GetStreamSpectators(streamID string) []*Spectator {
	var spectators []*Spectator

	m.spectators.Range(func(key, value interface{}) bool {
		spectator := value.(*Spectator)
		if spectator.StreamID == streamID {
			spectators = append(spectators, spectator)
		}
		return true
	})

	return spectators
}

// GetStats returns streaming statistics
func (m *Manager) GetStats() *types.StreamingStats {
	stats := &types.StreamingStats{
		ActiveStreams:    0,
		TotalStreams:     0,
		ActiveSpectators: 0,
		TotalSpectators:  0,
		Streams:          make(map[string]*types.StreamInfo),
		Spectators:       make(map[string]*types.SpectatorInfo),
	}

	m.streams.Range(func(key, value interface{}) bool {
		stream := value.(*Stream)
		stats.TotalStreams++
		if stream.Active {
			stats.ActiveStreams++
		}

		stats.Streams[stream.ID] = &types.StreamInfo{
			ID:        stream.ID,
			SessionID: stream.SessionID,
			UserID:    stream.UserID,
			Active:    stream.Active,
		}

		return true
	})

	m.spectators.Range(func(key, value interface{}) bool {
		spectator := value.(*Spectator)
		stats.TotalSpectators++
		if spectator.Connected {
			stats.ActiveSpectators++
		}

		stats.Spectators[spectator.ID] = &types.SpectatorInfo{
			ID:        spectator.ID,
			StreamID:  spectator.StreamID,
			UserID:    spectator.UserID,
			Connected: spectator.Connected,
		}

		return true
	})

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
