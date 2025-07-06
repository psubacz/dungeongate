package session

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dungeongate/pkg/config"
	"github.com/dungeongate/pkg/ttyrec"
)

// Core data structures

// User represents a user in the system
type User struct {
	ID              int       `json:"id"`
	Username        string    `json:"username"`
	Email           string    `json:"email,omitempty"`
	IsAuthenticated bool      `json:"is_authenticated"`
	IsActive        bool      `json:"is_active"`
	IsAdmin         bool      `json:"is_admin"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	LastLogin       time.Time `json:"last_login"`
}

// Game represents a game configuration
type Game struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	ShortName   string            `json:"short_name"`
	Description string            `json:"description"`
	Enabled     bool              `json:"enabled"`
	Binary      string            `json:"binary"`
	Args        []string          `json:"args"`
	WorkingDir  string            `json:"working_dir"`
	Environment map[string]string `json:"environment"`
	MaxPlayers  int               `json:"max_players"`
	Spectatable bool              `json:"spectatable"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

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
		log.Printf("Warning: Dropped frame %d due to full buffer", frameID)
	}
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
					log.Printf("Failed to send frame %d to spectator %s: %v", f.FrameID, spec.Username, err)
					// TODO: Mark spectator as inactive or remove
				}
			}(spectator, frame)
		}
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

// SSHSpectatorConnection represents an SSH-based spectator connection
type SSHSpectatorConnection struct {
	SessionCtx *SSHSessionContext
	connected  bool
	mutex      sync.RWMutex
}

func NewSSHSpectatorConnection(sessionCtx *SSHSessionContext) *SSHSpectatorConnection {
	return &SSHSpectatorConnection{
		SessionCtx: sessionCtx,
		connected:  true,
	}
}

func (c *SSHSpectatorConnection) Write(frame *StreamFrame) error {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if !c.connected || c.SessionCtx == nil || c.SessionCtx.Channel == nil {
		return fmt.Errorf("SSH connection not available")
	}

	// Write immutable frame data directly
	n, err := c.SessionCtx.Channel.Write(frame.Data)
	if err == nil && n > 0 {
		// Debug: log first few bytes to see what's being sent
		preview := frame.Data
		if len(preview) > 50 {
			preview = preview[:50]
		}
		log.Printf("Spectator %s: wrote %d bytes (preview: %q...)", c.SessionCtx.Username, n, preview)
	}
	return err
}

func (c *SSHSpectatorConnection) Close() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.connected = false
	if c.SessionCtx != nil && c.SessionCtx.Channel != nil {
		return c.SessionCtx.Channel.Close()
	}
	return nil
}

func (c *SSHSpectatorConnection) GetType() string {
	return "ssh"
}

func (c *SSHSpectatorConnection) IsConnected() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.connected
}

// WebSocketSpectatorConnection represents a WebSocket-based spectator connection (stubbed)
type WebSocketSpectatorConnection struct {
	ConnID    string
	connected bool
	mutex     sync.RWMutex
}

func NewWebSocketSpectatorConnection(connID string) *WebSocketSpectatorConnection {
	return &WebSocketSpectatorConnection{
		ConnID:    connID,
		connected: true,
	}
}

func (c *WebSocketSpectatorConnection) Write(frame *StreamFrame) error {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if !c.connected {
		return fmt.Errorf("WebSocket connection closed")
	}

	// TODO: Implement WebSocket frame writing when ready
	// Convert frame to JSON and send via WebSocket
	log.Printf("WebSocket spectator %s would receive frame %d with %d bytes at %v",
		c.ConnID, frame.FrameID, len(frame.Data), frame.Timestamp)
	return nil
}

func (c *WebSocketSpectatorConnection) Close() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.connected = false
	// TODO: Implement WebSocket close when ready
	log.Printf("WebSocket spectator %s connection closed", c.ConnID)
	return nil
}

func (c *WebSocketSpectatorConnection) GetType() string {
	return "websocket"
}

