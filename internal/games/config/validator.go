package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
)

// GameConfigValidator validates game configuration
type GameConfigValidator struct {
	requiredPaths    []string
	optionalPaths    []string
	pathValidators   map[string]func(string) error
	systemValidators []func(*NetHackPaths) error
	logger           *log.Logger
}

// NewGameConfigValidator creates a new GameConfigValidator
func NewGameConfigValidator(logger *log.Logger) *GameConfigValidator {
	validator := &GameConfigValidator{
		requiredPaths:  []string{"SaveDir", "ConfigDir"},
		optionalPaths:  []string{"BonesDir", "LevelDir", "LockDir", "TroubleDir"},
		pathValidators: make(map[string]func(string) error),
		logger:         logger,
	}

	// Setup path validators
	validator.pathValidators["SaveDir"] = validator.validateWritableDirectory
	validator.pathValidators["ConfigDir"] = validator.validateWritableDirectory
	validator.pathValidators["BonesDir"] = validator.validateWritableDirectory
	validator.pathValidators["LevelDir"] = validator.validateWritableDirectory
	validator.pathValidators["LockDir"] = validator.validateWritableDirectory
	validator.pathValidators["TroubleDir"] = validator.validateWritableDirectory

	validator.pathValidators["SysConfDir"] = validator.validateReadableDirectory
	validator.pathValidators["ScoreDir"] = validator.validateReadableDirectory
	validator.pathValidators["SymbolsFile"] = validator.validateReadableFile
	validator.pathValidators["DataFile"] = validator.validateReadableFile

	// Setup system validators
	validator.systemValidators = []func(*NetHackPaths) error{
		validator.validateSystemPaths,
		validator.validateUserPaths,
		validator.validatePathPermissions,
		validator.validateDiskSpace,
	}

	return validator
}

// ValidateConfig validates a NetHack configuration
func (gcv *GameConfigValidator) ValidateConfig(paths *NetHackPaths) error {
	// Validate individual paths
	pathValue := reflect.ValueOf(paths).Elem()
	pathType := pathValue.Type()

	for i := 0; i < pathValue.NumField(); i++ {
		field := pathType.Field(i)
		value := pathValue.Field(i).String()

		if value == "" {
			// Check if this is a required path
			if gcv.isRequiredPath(field.Name) {
				return &GameConfigError{
					ErrorType:   "missing_required_path",
					Message:     fmt.Sprintf("required path %s is empty", field.Name),
					Path:        field.Name,
					Recoverable: true,
					Suggestions: []string{
						fmt.Sprintf("Set %s to a valid directory path", field.Name),
						"Use auto-detection to populate system paths",
					},
				}
			}
			continue
		}

		// Run specific validator for this path
		if validator, exists := gcv.pathValidators[field.Name]; exists {
			if err := validator(value); err != nil {
				return &GameConfigError{
					ErrorType:   "path_validation_failed",
					Message:     fmt.Sprintf("validation failed for %s: %v", field.Name, err),
					Path:        value,
					Recoverable: true,
					Suggestions: []string{
						"Check if the path exists and is accessible",
						"Verify file/directory permissions",
						"Create the directory if it doesn't exist",
					},
				}
			}
		}
	}

	// Run system validators
	for _, validator := range gcv.systemValidators {
		if err := validator(paths); err != nil {
			return err
		}
	}

	return nil
}

// isRequiredPath checks if a path is required
func (gcv *GameConfigValidator) isRequiredPath(pathName string) bool {
	for _, required := range gcv.requiredPaths {
		if required == pathName {
			return true
		}
	}
	return false
}

// validateWritableDirectory validates that a directory is writable
func (gcv *GameConfigValidator) validateWritableDirectory(path string) error {
	// Check if directory exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Try to create the directory
			if err := os.MkdirAll(path, 0755); err != nil {
				return fmt.Errorf("directory does not exist and cannot be created: %w", err)
			}
		} else {
			return fmt.Errorf("cannot access directory: %w", err)
		}
	} else if !info.IsDir() {
		return fmt.Errorf("path exists but is not a directory")
	}

	// Check write permissions
	testFile := filepath.Join(path, ".write_test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("directory is not writable: %w", err)
	}

	// Clean up test file
	os.Remove(testFile)

	return nil
}

// validateReadableDirectory validates that a directory is readable
func (gcv *GameConfigValidator) validateReadableDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot access directory: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path exists but is not a directory")
	}

	// Check read permissions
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("directory is not readable: %w", err)
	}

	_ = entries // Suppress unused variable warning

	return nil
}

