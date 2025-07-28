package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CleanupGame cleans up game-specific directories and files
func (gm *GameDirectoryManager) CleanupGame(gameID string, options *GameCleanupOptions) error {
	gm.mutex.Lock()
	defer gm.mutex.Unlock()

	// Remove temporary directories
	if tempDir, exists := gm.TempDirs[gameID]; exists {
		if options.ClearTempFiles {
			if err := os.RemoveAll(tempDir); err != nil {
				return fmt.Errorf("failed to remove temp dir: %w", err)
			}
			gm.logger.Printf("Removed temporary directory: %s", tempDir)
		}
		delete(gm.TempDirs, gameID)
	}

	// Clear lock files
	if options.RemoveLockFiles {
		if err := gm.clearLockFiles(gameID); err != nil {
			return fmt.Errorf("failed to clear lock files: %w", err)
		}
	}

	// Backup saves if requested
	if options.BackupSaves {
		if err := gm.backupSaves(gameID); err != nil {
			return fmt.Errorf("failed to backup saves: %w", err)
		}
	}

	// Cleanup save symlinks
	if options.CleanupSaveLinks {
		if err := gm.cleanupSaveSymlinks(gameID); err != nil {
			return fmt.Errorf("failed to cleanup save symlinks: %w", err)
		}
	}

	// Validate cleanup completion
	if options.ValidateCleanup {
		if err := gm.validateCleanupCompletion(gameID); err != nil {
			return fmt.Errorf("cleanup validation failed: %w", err)
		}
	}

	return nil
}

// clearLockFiles removes lock files associated with a game
func (gm *GameDirectoryManager) clearLockFiles(gameID string) error {
	lockFiles, err := gm.findLockFiles(gameID)
	if err != nil {
		return fmt.Errorf("failed to find lock files: %w", err)
	}

	for _, lockFile := range lockFiles {
		if err := os.Remove(lockFile); err != nil {
			gm.logger.Printf("Failed to remove lock file %s: %v", lockFile, err)
			continue
		}
		gm.logger.Printf("Removed lock file: %s", lockFile)
	}

	return nil
}

// findLockFiles finds lock files associated with a game
func (gm *GameDirectoryManager) findLockFiles(gameID string) ([]string, error) {
	var lockFiles []string

	// Check common lock file locations
	lockDirs := []string{
		"/tmp",
		"/var/tmp",
		"/usr/games/lib/nethackdir",
		filepath.Join(gm.BaseDir, "locks"),
	}

	for _, dir := range lockDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // Skip directories that can't be read
		}

		for _, entry := range entries {
			if strings.Contains(entry.Name(), gameID) && strings.Contains(entry.Name(), "lock") {
				lockFiles = append(lockFiles, filepath.Join(dir, entry.Name()))
			}
		}
	}

	return lockFiles, nil
}

// backupSaves creates backups of save files
func (gm *GameDirectoryManager) backupSaves(gameID string) error {
	// Find save files for this game
	saveFiles, err := gm.findSaveFiles(gameID)
	if err != nil {
		return fmt.Errorf("failed to find save files: %w", err)
	}

	if len(saveFiles) == 0 {
		gm.logger.Printf("No save files found for game %s", gameID)
		return nil
	}

	// Create backup directory
	backupDir := filepath.Join(gm.BaseDir, "backups", gameID, time.Now().Format("20060102_150405"))
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Copy save files to backup
	for _, saveFile := range saveFiles {
		backupFile := filepath.Join(backupDir, filepath.Base(saveFile))
		if err := gm.copyFile(saveFile, backupFile); err != nil {
			gm.logger.Printf("Failed to backup save file %s: %v", saveFile, err)
			continue
		}
		gm.logger.Printf("Backed up save file: %s -> %s", saveFile, backupFile)
	}

	return nil
}

// findSaveFiles finds save files associated with a game
func (gm *GameDirectoryManager) findSaveFiles(gameID string) ([]string, error) {
	var saveFiles []string

	// Check all user save directories
	gm.mutex.RLock()
	defer gm.mutex.RUnlock()

	for _, userDirs := range gm.UserDirs {
		entries, err := os.ReadDir(userDirs.SaveDir)
		if err != nil {
			continue // Skip directories that can't be read
		}

		for _, entry := range entries {
			if strings.Contains(entry.Name(), gameID) || strings.HasSuffix(entry.Name(), ".save") {
				saveFiles = append(saveFiles, filepath.Join(userDirs.SaveDir, entry.Name()))
			}
		}
	}

	return saveFiles, nil
}

