package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// GameDirectoryManager manages game directories and configuration
type GameDirectoryManager struct {
	BaseDir      string                   // Base directory for all game data
	UserDirs     map[string]*UserGameDirs // Per-user directory structure
	TempDirs     map[string]string        // Temporary directories for active games
	CleanupQueue []string                 // Directories pending cleanup
	pathDetector *PathDetector            // Path detector for system paths
	mutex        sync.RWMutex             // Protects concurrent access
	logger       *log.Logger
}

// NewGameDirectoryManager creates a new GameDirectoryManager
func NewGameDirectoryManager(baseDir string, logger *log.Logger) *GameDirectoryManager {
	return &GameDirectoryManager{
		BaseDir:      baseDir,
		UserDirs:     make(map[string]*UserGameDirs),
		TempDirs:     make(map[string]string),
		CleanupQueue: make([]string, 0),
		pathDetector: NewPathDetector(),
		logger:       logger,
	}
}

// GetUserDirs returns the directory structure for a user
func (gm *GameDirectoryManager) GetUserDirs(userID int) *UserGameDirs {
	gm.mutex.RLock()
	defer gm.mutex.RUnlock()

	userKey := fmt.Sprintf("user_%d", userID)
	if dirs, exists := gm.UserDirs[userKey]; exists {
		return dirs
	}

	// Create new user directory structure
	baseDir := filepath.Join(gm.BaseDir, "users", fmt.Sprintf("%d", userID))
	userDirs := &UserGameDirs{
		UserID:     userID,
		BaseDir:    baseDir,
		SaveDir:    filepath.Join(baseDir, "saves"),
		ConfigDir:  filepath.Join(baseDir, "config"),
		BonesDir:   filepath.Join(baseDir, "bones"),
		LevelDir:   filepath.Join(baseDir, "levels"),
		LockDir:    filepath.Join(baseDir, "locks"),
		TroubleDir: filepath.Join(baseDir, "trouble"),
	}

	gm.UserDirs[userKey] = userDirs
	return userDirs
}

// SetupGamePaths sets up game paths for a specific user and game
func (gm *GameDirectoryManager) SetupGamePaths(userID int, gameID string, options *GameSetupOptions) (*NetHackPaths, error) {
	userDirs := gm.GetUserDirs(userID)

	// Auto-detect system paths if requested
	var systemPaths *NetHackSystemPaths
	var err error
	if options.DetectSystemPaths {
		systemPaths, err = gm.pathDetector.DetectNetHackPaths()
		if err != nil {
			return nil, fmt.Errorf("failed to detect system paths: %w", err)
		}
	}

	// Create user directories if requested
	if options.CreateUserDirs {
		if err := gm.createUserDirectories(userDirs); err != nil {
			return nil, fmt.Errorf("failed to create user directories: %w", err)
		}
	}

	// Create game-specific temporary directories
	tempDir := filepath.Join(userDirs.BaseDir, "temp", gameID)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	paths := &NetHackPaths{
		// User-specific paths
		HackDir:    userDirs.BaseDir,
		SaveDir:    userDirs.SaveDir,
		ConfigDir:  userDirs.ConfigDir,
		BonesDir:   userDirs.BonesDir,
		LevelDir:   tempDir, // Game-specific temp level dir
		LockDir:    userDirs.LockDir,
		TroubleDir: userDirs.TroubleDir,

		// System paths (use detected paths if available, fallback to defaults)
		ScoreDir:    gm.getSystemPath(systemPaths, "scoredir", "/opt/homebrew/share/nethack/"),
		SysConfDir:  gm.getSystemPath(systemPaths, "sysconf", "/opt/homebrew/Cellar/nethack/3.6.7/libexec/sysconf"),
		SymbolsFile: gm.getSystemPath(systemPaths, "symbols", "/opt/homebrew/Cellar/nethack/3.6.7/libexec/symbols"),
		DataFile:    gm.getSystemPath(systemPaths, "datafile", "nhdat"),
		UserConfig:  gm.getSystemPath(systemPaths, "userconfig", filepath.Join(userDirs.ConfigDir, ".nethackrc")),
	}

	// Create save directory symlinks if requested
	if options.CreateSaveLinks {
		if err := gm.createSaveSymlinks(paths, userDirs); err != nil {
			return nil, fmt.Errorf("failed to create save symlinks: %w", err)
		}
	}

	// Copy default config if requested
	if options.CopyDefaultConfig {
		if err := gm.copyDefaultConfig(userDirs); err != nil {
			return nil, fmt.Errorf("failed to copy default config: %w", err)
		}
	}

	// Set permissions if requested
	if options.SetPermissions {
		if err := gm.setPermissions(userDirs); err != nil {
			return nil, fmt.Errorf("failed to set permissions: %w", err)
		}
	}

	// Register for cleanup
	gm.mutex.Lock()
	gm.TempDirs[gameID] = tempDir
	gm.mutex.Unlock()

	return paths, nil
}

