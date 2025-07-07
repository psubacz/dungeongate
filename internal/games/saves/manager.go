package saves

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// UserSave represents a user's game save data
type UserSave struct {
	UserID    int       `json:"user_id"`
	Username  string    `json:"username"`
	GameID    string    `json:"game_id"`
	SaveData  []byte    `json:"save_data"`
	SavePath  string    `json:"save_path"`
	SaveHash  string    `json:"save_hash"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	FileSize  int64     `json:"file_size"`
	HasSave   bool      `json:"has_save"`
}

// SaveManager handles save file operations for single save per user
type SaveManager struct {
	baseSaveDir string
}

// NewSaveManager creates a new save manager
func NewSaveManager(baseSaveDir string) *SaveManager {
	return &SaveManager{
		baseSaveDir: baseSaveDir,
	}
}

// GetUserSaveDir returns the save directory for a user
func (sm *SaveManager) GetUserSaveDir(username string) string {
	return filepath.Join(sm.baseSaveDir, "users", username)
}

// GetUserSave retrieves a user's save data for a specific game
func (sm *SaveManager) GetUserSave(username, gameID string) (*UserSave, error) {
	userDir := sm.GetUserSaveDir(username)

	// For NetHack, check the standard save file location
	var savePath string
	var saveData []byte
	var fileSize int64
	var hasSave bool
	var modTime time.Time

	if gameID == "nethack" {
		// Check for NetHack save file patterns in multiple locations
		patterns := []string{
			// Our configured save directory
			filepath.Join(sm.baseSaveDir, "save", "*"+username+"*"), // e.g., 501caboose.Z
			filepath.Join(sm.baseSaveDir, "save", "*"+username),     // e.g., 501caboose
			filepath.Join(sm.baseSaveDir, "save", username+"*"),     // e.g., caboose.nh
			filepath.Join(sm.baseSaveDir, "save", "*"),              // Check all files in save dir
			// NetHack system save directory (homebrew)
			filepath.Join("/opt/homebrew/share/nethack/save", "*"+username+"*"),
			filepath.Join("/opt/homebrew/share/nethack/save", "*"),
			// User directories
			filepath.Join(userDir, username),
			filepath.Join(userDir, "save"),
		}

		for _, pattern := range patterns {
			matches, err := filepath.Glob(pattern)
			if err != nil {
				continue
			}

			for _, match := range matches {
				if fileInfo, err := os.Stat(match); err == nil && !fileInfo.IsDir() && fileInfo.Size() > 0 {
					// For the wildcard patterns, check if filename contains username
					filename := filepath.Base(match)
					if strings.HasSuffix(pattern, "*") {
						if !strings.Contains(strings.ToLower(filename), strings.ToLower(username)) {
							continue // Skip files that don't contain username
						}
					}

					savePath = match
					fileSize = fileInfo.Size()
					modTime = fileInfo.ModTime()
					hasSave = true

					// Read save data if file is small enough (< 1MB)
					if fileSize < 1024*1024 {
						if data, err := os.ReadFile(match); err == nil {
							saveData = data
						}
					}
					break
				}
			}
			if hasSave {
				break
			}
		}
	}

	// Calculate save hash if we have data
	var saveHash string
	if len(saveData) > 0 {
		hash := sha256.Sum256(saveData)
		saveHash = fmt.Sprintf("%x", hash)[:8] // First 8 characters for display
	} else if hasSave && savePath != "" {
		// If we didn't read the data but have a file, calculate hash from file
		if data, err := os.ReadFile(savePath); err == nil {
			hash := sha256.Sum256(data)
			saveHash = fmt.Sprintf("%x", hash)[:8]
		}
	}

	save := &UserSave{
		Username:  username,
		GameID:    gameID,
		SaveData:  saveData,
		SavePath:  savePath,
		SaveHash:  saveHash,
		UpdatedAt: modTime,
		FileSize:  fileSize,
		HasSave:   hasSave,
	}

	if !hasSave {
		save.CreatedAt = time.Now()
		save.UpdatedAt = time.Now()
	} else {
		save.CreatedAt = modTime
	}

	return save, nil
}

// DeleteUserSave removes a user's save data
func (sm *SaveManager) DeleteUserSave(username, gameID string) error {
	save, err := sm.GetUserSave(username, gameID)
	if err != nil {
		return err
	}

	if !save.HasSave {
		return nil // Nothing to delete
	}

	// Remove the save file
	if err := os.Remove(save.SavePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete save file: %w", err)
	}

	// Also remove any backup files
	backupPath := save.SavePath + ".bak"
	os.Remove(backupPath) // Ignore errors for backup removal

	return nil
}

// BackupUserSave creates a backup of a user's save
func (sm *SaveManager) BackupUserSave(username, gameID string) error {
	save, err := sm.GetUserSave(username, gameID)
	if err != nil || !save.HasSave {
		return err
	}

	// Create backup filename
	backupPath := save.SavePath + ".bak"

	// Copy file
	input, err := os.ReadFile(save.SavePath)
	if err != nil {
		return fmt.Errorf("failed to read save file: %w", err)
	}

	if err := os.WriteFile(backupPath, input, 0644); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	return nil
}

// PrepareUserSaveEnvironment sets up the save environment for a user's game
func (sm *SaveManager) PrepareUserSaveEnvironment(username, gameID string) (map[string]string, error) {
	userDir := sm.GetUserSaveDir(username)

	// Create user directory if it doesn't exist
	if err := os.MkdirAll(userDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create user save directory: %w", err)
	}

	env := map[string]string{
		"HACKDIR":    "/opt/homebrew/Cellar/nethack/3.6.7/libexec", // NetHack data directory
		"NETHACKDIR": "/opt/homebrew/Cellar/nethack/3.6.7/libexec", // NetHack data directory
		"HOME":       fmt.Sprintf("/tmp/%s", username),             // Use simple temp directory like older version
		"USER":       username,
		"LOGNAME":    username,
	}

	if gameID == "nethack" {
		// NetHack-specific environment setup
		env["NETHACKOPTIONS"] = "@" + filepath.Join(userDir, ".nethackrc")

		// Create a simple .nethackrc if it doesn't exist
		nethackrc := filepath.Join(userDir, ".nethackrc")
		if _, err := os.Stat(nethackrc); os.IsNotExist(err) {
			rcContent := `# NetHack configuration for ` + username + `
OPTIONS=color,autopickup,pickup_types:$
OPTIONS=!time,showexp,showscore
OPTIONS=hilite_pet,boulder:0
`
			_ = os.WriteFile(nethackrc, []byte(rcContent), 0644)
		}
	}

	return env, nil
}

// CleanupOldSaves removes old backup files and temporary saves
func (sm *SaveManager) CleanupOldSaves(username, gameID string, maxAge time.Duration) error {
	userDir := sm.GetUserSaveDir(username)

	return filepath.WalkDir(userDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if d.IsDir() {
			return nil
		}

		// Check if it's a backup file or temporary file
		if filepath.Ext(path) == ".bak" || filepath.Ext(path) == ".tmp" {
			if info, err := d.Info(); err == nil {
				if time.Since(info.ModTime()) > maxAge {
					os.Remove(path) // Ignore errors
				}
			}
		}

		return nil
	})
}