func (c *WebSocketSpectatorConnection) IsConnected() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.connected
}

// Request/Response structures


// CreateUserRequest represents a user creation request
type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email,omitempty"`
}

// RegistrationRequest represents a user registration request
type RegistrationRequest struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	PasswordConfirm string `json:"password_confirm"`
	Email           string `json:"email,omitempty"`
	RealName        string `json:"real_name,omitempty"`
	AcceptTerms     bool   `json:"accept_terms"`
	CaptchaResponse string `json:"captcha_response,omitempty"`
	Source          string `json:"source"` // "ssh", "web", "api"
	IPAddress       string `json:"ip_address,omitempty"`
	UserAgent       string `json:"user_agent,omitempty"`
}

// RegistrationResponse represents a registration response
type RegistrationResponse struct {
	Success              bool              `json:"success"`
	User                 *User             `json:"user,omitempty"`
	Message              string            `json:"message"`
	Errors               []ValidationError `json:"errors,omitempty"`
	RequiresVerification bool              `json:"requires_verification"`
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// CreateSessionRequest represents a session creation request
type CreateSessionRequest struct {
	UserID       int    `json:"user_id"`
	Username     string `json:"username"`
	GameID       string `json:"game_id"`
	TerminalSize string `json:"terminal_size"`
	Encoding     string `json:"encoding"`
}

// StartGameRequest represents a game start request
type StartGameRequest struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	GameID   string `json:"game_id"`
}

// GameSession represents a game session response
type GameSession struct {
	ID       string `json:"id"`
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	GameID   string `json:"game_id"`
	Status   string `json:"status"`
}

// ServiceMetrics represents service metrics
type ServiceMetrics struct {
	ActiveSessions   int   `json:"active_sessions"`
	TotalSessions    int   `json:"total_sessions"`
	ActiveSpectators int   `json:"active_spectators"`
	TotalSpectators  int   `json:"total_spectators"`
	BytesTransferred int64 `json:"bytes_transferred"`
	UptimeSeconds    int64 `json:"uptime_seconds"`
}

// Service interfaces


// UserServiceClient interface for user service
type UserServiceClient interface {
	GetUser(ctx context.Context, username string) (*User, error)
	GetUserByID(ctx context.Context, userID int) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error)
	RegisterUser(ctx context.Context, req *RegistrationRequest) (*RegistrationResponse, error)
	UpdateUser(ctx context.Context, userID int, updates map[string]interface{}) (*User, error)
	DeleteUser(ctx context.Context, userID int) error
	ListUsers(ctx context.Context, limit, offset int) ([]*User, error)
	UpdateLastLogin(ctx context.Context, userID int) error
}

// GameServiceClient interface for game service
type GameServiceClient interface {
	ListGames(ctx context.Context) ([]*Game, error)
	GetGame(ctx context.Context, gameID string) (*Game, error)
	StartGame(ctx context.Context, req *StartGameRequest) (*GameSession, error)
	StopGame(ctx context.Context, sessionID string) error
	GetGameStatus(ctx context.Context, sessionID string) (*GameSession, error)
	UpdateGameConfig(ctx context.Context, gameID string, config *Game) error
}

// Server access control modes
type ServerAccessMode string

const (
	AccessModePublic     ServerAccessMode = "public"      // Anonymous signups allowed
	AccessModeSemiPublic ServerAccessMode = "semi-public" // Invitation keys required
	AccessModePrivate    ServerAccessMode = "private"     // Preloaded keys required
)

// ServerAccessConfig represents server access control configuration
type ServerAccessConfig struct {
	Mode                ServerAccessMode `json:"mode"`
	AllowAnonymous      bool             `json:"allow_anonymous"`
	RequireInviteKey    bool             `json:"require_invite_key"`
	RequirePreloadedKey bool             `json:"require_preloaded_key"`
	MaxUsers            int              `json:"max_users"`
	MaxAnonymousUsers   int              `json:"max_anonymous_users"`
	InviteKeyExpiration string           `json:"invite_key_expiration"`
}

