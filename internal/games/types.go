package games

import (
	"fmt"
	"strings"
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

	// Full screen state buffer sized to terminal dimensions
	screenBuffer     [][]byte // [row][col] - 2D array representing the screen
	screenBufferLock sync.RWMutex
	terminalRows     int
	terminalCols     int

	// Internal spectator registry
	registry *atomic.Pointer[SpectatorRegistry]
}

// NewStreamManager creates a new stream manager for a session
func NewStreamManager() *StreamManager {
	const defaultBufferSize = 100 // Keep last 100 frames to provide better context for new spectators
	registry := &atomic.Pointer[SpectatorRegistry]{}
	registry.Store(NewSpectatorRegistry())

	return &StreamManager{
		frameChannel: make(chan *StreamFrame, 1000), // Buffered channel for frames
		stopChan:     make(chan struct{}),
		bufferSize:   defaultBufferSize,
		recentFrames: make([]*StreamFrame, defaultBufferSize),
		bufferIndex:  0,
		terminalRows: 24, // Default terminal size
		terminalCols: 80,
		screenBuffer: make([][]byte, 24), // Initialize with default size
		registry:     registry,
	}
}

// NewStreamManagerWithSize creates a new stream manager with specific terminal dimensions
func NewStreamManagerWithSize(rows, cols int) *StreamManager {
	const defaultBufferSize = 100 // Keep last 100 frames to provide better context for new spectators

	// Initialize screen buffer with terminal dimensions
	screenBuffer := make([][]byte, rows)
	for i := range screenBuffer {
		screenBuffer[i] = make([]byte, cols)
		// Initialize with spaces
		for j := range screenBuffer[i] {
			screenBuffer[i][j] = ' '
		}
	}

	registry := &atomic.Pointer[SpectatorRegistry]{}
	registry.Store(NewSpectatorRegistry())

	return &StreamManager{
		frameChannel: make(chan *StreamFrame, 1000), // Buffered channel for frames
		stopChan:     make(chan struct{}),
		bufferSize:   defaultBufferSize,
		recentFrames: make([]*StreamFrame, defaultBufferSize),
		bufferIndex:  0,
		terminalRows: rows,
		terminalCols: cols,
		screenBuffer: screenBuffer,
		registry:     registry,
	}
}

// Start begins the streaming process
func (sm *StreamManager) Start() {
	sm.wg.Add(1)
	go sm.streamLoop(sm.registry)
}

// Stop gracefully stops the streaming process
func (sm *StreamManager) Stop() {
	close(sm.stopChan)
	sm.wg.Wait()
}

// AddSpectator adds a spectator to the stream
func (sm *StreamManager) AddSpectator(spectator *Spectator) {
	currentRegistry := sm.registry.Load()
	newRegistry := currentRegistry.AddSpectator(spectator)
	sm.registry.Store(newRegistry)
}

// RemoveSpectator removes a spectator from the stream
func (sm *StreamManager) RemoveSpectator(userID int, username string) {
	currentRegistry := sm.registry.Load()
	newRegistry := currentRegistry.RemoveSpectator(userID, username)
	sm.registry.Store(newRegistry)
}

// SendFrame sends an immutable frame to all spectators
func (sm *StreamManager) SendFrame(data []byte) {
	if len(data) == 0 {
		return
	}

	// Update the screen buffer with the new data
	sm.UpdateScreenBuffer(data)

	frameID := sm.frameID.Add(1)
	frame := NewStreamFrame(data, frameID)

	// Store frame in circular buffer immediately (for spectators joining later)
	sm.storeFrameInBuffer(frame)

	// Also queue for active spectators (non-blocking)
	select {
	case sm.frameChannel <- frame:
		// Frame queued successfully for active spectators
	default:
		// Channel full, but frame is still stored in buffer for new spectators
		// This prevents blocking the player's output
	}
}

// storeFrameInBuffer stores a frame directly in the circular buffer
func (sm *StreamManager) storeFrameInBuffer(frame *StreamFrame) {
	sm.recentFramesLock.Lock()
	defer sm.recentFramesLock.Unlock()

	sm.recentFrames[sm.bufferIndex] = frame
	sm.bufferIndex = (sm.bufferIndex + 1) % sm.bufferSize
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

// ClearRecentFrames clears the recent frames buffer
func (sm *StreamManager) ClearRecentFrames() {
	sm.recentFramesLock.Lock()
	defer sm.recentFramesLock.Unlock()

	// Clear all frames
	for i := range sm.recentFrames {
		sm.recentFrames[i] = nil
	}
	sm.bufferIndex = 0
}

// GetFullScreen returns the complete screen state as a single byte slice
func (sm *StreamManager) GetFullScreen() []byte {
	sm.screenBufferLock.RLock()
	defer sm.screenBufferLock.RUnlock()

	// For now, return empty - we'll rely on the redraw command to get current state
	// This avoids the complexity of terminal emulation while still providing the interface
	return []byte{}
}

// UpdateScreenBuffer processes terminal escape sequences and updates the screen buffer
func (sm *StreamManager) UpdateScreenBuffer(data []byte) {
	sm.screenBufferLock.Lock()
	defer sm.screenBufferLock.Unlock()

	// For spectating, we want to preserve the current screen state
	// Only clear the buffer when we see explicit clear screen commands
	// This helps maintain visual continuity for spectators

	dataStr := string(data)

	// Check for explicit clear screen escape sequences
	if strings.Contains(dataStr, "\x1b[2J") {
		// Clear screen command - reset our buffer
		for row := 0; row < sm.terminalRows; row++ {
			for col := 0; col < sm.terminalCols; col++ {
				sm.screenBuffer[row][col] = ' '
			}
		}
		return
	}

	// Don't clear buffer on cursor home alone - this happens frequently in games
	// like NetHack for normal screen updates without full clears
	// Only clear when we see an explicit clear screen sequence

	// For now, don't try to parse complex escape sequences for position updates
	// Just let the raw data through - the terminal emulator on the spectator side
	// will handle the rendering. We preserve the recent frames which contain
	// the actual screen state that spectators need.
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
	// Frame is already stored in buffer by SendFrame method, so no need to store again

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
