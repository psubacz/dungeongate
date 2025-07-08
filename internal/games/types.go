package games

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dungeongate/pkg/ttyrec"
)

// Session represents a game session
type Session struct {
	ID            string                             `json:"id"`
	UserID        int                                `json:"user_id"`
	Username      string                             `json:"username"`
	GameID        string                             `json:"game_id"`
	StartTime     time.Time                          `json:"start_time"`
	EndTime       *time.Time                         `json:"end_time,omitempty"`
	IsActive      bool                               `json:"is_active"`
	TTYRecording  *ttyrec.Session                    `json:"-"`
	TerminalSize  string                             `json:"terminal_size"`
	Encoding      string                             `json:"encoding"`
	LastActivity  time.Time                          `json:"last_activity"`
	StreamEnabled bool                               `json:"stream_enabled"`
	Encrypted     bool                               `json:"encrypted"`
	Spectators    []*Spectator                       `json:"spectators,omitempty"` // For JSON serialization (legacy)
	Registry      *atomic.Pointer[SpectatorRegistry] `json:"-"`                    // Immutable spectator registry
	StreamManager *StreamManager                     `json:"-"`                    // Handles immutable data streaming
	ProcessPID    int                                `json:"process_pid,omitempty"`
	ExitCode      int                                `json:"exit_code,omitempty"`
}

// StreamFrame represents an immutable frame of terminal data
type StreamFrame struct {
	Timestamp time.Time `json:"timestamp"`
	Data      []byte    `json:"data"`     // Immutable copy of terminal data
	FrameID   uint64    `json:"frame_id"` // Sequential frame identifier
}

// NewStreamFrame creates a new immutable stream frame
func NewStreamFrame(data []byte, frameID uint64) *StreamFrame {
	// Create deep copy to ensure immutability
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	return &StreamFrame{
		Timestamp: time.Now(),
		Data:      dataCopy,
		FrameID:   frameID,
	}
}

// SpectatorRegistry represents an immutable list of spectators
type SpectatorRegistry struct {
	Spectators map[string]*Spectator `json:"spectators"` // key: spectator ID
	Version    uint64                `json:"version"`    // Registry version for atomic updates
}

// NewSpectatorRegistry creates a new immutable spectator registry
func NewSpectatorRegistry() *SpectatorRegistry {
	return &SpectatorRegistry{
		Spectators: make(map[string]*Spectator),
		Version:    0,
	}
}

// AddSpectator returns a new registry with the spectator added (immutable)
func (r *SpectatorRegistry) AddSpectator(spectator *Spectator) *SpectatorRegistry {
	newSpectators := make(map[string]*Spectator, len(r.Spectators)+1)

	// Copy existing spectators
	for id, spec := range r.Spectators {
		newSpectators[id] = spec
	}

	// Add new spectator
	spectatorID := fmt.Sprintf("%d_%s", spectator.UserID, spectator.Username)
	newSpectators[spectatorID] = spectator

	return &SpectatorRegistry{
		Spectators: newSpectators,
		Version:    r.Version + 1,
	}
}

// RemoveSpectator returns a new registry with the spectator removed (immutable)
func (r *SpectatorRegistry) RemoveSpectator(userID int, username string) *SpectatorRegistry {
	spectatorID := fmt.Sprintf("%d_%s", userID, username)

	// If spectator doesn't exist, return same registry
	if _, exists := r.Spectators[spectatorID]; !exists {
		return r
	}

	newSpectators := make(map[string]*Spectator, len(r.Spectators)-1)

	// Copy all except the removed spectator
	for id, spec := range r.Spectators {
		if id != spectatorID {
			newSpectators[id] = spec
		}
	}

	return &SpectatorRegistry{
		Spectators: newSpectators,
		Version:    r.Version + 1,
	}
}