// InviteKey represents an invitation key for semi-public servers
type InviteKey struct {
	ID          string     `json:"id"`
	Key         string     `json:"key"`
	CreatedBy   string     `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	UsedBy      *string    `json:"used_by,omitempty"`
	UsedAt      *time.Time `json:"used_at,omitempty"`
	IsActive    bool       `json:"is_active"`
	MaxUses     int        `json:"max_uses"`
	CurrentUses int        `json:"current_uses"`
	Notes       string     `json:"notes,omitempty"`
}

// PreloadedKey represents a preloaded access key for private servers
type PreloadedKey struct {
	ID        string     `json:"id"`
	Key       string     `json:"key"`
	Username  string     `json:"username"`
	Email     string     `json:"email,omitempty"`
	CreatedBy string     `json:"created_by"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	IsActive  bool       `json:"is_active"`
	Role      string     `json:"role,omitempty"`
	Notes     string     `json:"notes,omitempty"`
}

// AccessControlRequest represents a request for server access
type AccessControlRequest struct {
	Username  string `json:"username"`
	Email     string `json:"email,omitempty"`
	Password  string `json:"password"`
	InviteKey string `json:"invite_key,omitempty"`
	AccessKey string `json:"access_key,omitempty"`
	IPAddress string `json:"ip_address,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
}

// AccessControlResponse represents the response to access control check
type AccessControlResponse struct {
	Allowed      bool   `json:"allowed"`
	Reason       string `json:"reason,omitempty"`
	RequiredRole string `json:"required_role,omitempty"`
	MaxUsers     int    `json:"max_users,omitempty"`
	CurrentUsers int    `json:"current_users,omitempty"`
}

// AccessControlManager interface for managing server access
type AccessControlManager interface {
	CheckAccess(ctx context.Context, req *AccessControlRequest) (*AccessControlResponse, error)
	ValidateInviteKey(ctx context.Context, key string) (*InviteKey, error)
	ValidatePreloadedKey(ctx context.Context, key string) (*PreloadedKey, error)
	CreateInviteKey(ctx context.Context, createdBy string, opts *InviteKeyOptions) (*InviteKey, error)
	CreatePreloadedKey(ctx context.Context, createdBy string, opts *PreloadedKeyOptions) (*PreloadedKey, error)
	RevokeInviteKey(ctx context.Context, keyID string) error
	RevokePreloadedKey(ctx context.Context, keyID string) error
	ListInviteKeys(ctx context.Context, activeOnly bool) ([]*InviteKey, error)
	ListPreloadedKeys(ctx context.Context, activeOnly bool) ([]*PreloadedKey, error)
	GetServerStats(ctx context.Context) (*ServerAccessStats, error)
}

// InviteKeyOptions represents options for creating invite keys
type InviteKeyOptions struct {
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	MaxUses   int        `json:"max_uses"`
	Notes     string     `json:"notes,omitempty"`
}

// PreloadedKeyOptions represents options for creating preloaded keys
type PreloadedKeyOptions struct {
	Username  string     `json:"username"`
	Email     string     `json:"email,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Role      string     `json:"role,omitempty"`
	Notes     string     `json:"notes,omitempty"`
}

