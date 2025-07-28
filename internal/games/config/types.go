package config

import (
	"time"
)

// NetHackPaths represents the configuration paths for NetHack
type NetHackPaths struct {
	// Variable playground locations (customizable per user/game)
	HackDir    string `json:"hackdir"`    // User-specific game directory
	LevelDir   string `json:"leveldir"`   // Level save directory
	SaveDir    string `json:"savedir"`    // Save game directory
	BonesDir   string `json:"bonesdir"`   // Bones files directory
	DataDir    string `json:"datadir"`    // Game data directory
	LockDir    string `json:"lockdir"`    // Lock files directory
	ConfigDir  string `json:"configdir"`  // User config directory
	TroubleDir string `json:"troubledir"` // Debug/trouble directory

	// Fixed system paths (read-only)
	ScoreDir    string `json:"scoredir"`   // "/opt/homebrew/share/nethack/"
	SysConfDir  string `json:"sysconfdir"` // "/opt/homebrew/Cellar/nethack/3.6.7/libexec/sysconf"
	SymbolsFile string `json:"symbols"`    // "/opt/homebrew/Cellar/nethack/3.6.7/libexec/symbols"
	DataFile    string `json:"datafile"`   // "nhdat" (basic data files)
	UserConfig  string `json:"userconfig"` // "~/.nethackrc" (personal config)
}

// GameSetupOptions defines options for setting up game directories
type GameSetupOptions struct {
	CreateUserDirs    bool `json:"create_user_dirs"`    // Create user-specific directories
	CopyDefaultConfig bool `json:"copy_default_config"` // Copy default .nethackrc
	InitializeShared  bool `json:"initialize_shared"`   // Setup shared bones/data
	ValidatePaths     bool `json:"validate_paths"`      // Validate all paths exist
	SetPermissions    bool `json:"set_permissions"`     // Set correct file permissions
	DetectSystemPaths bool `json:"detect_system_paths"` // Auto-detect system paths via --showpaths
	CreateSaveLinks   bool `json:"create_save_links"`   // Create save directory symlinks
}

// GameCleanupOptions defines options for cleaning up game directories
type GameCleanupOptions struct {
	RemoveUserDirs     bool `json:"remove_user_dirs"`     // Delete user directories
	ClearTempFiles     bool `json:"clear_temp_files"`     // Clear temporary game files
	RemoveLockFiles    bool `json:"remove_lock_files"`    // Remove stale lock files
	ClearPersonalBones bool `json:"clear_personal_bones"` // Clear user's personal bones
	PreserveConfig     bool `json:"preserve_config"`      // Keep user config files
	BackupSaves        bool `json:"backup_saves"`         // Backup saves before cleanup
	CleanupSaveLinks   bool `json:"cleanup_save_links"`   // Remove save directory symlinks
	ValidateCleanup    bool `json:"validate_cleanup"`     // Verify cleanup completion
}

// NetHackSystemPaths represents system paths detected from NetHack
type NetHackSystemPaths struct {
	// Detected from `nethack --showpaths` command
	ScoreDir    string `json:"scoredir"`   // "/opt/homebrew/share/nethack/"
	SysConfFile string `json:"sysconf"`    // "/opt/homebrew/Cellar/nethack/3.6.7/libexec/sysconf"
	SymbolsFile string `json:"symbols"`    // "/opt/homebrew/Cellar/nethack/3.6.7/libexec/symbols"
	DataFile    string `json:"datafile"`   // "nhdat"
	UserConfig  string `json:"userconfig"` // "/Users/caboose/.nethackrc"

	// Variable paths (customizable, typically "not set")
	HackDir    string `json:"hackdir"`    // User-specific game directory
	LevelDir   string `json:"leveldir"`   // Level save directory
	SaveDir    string `json:"savedir"`    // Save game directory
	BonesDir   string `json:"bonesdir"`   // Bones files directory
	DataDir    string `json:"datadir"`    // Game data directory
	LockDir    string `json:"lockdir"`    // Lock files directory
	ConfigDir  string `json:"configdir"`  // User config directory
	TroubleDir string `json:"troubledir"` // Debug/trouble directory
}

// UserGameDirs represents per-user directory structure
type UserGameDirs struct {
	UserID     int    `json:"user_id"`
	BaseDir    string `json:"base_dir"`    // /data/users/{userID}
	SaveDir    string `json:"save_dir"`    // /data/users/{userID}/saves
	ConfigDir  string `json:"config_dir"`  // /data/users/{userID}/config
	BonesDir   string `json:"bones_dir"`   // /data/users/{userID}/bones (personal bones)
	LevelDir   string `json:"level_dir"`   // /data/users/{userID}/levels (temp levels)
	LockDir    string `json:"lock_dir"`    // /data/users/{userID}/locks
	TroubleDir string `json:"trouble_dir"` // /data/users/{userID}/trouble
}

// GameConfigError represents a configuration error
type GameConfigError struct {
	ErrorType   string         `json:"error_type"`
	Message     string         `json:"message"`
	Path        string         `json:"path,omitempty"`
	Details     map[string]any `json:"details,omitempty"`
	Recoverable bool           `json:"recoverable"`
	Suggestions []string       `json:"suggestions,omitempty"`
}

func (e *GameConfigError) Error() string {
	return e.Message
}

// CachedConfig represents a cached configuration
type CachedConfig struct {
	Config    *NetHackPaths `json:"config"`
	CachedAt  time.Time     `json:"cached_at"`
	ExpiresAt time.Time     `json:"expires_at"`
	HitCount  int64         `json:"hit_count"`
}

// CacheEntry represents an entry in the cache eviction queue
type CacheEntry struct {
	Key       string    `json:"key"`
	ExpiresAt time.Time `json:"expires_at"`
}
