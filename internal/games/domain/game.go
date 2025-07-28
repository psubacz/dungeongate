package domain

import (
	"time"
)

// Game aggregate root represents a game configuration and management
type Game struct {
	// Identity
	id       GameID
	name     string
	metadata GameMetadata

	// Configuration
	config GameConfig

	// State
	status     GameStatus
	statistics GameStatistics

	// Audit
	createdAt time.Time
	updatedAt time.Time
}

// GameID represents a unique game identifier
type GameID struct {
	value string
}

// NewGameID creates a new game ID
func NewGameID(value string) GameID {
	return GameID{value: value}
}

// String returns the string representation of the game ID
func (id GameID) String() string {
	return id.value
}

// GameMetadata contains descriptive information about a game
type GameMetadata struct {
	Name        string
	ShortName   string
	Description string
	Category    string
	Tags        []string
	Version     string
	Difficulty  int // 1-10 scale
}

// GameConfig contains technical configuration for a game
type GameConfig struct {
	Binary      BinaryConfig
	Environment map[string]string
	Resources   ResourceConfig
	Security    SecurityConfig
	Networking  NetworkConfig
}

// BinaryConfig defines how to execute the game binary
type BinaryConfig struct {
	Path             string
	Args             []string
	WorkingDirectory string
}

// ResourceConfig defines resource limits for the game
type ResourceConfig struct {
	CPULimit    string
	MemoryLimit string
	DiskLimit   string
	Timeout     time.Duration
}

// SecurityConfig defines security settings for the game
type SecurityConfig struct {
	RunAsUser                uint32
	RunAsGroup               uint32
	ReadOnlyRootFilesystem   bool
	AllowPrivilegeEscalation bool
	Capabilities             []string
}

// NetworkConfig defines networking settings for the game
type NetworkConfig struct {
	Isolated       bool
	AllowedPorts   []int
	AllowedDomains []string
	BlockInternet  bool
}

// GameStatus represents the current status of a game
type GameStatus string

const (
	GameStatusEnabled     GameStatus = "enabled"
	GameStatusDisabled    GameStatus = "disabled"
	GameStatusMaintenance GameStatus = "maintenance"
	GameStatusDeprecated  GameStatus = "deprecated"
)

// GameStatistics tracks usage statistics for a game
type GameStatistics struct {
	TotalSessions      int
	ActiveSessions     int
	TotalPlayTime      time.Duration
	AverageSessionTime time.Duration
	UniqueUsers        int
	LastPlayed         *time.Time
	PopularityRank     int
	Rating             float32
}

// NewGame creates a new game aggregate
func NewGame(id GameID, metadata GameMetadata, config GameConfig) *Game {
	now := time.Now()
	return &Game{
		id:         id,
		metadata:   metadata,
		config:     config,
		status:     GameStatusEnabled,
		statistics: GameStatistics{},
		createdAt:  now,
		updatedAt:  now,
	}
}

// ID returns the game's ID
func (g *Game) ID() GameID {
	return g.id
}

// Metadata returns the game's metadata
func (g *Game) Metadata() GameMetadata {
	return g.metadata
}

// Config returns the game's configuration
func (g *Game) Config() GameConfig {
	return g.config
}

// Status returns the game's current status
func (g *Game) Status() GameStatus {
	return g.status
}

// Statistics returns the game's statistics
func (g *Game) Statistics() GameStatistics {
	return g.statistics
}

// Enable enables the game
func (g *Game) Enable() {
	g.status = GameStatusEnabled
	g.updatedAt = time.Now()
}

// Disable disables the game
func (g *Game) Disable() {
	g.status = GameStatusDisabled
	g.updatedAt = time.Now()
}

// SetMaintenance sets the game to maintenance mode
func (g *Game) SetMaintenance() {
	g.status = GameStatusMaintenance
	g.updatedAt = time.Now()
}

// UpdateConfig updates the game's configuration
func (g *Game) UpdateConfig(config GameConfig) {
	g.config = config
	g.updatedAt = time.Now()
}

// UpdateMetadata updates the game's metadata
func (g *Game) UpdateMetadata(metadata GameMetadata) {
	g.metadata = metadata
	g.updatedAt = time.Now()
}

// IsEnabled returns true if the game is enabled
func (g *Game) IsEnabled() bool {
	return g.status == GameStatusEnabled
}

// CanStart returns true if the game can be started
func (g *Game) CanStart() bool {
	return g.status == GameStatusEnabled
}

// UpdateStatistics updates the game's statistics
func (g *Game) UpdateStatistics(stats GameStatistics) {
	g.statistics = stats
	g.updatedAt = time.Now()
}

// IncrementPlayCount increments the play count and updates last played time
func (g *Game) IncrementPlayCount() {
	g.statistics.TotalSessions++
	now := time.Now()
	g.statistics.LastPlayed = &now
	g.updatedAt = now
}

// CreatedAt returns when the game was created
func (g *Game) CreatedAt() time.Time {
	return g.createdAt
}

// UpdatedAt returns when the game was last updated
func (g *Game) UpdatedAt() time.Time {
	return g.updatedAt
}