// GetSpectators returns a slice of all spectators (safe to read concurrently)
func (r *SpectatorRegistry) GetSpectators() []*Spectator {
	spectators := make([]*Spectator, 0, len(r.Spectators))
	for _, spectator := range r.Spectators {
		spectators = append(spectators, spectator)
	}
	return spectators
}

// Spectator represents a session spectator
type Spectator struct {
	UserID     int                 `json:"user_id"`
	Username   string              `json:"username"`
	JoinTime   time.Time           `json:"join_time"`
	Connection SpectatorConnection `json:"-"`
	BytesSent  int64               `json:"bytes_sent"`
	IsActive   bool                `json:"is_active"`
}

// SpectatorConnection interface for different connection types
type SpectatorConnection interface {
	Write(frame *StreamFrame) error
	Close() error
	GetType() string
	IsConnected() bool
}

// StreamManager handles immutable data streaming to spectators
type StreamManager struct {
	frameID      atomic.Uint64
	frameChannel chan *StreamFrame
	stopChan     chan struct{}
	wg           sync.WaitGroup

	// Circular buffer for recent frames
	recentFrames     []*StreamFrame
	recentFramesLock sync.RWMutex
	bufferSize       int
	bufferIndex      int
}

// NewStreamManager creates a new stream manager for a session
func NewStreamManager() *StreamManager {
	const defaultBufferSize = 20 // Keep last 20 frames
	return &StreamManager{
		frameChannel: make(chan *StreamFrame, 1000), // Buffered channel for frames
		stopChan:     make(chan struct{}),
		bufferSize:   defaultBufferSize,
		recentFrames: make([]*StreamFrame, defaultBufferSize),
		bufferIndex:  0,
	}
}

// Start begins the streaming process
func (sm *StreamManager) Start(registry *atomic.Pointer[SpectatorRegistry]) {
	sm.wg.Add(1)
	go sm.streamLoop(registry)
}

// Stop gracefully stops the streaming process
func (sm *StreamManager) Stop() {
	close(sm.stopChan)
	sm.wg.Wait()
}

// SendFrame sends an immutable frame to all spectators
func (sm *StreamManager) SendFrame(data []byte) {
	if len(data) == 0 {
		return
	}

	frameID := sm.frameID.Add(1)
	frame := NewStreamFrame(data, frameID)

	select {
	case sm.frameChannel <- frame:
		// Frame queued successfully
	default:
		// Channel full, drop frame (prevents blocking)
		// Note: would need to import log package for this
		// log.Printf("Warning: Dropped frame %d due to full buffer", frameID)
	}
}

// GetRecentFrames returns the recent frames from the circular buffer
func (sm *StreamManager) GetRecentFrames() []*StreamFrame {
	sm.recentFramesLock.RLock()
	defer sm.recentFramesLock.RUnlock()

	// Collect non-nil frames in order
	frames := make([]*StreamFrame, 0, sm.bufferSize)

	// Start from the oldest frame position
	startIdx := sm.bufferIndex
	for i := 0; i < sm.bufferSize; i++ {
		idx := (startIdx + i) % sm.bufferSize
		if sm.recentFrames[idx] != nil {
			frames = append(frames, sm.recentFrames[idx])
		}
	}

	return frames
}

// streamLoop processes frames and distributes them to spectators
func (sm *StreamManager) streamLoop(registry *atomic.Pointer[SpectatorRegistry]) {
	defer sm.wg.Done()

	for {
		select {
		case frame := <-sm.frameChannel:
			sm.distributeFrame(frame, registry)
		case <-sm.stopChan:
			return
		}
	}
}

