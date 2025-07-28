package config

import (
	"log"
	"os"
	"path/filepath"
	"testing"
)

func TestGameDirectoryManager(t *testing.T) {
	// Create a temporary base directory for testing
	tempDir := t.TempDir()
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	// Create manager
	manager := NewGameDirectoryManager(tempDir, logger)

	// Test GetUserDirs
	userID := 123
	userDirs := manager.GetUserDirs(userID)

	if userDirs.UserID != userID {
		t.Errorf("Expected UserID %d, got %d", userID, userDirs.UserID)
	}

	expectedBaseDir := filepath.Join(tempDir, "users", "123")
	if userDirs.BaseDir != expectedBaseDir {
		t.Errorf("Expected BaseDir %s, got %s", expectedBaseDir, userDirs.BaseDir)
	}

	// Test SetupGamePaths
	gameID := "test-game-123"
	options := &GameSetupOptions{
		CreateUserDirs:    true,
		CopyDefaultConfig: true,
		ValidatePaths:     false, // Skip validation for test
		SetPermissions:    true,
		DetectSystemPaths: false, // Skip system path detection for test
		CreateSaveLinks:   false, // Skip symlinks for test
	}

	paths, err := manager.SetupGamePaths(userID, gameID, options)
	if err != nil {
		t.Fatalf("SetupGamePaths failed: %v", err)
	}

	// Verify paths are set
	if paths.SaveDir == "" {
		t.Error("SaveDir should not be empty")
	}

	if paths.ConfigDir == "" {
		t.Error("ConfigDir should not be empty")
	}

	// Verify directories were created
	if _, err := os.Stat(userDirs.SaveDir); os.IsNotExist(err) {
		t.Error("SaveDir was not created")
	}

	if _, err := os.Stat(userDirs.ConfigDir); os.IsNotExist(err) {
		t.Error("ConfigDir was not created")
	}

	// Verify config file was created
	configFile := filepath.Join(userDirs.ConfigDir, ".nethackrc")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("Default config file was not created")
	}

	// Test cleanup
	cleanupOptions := &GameCleanupOptions{
		ClearTempFiles:   true,
		RemoveLockFiles:  false, // Skip for test
		BackupSaves:      false, // Skip for test
		CleanupSaveLinks: false, // Skip for test
		ValidateCleanup:  true,
	}

	err = manager.CleanupGame(gameID, cleanupOptions)
	if err != nil {
		t.Errorf("CleanupGame failed: %v", err)
	}
}

func TestPathDetector(t *testing.T) {
	detector := NewPathDetector()

	// Test parsePathValue
	testCases := []struct {
		input    string
		expected string
	}{
		{`[hackdir]="not set"`, ""},
		{`[savedir]="/home/user/saves"`, "/home/user/saves"},
		{`[configdir]="/etc/nethack"`, "/etc/nethack"},
		{`invalid format`, ""},
	}

	for _, tc := range testCases {
		result := detector.parsePathValue(tc.input)
		if result != tc.expected {
			t.Errorf("parsePathValue(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestGameConfigValidator(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	validator := NewGameConfigValidator(logger)

	// Create temporary directories for testing
	tempDir := t.TempDir()
	saveDir := filepath.Join(tempDir, "saves")
	configDir := filepath.Join(tempDir, "config")

	// Create directories
	os.MkdirAll(saveDir, 0755)
	os.MkdirAll(configDir, 0755)

	// Test valid configuration
	validPaths := &NetHackPaths{
		SaveDir:   saveDir,
		ConfigDir: configDir,
	}

	err := validator.ValidateConfig(validPaths)
	if err != nil {
		t.Errorf("ValidateConfig failed for valid paths: %v", err)
	}

	// Test missing required path
	invalidPaths := &NetHackPaths{
		SaveDir:   "", // Missing required path
		ConfigDir: configDir,
	}

	err = validator.ValidateConfig(invalidPaths)
	if err == nil {
		t.Error("ValidateConfig should have failed for missing required path")
	}

	// Check if it's a GameConfigError
	if configErr, ok := err.(*GameConfigError); ok {
		if configErr.ErrorType != "missing_required_path" {
			t.Errorf("Expected error type 'missing_required_path', got %s", configErr.ErrorType)
		}
	} else {
		t.Error("Expected GameConfigError")
	}
}

func TestGameConfigError(t *testing.T) {
	err := &GameConfigError{
		ErrorType:   "test_error",
		Message:     "This is a test error",
		Path:        "/test/path",
		Recoverable: true,
		Suggestions: []string{"Try this", "Or this"},
	}

	if err.Error() != "This is a test error" {
		t.Errorf("Expected error message 'This is a test error', got %s", err.Error())
	}
}
