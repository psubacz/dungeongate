package banner

import (
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"
)

// BannerManager handles loading and rendering banners with template variables
type BannerManager struct {
	config *BannerConfig
}

// BannerConfig contains paths to banner files
type BannerConfig struct {
	MainAnon  string
	MainUser  string
	WatchMenu string
}

// NewBannerManager creates a new banner manager
func NewBannerManager(config *BannerConfig) *BannerManager {
	return &BannerManager{
		config: config,
	}
}

// RenderMainAnon renders the main anonymous user banner
func (bm *BannerManager) RenderMainAnon() (string, error) {
	// Debug: log the banner path being used
	if bm.config.MainAnon == "" {
		return "", fmt.Errorf("main anonymous banner path is not configured")
	}
	
	return bm.renderBanner(bm.config.MainAnon, map[string]string{
		"$SERVERID": "DungeonGate",
		"$DATE":     time.Now().Format("2006-01-02"),
		"$TIME":     time.Now().Format("15:04:05"),
	})
}

// RenderMainUser renders the main authenticated user banner
func (bm *BannerManager) RenderMainUser(username string) (string, error) {
	return bm.renderBanner(bm.config.MainUser, map[string]string{
		"$SERVERID": "DungeonGate",
		"$USERNAME": username,
		"$DATE":     time.Now().Format("2006-01-02"),
		"$TIME":     time.Now().Format("15:04:05"),
	})
}

// RenderWatchMenu renders the watch menu banner
func (bm *BannerManager) RenderWatchMenu() (string, error) {
	return bm.renderBanner(bm.config.WatchMenu, map[string]string{
		"$SERVERID": "DungeonGate",
		"$DATE":     time.Now().Format("2006-01-02"),
		"$TIME":     time.Now().Format("15:04:05"),
	})
}

// renderBanner loads a banner file and substitutes template variables
func (bm *BannerManager) renderBanner(filePath string, variables map[string]string) (string, error) {
	// Check if filePath is empty
	if filePath == "" {
		return "", fmt.Errorf("banner file path is empty")
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", fmt.Errorf("banner file not found: %s", filePath)
	}

	// Read the banner file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read banner file %s: %w", filePath, err)
	}

	// Convert to string and substitute variables
	banner := string(content)
	for variable, value := range variables {
		banner = strings.ReplaceAll(banner, variable, value)
	}

	// Convert line endings for SSH (use \r\n)
	banner = strings.ReplaceAll(banner, "\n", "\r\n")

	// Ensure it ends with a newline
	if !strings.HasSuffix(banner, "\r\n") {
		banner += "\r\n"
	}

	return banner, nil
}

// ValidateBannerFiles checks if all configured banner files exist
func (bm *BannerManager) ValidateBannerFiles() error {
	files := []struct {
		name string
		path string
	}{
		{"main_anon", bm.config.MainAnon},
		{"main_user", bm.config.MainUser},
		{"watch_menu", bm.config.WatchMenu},
	}

	for _, file := range files {
		if _, err := os.Stat(file.path); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("banner file %s not found: %s", file.name, file.path)
			}
			return fmt.Errorf("error accessing banner file %s (%s): %w", file.name, file.path, err)
		}
	}

	return nil
}

// GetBannerInfo returns information about the configured banners
func (bm *BannerManager) GetBannerInfo() map[string]fs.FileInfo {
	info := make(map[string]fs.FileInfo)

	files := map[string]string{
		"main_anon":  bm.config.MainAnon,
		"main_user":  bm.config.MainUser,
		"watch_menu": bm.config.WatchMenu,
	}

	for name, path := range files {
		if stat, err := os.Stat(path); err == nil {
			info[name] = stat
		}
	}

	return info
}