// distributeFrame sends a frame to all active spectators
func (sm *StreamManager) distributeFrame(frame *StreamFrame, registry *atomic.Pointer[SpectatorRegistry]) {
	// Store frame in circular buffer
	sm.recentFramesLock.Lock()
	sm.recentFrames[sm.bufferIndex] = frame
	sm.bufferIndex = (sm.bufferIndex + 1) % sm.bufferSize
	sm.recentFramesLock.Unlock()

	// Load current immutable registry
	currentRegistry := registry.Load()
	if currentRegistry == nil {
		return
	}

	// Get current spectators (safe concurrent read)
	spectators := currentRegistry.GetSpectators()

	// Send frame to each spectator concurrently
	for _, spectator := range spectators {
		if spectator.IsActive && spectator.Connection != nil && spectator.Connection.IsConnected() {
			go func(spec *Spectator, f *StreamFrame) {
				if err := spec.Connection.Write(f); err != nil {
					// Note: would need to import log package for this
					// log.Printf("Failed to send frame %d to spectator %s: %v", f.FrameID, spec.Username, err)
					// TODO: Mark spectator as inactive or remove
				}
			}(spectator, frame)
		}
	}
}

// Note: GameSession struct is defined in games.go

// GameType represents game type
type GameType string

const (
	GameTypeRoguelike GameType = "roguelike"
	GameTypeShell     GameType = "shell"
	GameTypeEditor    GameType = "editor"
	GameTypeOther     GameType = "other"
)

// Extended Game structure with additional fields
type ExtendedGame struct {
	*Game
	Type            GameType          `json:"type"`
	Category        string            `json:"category"`
	Version         string            `json:"version"`
	MinTerminalSize string            `json:"min_terminal_size"`
	MaxTerminalSize string            `json:"max_terminal_size"`
	Tags            []string          `json:"tags"`
	LastPlayed      *time.Time        `json:"last_played,omitempty"`
	PlayCount       int               `json:"play_count"`
	AveragePlayTime time.Duration     `json:"average_play_time"`
	Rating          float32           `json:"rating"`
	Difficulty      int               `json:"difficulty"` // 1-10 scale
	Requirements    map[string]string `json:"requirements"`
}

// SessionStatus represents session status
type SessionStatus string

const (
	SessionStatusStarting SessionStatus = "starting"
	SessionStatusActive   SessionStatus = "active"
	SessionStatusPaused   SessionStatus = "paused"
	SessionStatusEnding   SessionStatus = "ending"
	SessionStatusEnded    SessionStatus = "ended"
)

// GameStatistics represents game statistics
type GameStatistics struct {
	TotalSessions      int           `json:"total_sessions"`
	ActiveSessions     int           `json:"active_sessions"`
	TotalPlayTime      time.Duration `json:"total_play_time"`
	AverageSessionTime time.Duration `json:"average_session_time"`
	UniqueUsers        int           `json:"unique_users"`
	PopularityRank     int           `json:"popularity_rank"`
	Rating             float32       `json:"rating"`
	CompletionRate     float32       `json:"completion_rate"`
	AverageScore       float32       `json:"average_score"`
	HighScore          int           `json:"high_score"`
	HighScoreHolder    string        `json:"high_score_holder"`
}

// Service interfaces

// Event system for game notifications

// Note: GameEvent struct is defined in games.go (though with different structure)

// GameEventType constants
type GameEventType string

const (
	GameEventTypeSessionStart   GameEventType = "game.session.start"
	GameEventTypeSessionEnd     GameEventType = "game.session.end"
	GameEventTypeSessionPause   GameEventType = "game.session.pause"
	GameEventTypeSessionResume  GameEventType = "game.session.resume"
	GameEventTypeGameSave       GameEventType = "game.save"
	GameEventTypeGameLoad       GameEventType = "game.load"
	GameEventTypeSpectatorJoin  GameEventType = "game.spectator.join"
	GameEventTypeSpectatorLeave GameEventType = "game.spectator.leave"
)

// GameEventBus interface for game event handling
type GameEventBus interface {
	PublishGameEvent(event *GameEvent) error
	SubscribeToGameEvents(gameID string, handler func(*GameEvent)) error
	UnsubscribeFromGameEvents(gameID string, handler func(*GameEvent)) error
}