// ServerAccessStats represents server access statistics
type ServerAccessStats struct {
	Mode                ServerAccessMode `json:"mode"`
	TotalUsers          int              `json:"total_users"`
	ActiveUsers         int              `json:"active_users"`
	AnonymousUsers      int              `json:"anonymous_users"`
	RegisteredUsers     int              `json:"registered_users"`
	MaxUsers            int              `json:"max_users"`
	ActiveInviteKeys    int              `json:"active_invite_keys"`
	UsedInviteKeys      int              `json:"used_invite_keys"`
	ActivePreloadedKeys int              `json:"active_preloaded_keys"`
	UsedPreloadedKeys   int              `json:"used_preloaded_keys"`
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

// SessionStatistics represents session statistics
type SessionStatistics struct {
	TotalSessions       int           `json:"total_sessions"`
	ActiveSessions      int           `json:"active_sessions"`
	AverageSessionTime  time.Duration `json:"average_session_time"`
	TotalPlayTime       time.Duration `json:"total_play_time"`
	MostPlayedGame      string        `json:"most_played_game"`
	TotalUsers          int           `json:"total_users"`
	ActiveUsers         int           `json:"active_users"`
	PeakConcurrentUsers int           `json:"peak_concurrent_users"`
	UptimePercentage    float64       `json:"uptime_percentage"`
}

// UserStatistics represents user statistics
type UserStatistics struct {
	TotalSessions      int           `json:"total_sessions"`
	TotalPlayTime      time.Duration `json:"total_play_time"`
	AverageSessionTime time.Duration `json:"average_session_time"`
	FavoriteGame       string        `json:"favorite_game"`
	GamesPlayed        []string      `json:"games_played"`
	FirstLogin         time.Time     `json:"first_login"`
	LastLogin          time.Time     `json:"last_login"`
	LoginCount         int           `json:"login_count"`
	Achievements       []string      `json:"achievements"`
	Rank               int           `json:"rank"`
}

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

// Event system for notifications

// Event represents a system event
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Source    string                 `json:"source"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// EventType constants
const (
	EventTypeSessionStart   = "session.start"
	EventTypeSessionEnd     = "session.end"
	EventTypeUserLogin      = "user.login"
	EventTypeUserLogout     = "user.logout"
	EventTypeUserRegister   = "user.register"
	EventTypeGameStart      = "game.start"
	EventTypeGameEnd        = "game.end"
	EventTypeSpectatorJoin  = "spectator.join"
	EventTypeSpectatorLeave = "spectator.leave"
	EventTypeSystemShutdown = "system.shutdown"
	EventTypeSystemStartup  = "system.startup"
)

// EventBus interface for event handling
type EventBus interface {
	Publish(event *Event) error
	Subscribe(eventType string, handler func(*Event)) error
	Unsubscribe(eventType string, handler func(*Event)) error
}

// SimpleEventBus implements EventBus
type SimpleEventBus struct {
	handlers map[string][]func(*Event)
	mutex    sync.RWMutex
}

func NewSimpleEventBus() *SimpleEventBus {
	return &SimpleEventBus{
		handlers: make(map[string][]func(*Event)),
	}
}

func (eb *SimpleEventBus) Publish(event *Event) error {
	eb.mutex.RLock()
	defer eb.mutex.RUnlock()

	if handlers, exists := eb.handlers[event.Type]; exists {
		for _, handler := range handlers {
			go handler(event)
		}
	}

	return nil
}

func (eb *SimpleEventBus) Subscribe(eventType string, handler func(*Event)) error {
	eb.mutex.Lock()
	defer eb.mutex.Unlock()

	eb.handlers[eventType] = append(eb.handlers[eventType], handler)
	return nil
}

func (eb *SimpleEventBus) Unsubscribe(eventType string, handler func(*Event)) error {
	eb.mutex.Lock()
	defer eb.mutex.Unlock()

	// This is a simplified implementation
	// In a real implementation, you'd need to match function pointers
	delete(eb.handlers, eventType)
	return nil
}

// Logger types

// LogLevel represents log level
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// Logger interface for structured logging
type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
}

// SimpleLogger implements Logger interface
type SimpleLogger struct {
	level LogLevel
}

func NewSimpleLogger(level LogLevel) *SimpleLogger {
	return &SimpleLogger{level: level}
}

func (l *SimpleLogger) Debug(msg string, fields ...interface{}) {
	if l.level <= LogLevelDebug {
		log.Printf("[DEBUG] %s %v", msg, fields)
	}
}

func (l *SimpleLogger) Info(msg string, fields ...interface{}) {
	if l.level <= LogLevelInfo {
		log.Printf("[INFO] %s %v", msg, fields)
	}
}

func (l *SimpleLogger) Warn(msg string, fields ...interface{}) {
	if l.level <= LogLevelWarn {
		log.Printf("[WARN] %s %v", msg, fields)
	}
}

func (l *SimpleLogger) Error(msg string, fields ...interface{}) {
	if l.level <= LogLevelError {
		log.Printf("[ERROR] %s %v", msg, fields)
	}
}

// Validation functions


// ValidateCreateUserRequest validates a create user request
func ValidateCreateUserRequest(req *CreateUserRequest) []*ValidationError {
	var errors []*ValidationError

	if req.Username == "" {
		errors = append(errors, &ValidationError{
			Field:   "username",
			Message: "username is required",
		})
	}

	if req.Password == "" {
		errors = append(errors, &ValidationError{
			Field:   "password",
			Message: "password is required",
		})
	}

	if len(req.Username) < 3 || len(req.Username) > 32 {
		errors = append(errors, &ValidationError{
			Field:   "username",
			Message: "username must be between 3 and 32 characters long",
		})
	}

	if len(req.Password) < 6 {
		errors = append(errors, &ValidationError{
			Field:   "password",
			Message: "password must be at least 6 characters long",
		})
	}

	// Basic username validation (alphanumeric + underscore)
	for _, char := range req.Username {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '_') {
			errors = append(errors, &ValidationError{
				Field:   "username",
				Message: "username can only contain letters, numbers, and underscores",
			})
			break
		}
	}

	return errors
}

// Configuration helpers

// GetDefaultSSHConfig returns default SSH configuration
func GetDefaultSSHConfig() *config.SSHConfig {
	return &config.SSHConfig{
		Enabled:        true,
		Port:           22,
		Host:           "0.0.0.0",
		HostKeyPath:    "/etc/ssh/ssh_host_rsa_key",
		Banner:         "Welcome to dungeongate!\r\n",
		MaxSessions:    100,
		SessionTimeout: "4h",
		IdleTimeout:    "30m",
		Auth: &config.SSHAuthConfig{
			PasswordAuth:   true,
			PublicKeyAuth:  false,
			AllowAnonymous: true,
		},
		Terminal: &config.SSHTerminalConfig{
			DefaultSize:        "80x24",
			MaxSize:            "200x50",
			SupportedTerminals: []string{"xterm", "xterm-256color", "screen", "tmux", "vt100"},
		},
	}
}

// GetDefaultMenuConfig returns default menu configuration
func GetDefaultMenuConfig() *config.MenuConfig {
	return &config.MenuConfig{
		Banners: &config.BannersConfig{
			MainAnon:  "/etc/dungeongate/banners/main_anon.txt",
			MainUser:  "/etc/dungeongate/banners/main_user.txt",
			WatchMenu: "/etc/dungeongate/banners/watch_menu.txt",
		},
		Options: &config.MenuOptions{
			Anonymous: []*config.MenuOption{
				{Key: "l", Label: "Login", Action: "login"},
				{Key: "r", Label: "Register", Action: "register"},
				{Key: "w", Label: "Watch games", Action: "watch"},
				{Key: "g", Label: "List games", Action: "list_games"},
				{Key: "q", Label: "Quit", Action: "quit"},
			},
			Authenticated: []*config.MenuOption{
				{Key: "p", Label: "Play a game", Action: "play"},
				{Key: "w", Label: "Watch games", Action: "watch"},
				{Key: "e", Label: "Edit profile", Action: "edit_profile"},
				{Key: "l", Label: "List games", Action: "list_games"},
				{Key: "r", Label: "View recordings", Action: "recordings"},
				{Key: "s", Label: "Statistics", Action: "stats"},
				{Key: "q", Label: "Quit", Action: "quit"},
			},
		},
	}
}

// Security helpers

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	// This would use bcrypt in a real implementation
	return fmt.Sprintf("hashed_%s", password), nil
}

// VerifyPassword verifies a password against a hash
func VerifyPassword(password, hash string) bool {
	// This would use bcrypt in a real implementation
	return hash == fmt.Sprintf("hashed_%s", password)
}