// copyFile copies a file from src to dst
func (gm *GameDirectoryManager) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	// Copy file content
	buffer := make([]byte, 32*1024) // 32KB buffer
	for {
		n, err := sourceFile.Read(buffer)
		if err != nil && err.Error() == "EOF" {
			break
		}
		if err != nil {
			return err
		}

		if _, err := destFile.Write(buffer[:n]); err != nil {
			return err
		}
	}

	// Set file permissions
	return destFile.Chmod(sourceInfo.Mode())
}

// cleanupSaveSymlinks removes save symlinks for a game
func (gm *GameDirectoryManager) cleanupSaveSymlinks(gameID string) error {
	// Find and remove symlinks created for this game
	systemPaths, err := gm.pathDetector.DetectNetHackPaths()
	if err != nil {
		return fmt.Errorf("failed to detect system paths: %w", err)
	}

	if systemPaths.SysConfFile != "" {
		systemSaveDir := filepath.Join(filepath.Dir(systemPaths.SysConfFile), "save")

		// List all symlinks in the system save directory
		entries, err := os.ReadDir(systemSaveDir)
		if err != nil {
			return fmt.Errorf("failed to read system save directory: %w", err)
		}

		// Remove symlinks that match the game pattern
		for _, entry := range entries {
			if entry.Type()&os.ModeSymlink != 0 {
				linkPath := filepath.Join(systemSaveDir, entry.Name())
				if strings.Contains(entry.Name(), gameID) {
					if err := os.Remove(linkPath); err != nil {
						return fmt.Errorf("failed to remove symlink %s: %w", linkPath, err)
					}
					gm.logger.Printf("Removed symlink: %s", linkPath)
				}
			}
		}
	}

	return nil
}

// validateCleanupCompletion validates that cleanup was successful
func (gm *GameDirectoryManager) validateCleanupCompletion(gameID string) error {
	// Verify that all temporary files and directories are removed
	if _, exists := gm.TempDirs[gameID]; exists {
		return fmt.Errorf("temporary directory for game %s still exists", gameID)
	}

	// Check for remaining lock files
	if lockFiles, err := gm.findLockFiles(gameID); err != nil {
		return fmt.Errorf("failed to check for lock files: %w", err)
	} else if len(lockFiles) > 0 {
		return fmt.Errorf("found %d remaining lock files for game %s", len(lockFiles), gameID)
	}

	// Verify symlinks are removed
	if symlinks, err := gm.findRemainingSymlinks(gameID); err != nil {
		return fmt.Errorf("failed to check for remaining symlinks: %w", err)
	} else if len(symlinks) > 0 {
		return fmt.Errorf("found %d remaining symlinks for game %s", len(symlinks), gameID)
	}

	gm.logger.Printf("Cleanup validation successful for game %s", gameID)
	return nil
}

// findRemainingSymlinks finds remaining symlinks associated with a game
func (gm *GameDirectoryManager) findRemainingSymlinks(gameID string) ([]string, error) {
	var symlinks []string

	systemPaths, err := gm.pathDetector.DetectNetHackPaths()
	if err != nil {
		return symlinks, err
	}

	if systemPaths.SysConfFile != "" {
		systemSaveDir := filepath.Join(filepath.Dir(systemPaths.SysConfFile), "save")
		entries, err := os.ReadDir(systemSaveDir)
		if err != nil {
			return symlinks, err
		}

		for _, entry := range entries {
			if entry.Type()&os.ModeSymlink != 0 && strings.Contains(entry.Name(), gameID) {
				symlinks = append(symlinks, filepath.Join(systemSaveDir, entry.Name()))
			}
		}
	}

	return symlinks, nil
}

// CleanupExpiredTempDirs removes expired temporary directories
func (gm *GameDirectoryManager) CleanupExpiredTempDirs(maxAge time.Duration) error {
	gm.mutex.Lock()
	defer gm.mutex.Unlock()

	now := time.Now()
	expiredDirs := make([]string, 0)

	for gameID, tempDir := range gm.TempDirs {
		info, err := os.Stat(tempDir)
		if err != nil {
			if os.IsNotExist(err) {
				// Directory already removed, clean from map
				expiredDirs = append(expiredDirs, gameID)
			}
			continue
		}

		if now.Sub(info.ModTime()) > maxAge {
			// Remove expired directory
			if err := os.RemoveAll(tempDir); err != nil {
				gm.logger.Printf("Failed to remove expired temp dir %s: %v", tempDir, err)
				continue
			}
			gm.logger.Printf("Removed expired temp directory: %s", tempDir)
			expiredDirs = append(expiredDirs, gameID)
		}
	}

	// Remove expired entries from map
	for _, gameID := range expiredDirs {
		delete(gm.TempDirs, gameID)
	}

	return nil
}
