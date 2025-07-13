package adapters

import (
	"context"
	"os/exec"

	"github.com/dungeongate/internal/games/domain"
	"github.com/dungeongate/pkg/config"
)

// DefaultAdapter provides a basic implementation for games that don't need special handling
type DefaultAdapter struct {
	gameID string
}

// NewDefaultAdapter creates a new default adapter
func NewDefaultAdapter(gameID string) *DefaultAdapter {
	return &DefaultAdapter{
		gameID: gameID,
	}
}

// GetGameID returns the game ID this adapter handles
func (a *DefaultAdapter) GetGameID() string {
	return a.gameID
}

// Configure does nothing for default adapters
func (a *DefaultAdapter) Configure(config *config.GameConfig) error {
	// Default adapter doesn't need configuration
	return nil
}

// PrepareCommand sets up a basic command
func (a *DefaultAdapter) PrepareCommand(ctx context.Context, session *domain.GameSession, gamePath string, baseArgs []string, baseEnv []string) (*exec.Cmd, error) {
	// Create command without context binding to prevent process termination
	// when gRPC contexts are cancelled. Games should run independently.
	cmd := exec.Command(gamePath, baseArgs...)
	cmd.Env = baseEnv
	return cmd, nil
}

// GetInitialInput returns no initial input for default games
func (a *DefaultAdapter) GetInitialInput() []byte {
	return nil
}

// ProcessOutput returns output as-is
func (a *DefaultAdapter) ProcessOutput(data []byte) []byte {
	return data
}

// IsGameReady assumes game is ready immediately
func (a *DefaultAdapter) IsGameReady(output []byte) bool {
	return true
}

// GetRequiredFiles returns no required files
func (a *DefaultAdapter) GetRequiredFiles() []string {
	return nil
}

// SetupGameEnvironment does nothing for default games
func (a *DefaultAdapter) SetupGameEnvironment(session *domain.GameSession) error {
	return nil
}

// CleanupGameEnvironment does nothing for default games
func (a *DefaultAdapter) CleanupGameEnvironment(session *domain.GameSession) error {
	return nil
}
