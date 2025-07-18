package banner

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBannerManager_RenderMainAnon(t *testing.T) {
	// Create a temporary banner file
	tempFile, err := os.CreateTemp("", "test_banner_*.txt")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	bannerContent := `Welcome to $SERVERID!

Connected as: Anonymous
Date: $DATE | Time: $TIME

Menu Options:
  [l] Login
  [q] Quit

Choice: `

	_, err = tempFile.WriteString(bannerContent)
	require.NoError(t, err)
	tempFile.Close()

	// Create banner manager with temp file
	config := &BannerConfig{
		MainAnon:  tempFile.Name(),
		MainUser:  "",
		WatchMenu: "",
	}
	manager := NewBannerManager(config, "test-version")

	// Test rendering
	result, err := manager.RenderMainAnon()
	assert.NoError(t, err)
	assert.Contains(t, result, "Welcome to DungeonGate!")
	assert.Contains(t, result, "Connected as: Anonymous")
	assert.Contains(t, result, time.Now().Format("2006-01-02"))
	assert.Contains(t, result, "[l] Login")
	assert.Contains(t, result, "[q] Quit")
	// Check that line endings are converted to \r\n
	assert.Contains(t, result, "\r\n")
}

func TestBannerManager_RenderMainUser(t *testing.T) {
	// Create a temporary banner file
	tempFile, err := os.CreateTemp("", "test_user_banner_*.txt")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	bannerContent := `Welcome back, $USERNAME!

Connected to $SERVERID
Date: $DATE | Time: $TIME

Menu Options:
  [p] Play
  [q] Quit

Choice: `

	_, err = tempFile.WriteString(bannerContent)
	require.NoError(t, err)
	tempFile.Close()

	// Create banner manager with temp file
	config := &BannerConfig{
		MainAnon:  "",
		MainUser:  tempFile.Name(),
		WatchMenu: "",
	}
	manager := NewBannerManager(config, "test-version")

	// Test rendering
	result, err := manager.RenderMainUser("testuser")
	assert.NoError(t, err)
	assert.Contains(t, result, "Welcome back, testuser!")
	assert.Contains(t, result, "Connected to DungeonGate")
	assert.Contains(t, result, time.Now().Format("2006-01-02"))
	assert.Contains(t, result, "[p] Play")
	assert.Contains(t, result, "[q] Quit")
	// Check that line endings are converted to \r\n
	assert.Contains(t, result, "\r\n")
}

func TestBannerManager_EmptyPath(t *testing.T) {
	// Create banner manager with empty path
	config := &BannerConfig{
		MainAnon:  "",
		MainUser:  "",
		WatchMenu: "",
	}
	manager := NewBannerManager(config, "test-version")

	// Test rendering with empty path
	result, err := manager.RenderMainAnon()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "main anonymous banner path is not configured")
	assert.Empty(t, result)
}

func TestBannerManager_FileNotFound(t *testing.T) {
	// Create banner manager with non-existent file
	config := &BannerConfig{
		MainAnon:  "/nonexistent/path/banner.txt",
		MainUser:  "",
		WatchMenu: "",
	}
	manager := NewBannerManager(config, "test-version")

	// Test rendering with non-existent file
	result, err := manager.RenderMainAnon()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "banner file not found")
	assert.Empty(t, result)
}

func TestBannerManager_VariableSubstitution(t *testing.T) {
	// Create a temporary banner file with all variables
	tempFile, err := os.CreateTemp("", "test_variables_*.txt")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	bannerContent := `Server: $SERVERID
User: $USERNAME
Date: $DATE
Time: $TIME`

	_, err = tempFile.WriteString(bannerContent)
	require.NoError(t, err)
	tempFile.Close()

	// Create banner manager
	config := &BannerConfig{
		MainAnon:  tempFile.Name(),
		MainUser:  tempFile.Name(),
		WatchMenu: "",
	}
	manager := NewBannerManager(config, "test-version")

	// Test anonymous banner (no USERNAME)
	result, err := manager.RenderMainAnon()
	assert.NoError(t, err)
	assert.Contains(t, result, "Server: DungeonGate")
	assert.Contains(t, result, "User: $USERNAME") // Should not be substituted
	assert.Contains(t, result, "Date: "+time.Now().Format("2006-01-02"))
	assert.Contains(t, result, "Time: "+time.Now().Format("15:04:05"))

	// Test user banner (with USERNAME)
	result, err = manager.RenderMainUser("alice")
	assert.NoError(t, err)
	assert.Contains(t, result, "Server: DungeonGate")
	assert.Contains(t, result, "User: alice")
	assert.Contains(t, result, "Date: "+time.Now().Format("2006-01-02"))
	assert.Contains(t, result, "Time: "+time.Now().Format("15:04:05"))
}

func TestBannerManager_RenderWatchMenu(t *testing.T) {
	// Create a temporary banner file
	tempFile, err := os.CreateTemp("", "test_watch_*.txt")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	bannerContent := `=== $SERVERID Spectator Menu ===

Date: $DATE | Time: $TIME

Active Games:
  [1] NetHack - alice
  [2] NetHack - bob

[q] Quit spectating

Choice: `

	_, err = tempFile.WriteString(bannerContent)
	require.NoError(t, err)
	tempFile.Close()

	// Create banner manager with temp file
	config := &BannerConfig{
		MainAnon:  "",
		MainUser:  "",
		WatchMenu: tempFile.Name(),
	}
	manager := NewBannerManager(config, "test-version")

	// Test rendering
	result, err := manager.RenderWatchMenu()
	assert.NoError(t, err)
	assert.Contains(t, result, "=== DungeonGate Spectator Menu ===")
	assert.Contains(t, result, time.Now().Format("2006-01-02"))
	assert.Contains(t, result, "Active Games:")
	assert.Contains(t, result, "[1] NetHack - alice")
	assert.Contains(t, result, "[q] Quit spectating")
	// Check that line endings are converted to \r\n
	assert.Contains(t, result, "\r\n")
}

func TestBannerManager_LineEndingConversion(t *testing.T) {
	// Create a temporary banner file with Unix line endings
	tempFile, err := os.CreateTemp("", "test_endings_*.txt")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	bannerContent := "Line 1\nLine 2\nLine 3\n"

	_, err = tempFile.WriteString(bannerContent)
	require.NoError(t, err)
	tempFile.Close()

	// Create banner manager
	config := &BannerConfig{
		MainAnon:  tempFile.Name(),
		MainUser:  "",
		WatchMenu: "",
	}
	manager := NewBannerManager(config, "test-version")

	// Test rendering
	result, err := manager.RenderMainAnon()
	assert.NoError(t, err)

	// Should convert \n to \r\n
	assert.Contains(t, result, "Line 1\r\n")
	assert.Contains(t, result, "Line 2\r\n")
	assert.Contains(t, result, "Line 3\r\n")
	assert.NotContains(t, result, "Line 1\n")
}