// createUserDirectories creates all user directories
func (gm *GameDirectoryManager) createUserDirectories(userDirs *UserGameDirs) error {
	dirs := []string{
		userDirs.BaseDir,
		userDirs.SaveDir,
		userDirs.ConfigDir,
		userDirs.BonesDir,
		userDirs.LevelDir,
		userDirs.LockDir,
		userDirs.TroubleDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// getSystemPath returns the system path if available, otherwise returns default
func (gm *GameDirectoryManager) getSystemPath(systemPaths *NetHackSystemPaths, pathType, defaultValue string) string {
	if systemPaths == nil {
		return defaultValue
	}

	switch pathType {
	case "scoredir":
		if systemPaths.ScoreDir != "" {
			return systemPaths.ScoreDir
		}
	case "sysconf":
		if systemPaths.SysConfFile != "" {
			return systemPaths.SysConfFile
		}
	case "symbols":
		if systemPaths.SymbolsFile != "" {
			return systemPaths.SymbolsFile
		}
	case "datafile":
		if systemPaths.DataFile != "" {
			return systemPaths.DataFile
		}
	case "userconfig":
		if systemPaths.UserConfig != "" {
			return systemPaths.UserConfig
		}
	}

	return defaultValue
}

// createSaveSymlinks creates symlinks for save directories
func (gm *GameDirectoryManager) createSaveSymlinks(paths *NetHackPaths, userDirs *UserGameDirs) error {
	// Create symlink from system save directory to user save directory
	systemSaveDir := filepath.Join(filepath.Dir(paths.SysConfDir), "save")
	userSaveLink := filepath.Join(systemSaveDir, fmt.Sprintf("user_%d", userDirs.UserID))

	// Remove existing symlink if it exists
	if _, err := os.Lstat(userSaveLink); err == nil {
		if err := os.Remove(userSaveLink); err != nil {
			return fmt.Errorf("failed to remove existing symlink: %w", err)
		}
	}

	// Create new symlink
	if err := os.Symlink(userDirs.SaveDir, userSaveLink); err != nil {
		return fmt.Errorf("failed to create save symlink: %w", err)
	}

	return nil
}

// copyDefaultConfig copies default configuration files
func (gm *GameDirectoryManager) copyDefaultConfig(userDirs *UserGameDirs) error {
	// Create a basic .nethackrc file
	nethackrcPath := filepath.Join(userDirs.ConfigDir, ".nethackrc")

	// Check if file already exists
	if _, err := os.Stat(nethackrcPath); err == nil {
		gm.logger.Printf("Config file already exists: %s", nethackrcPath)
		return nil
	}

	// Basic NetHack configuration
	defaultConfig := `# DungeonGate NetHack Configuration
OPTIONS=color,DECgraphics,!autopickup,!cmdassist,!rest_on_space
OPTIONS=!news,!legacy,!mail,time,showexp,showscore,toptenwin
OPTIONS=hilite_pet,hilite_pile,showrace,showgender,!sparkle
OPTIONS=menucolors,statushilites,paranoid_confirmation:quit
OPTIONS=pickup_burden:burdened
OPTIONS=msg_window:full
OPTIONS=windowtype:tty
`

	if err := os.WriteFile(nethackrcPath, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("failed to write default config: %w", err)
	}

	gm.logger.Printf("Created default config file: %s", nethackrcPath)
	return nil
}

// setPermissions sets appropriate permissions on directories
func (gm *GameDirectoryManager) setPermissions(userDirs *UserGameDirs) error {
	dirs := []string{
		userDirs.BaseDir,
		userDirs.SaveDir,
		userDirs.ConfigDir,
		userDirs.BonesDir,
		userDirs.LevelDir,
		userDirs.LockDir,
		userDirs.TroubleDir,
	}

	for _, dir := range dirs {
		if err := os.Chmod(dir, 0755); err != nil {
			return fmt.Errorf("failed to set permissions on %s: %w", dir, err)
		}
	}

	return nil
}
