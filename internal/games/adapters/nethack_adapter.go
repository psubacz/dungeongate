package adapters

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dungeongate/internal/games/domain"
	"github.com/dungeongate/pkg/config"
)

// NetHackAdapter handles NetHack-specific setup and configuration
type NetHackAdapter struct {
	config *config.GameConfig
	logger *slog.Logger
}

// NewNetHackAdapter creates a new NetHack adapter
func NewNetHackAdapter(logger *slog.Logger) *NetHackAdapter {
	if logger == nil {
		// Create a default logger if none provided
		logger = slog.Default().With("component", "nethack-adapter")
	}
	return &NetHackAdapter{
		logger: logger.With("adapter", "nethack"),
	}
}

// GetGameID returns the game ID this adapter handles
func (a *NetHackAdapter) GetGameID() string {
	return "nethack"
}

// Configure sets up the adapter with game-specific configuration
func (a *NetHackAdapter) Configure(gameConfig *config.GameConfig) error {
	if gameConfig == nil {
		return fmt.Errorf("configuration cannot be nil")
	}
	if gameConfig.ID != "nethack" {
		return fmt.Errorf("invalid game ID: expected 'nethack', got '%s'", gameConfig.ID)
	}
	a.config = gameConfig
	return nil
}

// PrepareCommand sets up the NetHack command with proper configuration
func (a *NetHackAdapter) PrepareCommand(ctx context.Context, session *domain.GameSession, gamePath string, baseArgs []string, baseEnv []string) (*exec.Cmd, error) {
	if a.config == nil {
		return nil, fmt.Errorf("adapter not configured - call Configure() first")
	}

	// Extract username from session
	username := fmt.Sprintf("user_%d", session.UserID().Int()) // Create a safe username

	// NetHack-specific arguments - just the username
	args := []string{"-u", username}

	// Enhanced environment for NetHack with configuration-driven paths
	homeDir := fmt.Sprintf("/tmp/nethack-users/%s", username)
	userGameDir := fmt.Sprintf("%s/%s", homeDir, a.config.Paths.User.BaseDir)

	// Get system path from configuration
	systemPath := "/opt/homebrew/Cellar/nethack/3.6.7/libexec" // Default fallback
	if a.config.Paths != nil && a.config.Paths.System != nil && a.config.Paths.System.SysConfFile != "" {
		systemPath = filepath.Dir(a.config.Paths.System.SysConfFile)
	}

	env := append(os.Environ(),
		// Terminal configuration - use basic xterm for better compatibility
		fmt.Sprintf("TERM=%s", "xterm"),
		fmt.Sprintf("USER=%s", username),
		fmt.Sprintf("LOGNAME=%s", username),
		fmt.Sprintf("HOME=%s", homeDir),
		fmt.Sprintf("COLUMNS=%d", session.TerminalSize().Width),
		fmt.Sprintf("LINES=%d", session.TerminalSize().Height),

		// NetHack path configuration from game-service.yaml
		fmt.Sprintf("NETHACKDIR=%s", systemPath),
		fmt.Sprintf("HACKDIR=%s", systemPath),
		fmt.Sprintf("NETHACK_PLAYGROUND=%s", userGameDir),
		fmt.Sprintf("NETHACK_SAVEDIR=%s/%s", homeDir, a.config.Paths.User.SaveDir),
		fmt.Sprintf("NETHACK_LEVELDIR=%s/%s", homeDir, a.config.Paths.User.LevelDir),
		fmt.Sprintf("NETHACK_BONESDIR=%s/%s", homeDir, a.config.Paths.User.BonesDir),
		fmt.Sprintf("NETHACK_LOCKDIR=%s/%s", homeDir, a.config.Paths.User.LockDir),
		fmt.Sprintf("NETHACK_TROUBLEDIR=%s/%s", homeDir, a.config.Paths.User.TroubleDir),
		fmt.Sprintf("NETHACK_CONFIGDIR=%s/%s", homeDir, a.config.Paths.User.ConfigDir),
	)

	// Create the command without context binding to prevent process termination
	// when gRPC contexts are cancelled. NetHack should run independently.
	cmd := exec.Command(gamePath, args...)
	cmd.Env = env

	// Set working directory from configuration
	if a.config.Binary != nil && a.config.Binary.WorkingDirectory != "" {
		cmd.Dir = a.config.Binary.WorkingDirectory
	}
	// Note: SysProcAttr will be set by PTY manager using StartWithAttrs

	a.logger.Debug("NetHack adapter prepared command",
		"path", gamePath,
		"args", args,
		"working_dir", cmd.Dir,
		"username", username)

	return cmd, nil
}

