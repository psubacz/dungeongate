package saves

import (
	"context"
	"time"
)

// SaveRepository defines the interface for save data persistence
type SaveRepository interface {
	// Save operations
	GetSave(ctx context.Context, userID int, gameID string) (*UserSave, error)
	SaveGameData(ctx context.Context, userID int, gameID string, data []byte, metadata map[string]string) error
	DeleteSave(ctx context.Context, userID int, gameID string) error
	ListSaves(ctx context.Context, userID int) ([]*UserSave, error)

	// Backup operations
	CreateBackup(ctx context.Context, userID int, gameID string) error
	ListBackups(ctx context.Context, userID int, gameID string) ([]*SaveBackup, error)
	RestoreBackup(ctx context.Context, userID int, gameID string, backupID string) error
	DeleteBackup(ctx context.Context, backupID string) error

	// Cleanup operations
	CleanupOldBackups(ctx context.Context, maxAge time.Duration) error
}

// SaveBackup represents a backup of a save file
type SaveBackup struct {
	ID         string            `json:"id"`
	UserID     int               `json:"user_id"`
	GameID     string            `json:"game_id"`
	SavePath   string            `json:"save_path"`
	BackupPath string            `json:"backup_path"`
	CreatedAt  time.Time         `json:"created_at"`
	FileSize   int64             `json:"file_size"`
	Metadata   map[string]string `json:"metadata"`
}

// SaveMetadata represents metadata about a save file
type SaveMetadata struct {
	GameVersion  string            `json:"game_version"`
	Character    string            `json:"character,omitempty"`
	Level        int               `json:"level,omitempty"`
	Score        int               `json:"score,omitempty"`
	PlayTime     time.Duration     `json:"play_time,omitempty"`
	Location     string            `json:"location,omitempty"`
	CustomFields map[string]string `json:"custom_fields,omitempty"`
}

// SaveService provides high-level save operations
type SaveService interface {
	// Basic save operations
	GetUserSave(ctx context.Context, userID int, username, gameID string) (*UserSave, error)
	SaveGame(ctx context.Context, userID int, username, gameID string, data []byte, metadata *SaveMetadata) error
	DeleteSave(ctx context.Context, userID int, username, gameID string) error

	// Environment setup
	PrepareGameEnvironment(ctx context.Context, userID int, username, gameID string) (map[string]string, error)

	// Backup operations
	BackupSave(ctx context.Context, userID int, username, gameID string) (*SaveBackup, error)
	RestoreSave(ctx context.Context, userID int, username, gameID string, backupID string) error

	// Maintenance
	CleanupOldSaves(ctx context.Context, userID int, maxAge time.Duration) error
}
