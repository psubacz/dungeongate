package adapters

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/dungeongate/internal/games/domain"
	"github.com/dungeongate/pkg/config"
)

// GameAdapter defines the interface for game-specific adapters
type GameAdapter interface {
	// GetGameID returns the game ID this adapter handles
	GetGameID() string

	// Configure sets up the adapter with game-specific configuration
	Configure(config *config.GameConfig) error

	// PrepareCommand sets up the command with game-specific configuration
	PrepareCommand(ctx context.Context, session *domain.GameSession, gamePath string, baseArgs []string, baseEnv []string) (*exec.Cmd, error)

	// GetInitialInput returns any initial input that should be sent to the game after startup
	GetInitialInput() []byte

	// ProcessOutput processes raw game output and potentially modifies it
	ProcessOutput(data []byte) []byte

	// IsGameReady determines if the game has finished initializing and is ready for user input
	IsGameReady(output []byte) bool

	// GetRequiredFiles returns a list of files that must exist for the game to run
	GetRequiredFiles() []string

	// SetupGameEnvironment performs any pre-game setup (creating config files, etc.)
	SetupGameEnvironment(session *domain.GameSession) error

	// CleanupGameEnvironment performs any post-game cleanup
	CleanupGameEnvironment(session *domain.GameSession) error
}

// GameAdapterRegistry manages game adapters
type GameAdapterRegistry struct {
	adapters map[string]GameAdapter
}

// NewGameAdapterRegistry creates a new adapter registry
func NewGameAdapterRegistry() *GameAdapterRegistry {
	registry := &GameAdapterRegistry{
		adapters: make(map[string]GameAdapter),
	}

	// Register built-in adapters (without configuration)
	registry.Register(NewNetHackAdapter(nil))

	return registry
}

// NewGameAdapterRegistryWithConfig creates a new adapter registry with configuration
func NewGameAdapterRegistryWithConfig(gameConfigs []*config.GameConfig) (*GameAdapterRegistry, error) {
	registry := &GameAdapterRegistry{
		adapters: make(map[string]GameAdapter),
	}

	// Create a map of configs by game ID for easy lookup
	configMap := make(map[string]*config.GameConfig)
	for _, cfg := range gameConfigs {
		if cfg != nil && cfg.Enabled {
			configMap[cfg.ID] = cfg
		}
	}

	// Register and configure built-in adapters
	if nethackConfig, exists := configMap["nethack"]; exists {
		adapter := NewNetHackAdapter(nil)
		if err := adapter.Configure(nethackConfig); err != nil {
			return nil, fmt.Errorf("failed to configure NetHack adapter: %w", err)
		}
		registry.Register(adapter)
	}

	return registry, nil
}

// Register registers a new game adapter
func (r *GameAdapterRegistry) Register(adapter GameAdapter) {
	r.adapters[adapter.GetGameID()] = adapter
}

// GetAdapter returns the adapter for a specific game ID
func (r *GameAdapterRegistry) GetAdapter(gameID string) GameAdapter {
	if adapter, exists := r.adapters[gameID]; exists {
		return adapter
	}
	// Return default adapter if none found
	return NewDefaultAdapter(gameID)
}

// HasAdapter checks if an adapter exists for the given game ID
func (r *GameAdapterRegistry) HasAdapter(gameID string) bool {
	_, exists := r.adapters[gameID]
	return exists
}