// validateReadableFile validates that a file is readable
func (gcv *GameConfigValidator) validateReadableFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot access file: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("path exists but is a directory, not a file")
	}

	// Check read permissions
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("file is not readable: %w", err)
	}
	file.Close()

	return nil
}

// validateSystemPaths validates system paths
func (gcv *GameConfigValidator) validateSystemPaths(paths *NetHackPaths) error {
	systemPaths := []string{paths.SysConfDir, paths.ScoreDir, paths.SymbolsFile, paths.DataFile}

	for _, path := range systemPaths {
		if path == "" {
			continue
		}

		if _, err := os.Stat(path); err != nil {
			gcv.logger.Printf("Warning: System path %s is not accessible: %v", path, err)
			// Don't fail on system paths, just log warnings
		}
	}

	return nil
}

// validateUserPaths validates user paths don't conflict with system paths
func (gcv *GameConfigValidator) validateUserPaths(paths *NetHackPaths) error {
	userPaths := []string{paths.SaveDir, paths.ConfigDir, paths.BonesDir, paths.LevelDir}

	for _, path := range userPaths {
		if path == "" {
			continue
		}

		// Check that user paths are not in system directories
		if gcv.isSystemPath(path) {
			return &GameConfigError{
				ErrorType:   "system_path_conflict",
				Message:     fmt.Sprintf("user path %s conflicts with system directories", path),
				Path:        path,
				Recoverable: true,
				Suggestions: []string{
					"Use a path outside of system directories",
					"Choose a path in user home directory or /tmp",
				},
			}
		}
	}

	return nil
}

// isSystemPath checks if a path is in a system directory
func (gcv *GameConfigValidator) isSystemPath(path string) bool {
	systemPrefixes := []string{
		"/usr/",
		"/opt/",
		"/bin/",
		"/sbin/",
		"/lib/",
		"/lib64/",
		"/etc/",
		"/var/lib/",
		"/var/games/",
	}

	for _, prefix := range systemPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}

// validatePathPermissions validates path permissions
func (gcv *GameConfigValidator) validatePathPermissions(paths *NetHackPaths) error {
	// Check that user has appropriate permissions for all paths
	userPaths := map[string]string{
		"SaveDir":    paths.SaveDir,
		"ConfigDir":  paths.ConfigDir,
		"BonesDir":   paths.BonesDir,
		"LevelDir":   paths.LevelDir,
		"LockDir":    paths.LockDir,
		"TroubleDir": paths.TroubleDir,
	}

	for pathName, path := range userPaths {
		if path == "" {
			continue
		}

		// Check ownership and permissions
		info, err := os.Stat(path)
		if err != nil {
			continue // Skip non-existent paths
		}

		// Check if path is writable
		if info.Mode().Perm()&0200 == 0 {
			return &GameConfigError{
				ErrorType:   "permission_denied",
				Message:     fmt.Sprintf("path %s (%s) is not writable", pathName, path),
				Path:        path,
				Recoverable: true,
				Suggestions: []string{
					fmt.Sprintf("chmod 755 %s", path),
					"Check directory ownership",
				},
			}
		}
	}

	return nil
}

// validateDiskSpace validates available disk space
func (gcv *GameConfigValidator) validateDiskSpace(paths *NetHackPaths) error {
	// Check available disk space for user directories
	userPaths := []string{paths.SaveDir, paths.ConfigDir, paths.BonesDir, paths.LevelDir}

	for _, path := range userPaths {
		if path == "" {
			continue
		}

		// Get disk usage
		var stat syscall.Statfs_t
		if err := syscall.Statfs(path, &stat); err != nil {
			gcv.logger.Printf("Warning: Cannot check disk space for %s: %v", path, err)
			continue
		}

		// Calculate available space in bytes
		availableBytes := stat.Bavail * uint64(stat.Bsize)

		// Require at least 100MB available space
		requiredBytes := uint64(100 * 1024 * 1024)

		if availableBytes < requiredBytes {
			return &GameConfigError{
				ErrorType:   "insufficient_space",
				Message:     fmt.Sprintf("insufficient disk space for %s: %d bytes available, %d bytes required", path, availableBytes, requiredBytes),
				Path:        path,
				Recoverable: true,
				Suggestions: []string{
					"Free up disk space",
					"Use a different directory with more space",
					"Clean up old save files and temporary directories",
				},
			}
		}
	}

	return nil
}
