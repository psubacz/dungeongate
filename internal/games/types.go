package games

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dungeongate/pkg/ttyrec"
)

// Note: Game struct is defined in games.go

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

// GameServiceClient interface for game service
type GameServiceClient interface {
	ListGames(ctx context.Context) ([]*Game, error)
	GetGame(ctx context.Context, gameID string) (*Game, error)
	StartGame(ctx context.Context, req *StartGameRequest) (*GameSession, error)
	StopGame(ctx context.Context, sessionID string) error
	GetGameStatus(ctx context.Context, sessionID string) (*GameSession, error)
	UpdateGameConfig(ctx context.Context, gameID string, config *Game) error
}

// Note: StartGameRequest struct is defined in games.go

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