// GetInitialInput returns initial input to send to NetHack after startup
func (a *NetHackAdapter) GetInitialInput() []byte {
	// NetHack character creation sequence:
	// 1. If prompted "Shall I pick a character's race, role, gender and alignment for you?"
	//    Answer 'n' to pick ourselves
	// 2. Select race, role, etc. manually for now
	// For now, let the user manually handle character creation to avoid input conflicts
	return nil
}

// ProcessOutput processes NetHack output for any game-specific handling
func (a *NetHackAdapter) ProcessOutput(data []byte) []byte {
	// NetHack output processing
	output := string(data)

	// Log all NetHack output for debugging
	if len(output) > 2 || (len(output) <= 2 && output != "\r\n") {
		a.logger.Debug("NetHack output", "bytes", len(data), "content", output)
	}

	// Handle common NetHack startup messages
	if strings.Contains(output, "Shall I pick a character") {
		a.logger.Debug("NetHack character selection prompt detected")
	}

	if strings.Contains(output, "Welcome to NetHack!") {
		a.logger.Debug("NetHack welcome message detected")
	}

	if strings.Contains(output, "It is written in the Book of the Dead") {
		a.logger.Debug("NetHack intro text detected")
	}

	if strings.Contains(output, "restoring") {
		a.logger.Debug("NetHack save game restoration detected")
	}

	// Check for character creation prompts
	if strings.Contains(output, "What is your name?") {
		a.logger.Debug("NetHack name prompt detected")
	}

	// Return the data as-is
	return data
}

// IsGameReady determines if NetHack has finished initializing
func (a *NetHackAdapter) IsGameReady(output []byte) bool {
	outputStr := string(output)

	// NetHack is ready when we see:
	// - The main game interface
	// - Character selection prompts
	// - Welcome messages
	ready := strings.Contains(outputStr, "@") || // Player symbol
		strings.Contains(outputStr, "Welcome to NetHack") ||
		strings.Contains(outputStr, "Shall I pick a character") ||
		strings.Contains(outputStr, "Choose one of the following") ||
		len(outputStr) > 50 // Assume ready if we get substantial output

	if ready {
		a.logger.Debug("NetHack appears ready", "output", outputStr)
	}

	return ready
}

// GetRequiredFiles returns files that must exist for NetHack to run
func (a *NetHackAdapter) GetRequiredFiles() []string {
	// NetHack handles its own file requirements
	return []string{}
}

// SetupGameEnvironment performs NetHack-specific pre-game setup
func (a *NetHackAdapter) SetupGameEnvironment(session *domain.GameSession) error {
	if a.config == nil {
		return fmt.Errorf("adapter not configured - call Configure() first")
	}

	username := fmt.Sprintf("user_%d", session.UserID().Int())
	homeDir := fmt.Sprintf("/tmp/nethack-users/%s", username)

	// Create all required NetHack directories using configuration
	directories := []string{
		homeDir,
		fmt.Sprintf("%s/%s", homeDir, a.config.Paths.User.BaseDir),
		fmt.Sprintf("%s/%s", homeDir, a.config.Paths.User.SaveDir),
		fmt.Sprintf("%s/%s", homeDir, a.config.Paths.User.LevelDir),
		fmt.Sprintf("%s/%s", homeDir, a.config.Paths.User.BonesDir),
		fmt.Sprintf("%s/%s", homeDir, a.config.Paths.User.LockDir),
		fmt.Sprintf("%s/%s", homeDir, a.config.Paths.User.TroubleDir),
		fmt.Sprintf("%s/%s", homeDir, a.config.Paths.User.ConfigDir),
	}

	for _, dir := range directories {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create NetHack directory %s: %w", dir, err)
		}
	}

	a.logger.Debug("NetHack environment setup completed",
		"username", username,
		"home_dir", homeDir,
		"game_dir", fmt.Sprintf("%s/%s", homeDir, a.config.Paths.User.BaseDir),
		"directories_created", len(directories))
	return nil
}

// CleanupGameEnvironment performs NetHack-specific post-game cleanup
func (a *NetHackAdapter) CleanupGameEnvironment(session *domain.GameSession) error {
	// For now, we don't need to clean up much
	// In the future, we might want to backup saves, clean temp files, etc.
	a.logger.Debug("NetHack cleanup completed", "session_id", session.ID().String())
	return nil
}
