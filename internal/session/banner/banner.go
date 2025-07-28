package banner

import (
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"
	"time"
)

// BannerManager handles loading and rendering banners with template variables
type BannerManager struct {
	config        *BannerConfig
	cache         *BannerCache
	serverStarted time.Time
	unicode       *UnicodeSupport
	version       string
	mu            sync.RWMutex
}

// BannerCache holds cached rendered banners
type BannerCache struct {
	banners map[string]*CachedBanner
	ttl     time.Duration
	maxSize int
	mu      sync.RWMutex
}

// CachedBanner represents a cached banner with metadata
type CachedBanner struct {
	content     string
	renderedAt  time.Time
	accessCount int
	variables   map[string]string
}

// TerminalDimensions represents terminal capabilities and size
type TerminalDimensions struct {
	Width  int
	Height int
	Colors bool
	UTF8   bool
}

// UnicodeSupport handles Unicode character mapping and fallbacks
type UnicodeSupport struct {
	UTF8Enabled  bool
	CharMappings map[string]string // Unicode -> ASCII fallback
}

// NewUnicodeSupport creates a Unicode support handler with fallback mappings
func NewUnicodeSupport(utf8Enabled bool) *UnicodeSupport {
	charMappings := map[string]string{
		// Box drawing characters -> ASCII equivalents
		"═": "=",
		"─": "-",
		"│": "|",
		"┌": "+",
		"┐": "+",
		"└": "+",
		"┘": "+",
		"├": "+",
		"┤": "+",
		"┬": "+",
		"┴": "+",
		"┼": "+",

		// Special characters -> ASCII equivalents
		"•": "*",
		"◦": "-",
		"▸": ">",
		"▪": "*",
		"→": "->",
		"←": "<-",
		"↑": "^",
		"↓": "v",

		// Typography -> ASCII equivalents
		"\u201c": "\"",  // Left double quotation mark
		"\u201d": "\"",  // Right double quotation mark
		"\u2018": "'",   // Left single quotation mark
		"\u2019": "'",   // Right single quotation mark
		"\u2026": "...", // Horizontal ellipsis
		"\u2013": "-",   // En dash
		"\u2014": "--",  // Em dash

		// Emoji fallbacks (minimal set)
		"\U0001f4e2": "[!]",    // 📢 Loudspeaker
		"\U0001f552": "[TIME]", // 🕒 Clock
		"\U0001f3ae": "[GAME]", // 🎮 Video game
		"\U0001f4a1": "[TIP]",  // 💡 Light bulb
	}

	return &UnicodeSupport{
		UTF8Enabled:  utf8Enabled,
		CharMappings: charMappings,
	}
}

// ConvertText converts Unicode characters to ASCII if UTF-8 is not enabled
func (us *UnicodeSupport) ConvertText(text string) string {
	if us.UTF8Enabled {
		return text
	}

	// Replace Unicode characters with ASCII equivalents
	result := text
	for unicode, ascii := range us.CharMappings {
		result = strings.ReplaceAll(result, unicode, ascii)
	}

	return result
}

// SetUTF8Support enables or disables UTF-8 support
func (bm *BannerManager) SetUTF8Support(enabled bool) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.unicode.UTF8Enabled = enabled
}

// DetectTerminalCapabilities detects terminal capabilities from environment
func DetectTerminalCapabilities() *TerminalDimensions {
	// Basic detection - in a real implementation this would probe the terminal
	return &TerminalDimensions{
		Width:  80,
		Height: 24,
		Colors: true,
		UTF8:   true, // Default to UTF-8 support
	}
}

// BannerConfig contains paths to banner files and header/footer configuration
type BannerConfig struct {
	MainAnon           string
	MainUser           string
	MainAdmin          string
	WatchMenu          string
	ServiceUnavailable string

	// Header and footer configuration
	Headers HeaderFooterConfig `yaml:"headers"`
	Footers HeaderFooterConfig `yaml:"footers"`
}

// HeaderFooterConfig defines configurable headers and footers for different menu types
type HeaderFooterConfig struct {
	Anonymous     string `yaml:"anonymous"`
	User          string `yaml:"user"`
	GameSelection string `yaml:"game_selection"`
	Global        string `yaml:"global"`
}

// NewBannerManager creates a new banner manager with caching and Unicode support
func NewBannerManager(config *BannerConfig, version string) *BannerManager {
	cache := &BannerCache{
		banners: make(map[string]*CachedBanner),
		ttl:     5 * time.Minute, // Cache for 5 minutes
		maxSize: 50,              // Maximum 50 cached banners
	}

	// Default to UTF-8 enabled, can be overridden later
	unicode := NewUnicodeSupport(true)

	return &BannerManager{
		config:        config,
		cache:         cache,
		serverStarted: time.Now(),
		unicode:       unicode,
		version:       version,
	}
}

// NewBannerCache creates a new banner cache
func NewBannerCache(ttl time.Duration, maxSize int) *BannerCache {
	return &BannerCache{
		banners: make(map[string]*CachedBanner),
		ttl:     ttl,
		maxSize: maxSize,
	}
}

// RenderMainAnon renders the main anonymous user banner with header and footer
func (bm *BannerManager) RenderMainAnon() (string, error) {
	if bm.config.MainAnon == "" {
		return "", fmt.Errorf("main anonymous banner path is not configured")
	}

	variables := bm.GetTemplateVariables("")

	// Get header
	header := bm.RenderHeader("anonymous", variables)

	// Get main banner content
	banner, err := bm.renderBanner(bm.config.MainAnon, variables)
	if err != nil {
		return "", err
	}

	// Get footer
	footer := bm.RenderFooter("anonymous", variables)

	// Combine header + banner + footer
	result := header + banner + footer
	return result, nil
}

// RenderMainUser renders the main authenticated user banner with header and footer
func (bm *BannerManager) RenderMainUser(username string) (string, error) {
	if bm.config.MainUser == "" {
		return "", fmt.Errorf("main user banner path is not configured")
	}

	variables := bm.GetTemplateVariables(username)

	// Get header
	header := bm.RenderHeader("user", variables)

	// Get main banner content
	banner, err := bm.renderBanner(bm.config.MainUser, variables)
	if err != nil {
		return "", err
	}

	// Get footer
	footer := bm.RenderFooter("user", variables)

	// Combine header + banner + footer
	result := header + banner + footer
	return result, nil
}

// RenderMainAdmin renders the main admin menu banner for admin users
func (bm *BannerManager) RenderMainAdmin(username string) (string, error) {
	if bm.config.MainAdmin == "" {
		return "", fmt.Errorf("main admin banner path is not configured (MainAdmin field is empty, config: %+v)", bm.config)
	}

	variables := bm.GetTemplateVariables(username)

	// Get header
	header := bm.RenderHeader("admin", variables)

	// Get main banner content
	banner, err := bm.renderBanner(bm.config.MainAdmin, variables)
	if err != nil {
		return "", err
	}

	// Get footer
	footer := bm.RenderFooter("admin", variables)

	// Combine header + banner + footer
	result := header + banner + footer
	return result, nil
}

// RenderWatchMenu renders the watch menu banner
func (bm *BannerManager) RenderWatchMenu() (string, error) {
	return bm.renderBanner(bm.config.WatchMenu, map[string]string{
		"$SERVERID": "DungeonGate",
		"$DATE":     time.Now().Format("2006-01-02"),
		"$TIME":     time.Now().Format("15:04:05"),
	})
}

// RenderServiceUnavailable renders the service unavailable banner with countdown and service status
func (bm *BannerManager) RenderServiceUnavailable(username string, remainingMinutes, remainingSeconds int, serviceStatus string) (string, error) {
	var countdown string
	if remainingMinutes > 0 {
		countdown = fmt.Sprintf("%dm %ds", remainingMinutes, remainingSeconds)
	} else {
		countdown = fmt.Sprintf("%ds", remainingSeconds)
	}

	// Use fallback path if ServiceUnavailable is empty
	filePath := bm.config.ServiceUnavailable
	if filePath == "" {
		filePath = "./assets/banners/service_unavailable.txt"
	}

	return bm.renderBanner(filePath, map[string]string{
		"$SERVERID":       "DungeonGate",
		"$USERNAME":       username,
		"$DATE":           time.Now().Format("2006-01-02"),
		"$TIME":           time.Now().Format("15:04:05"),
		"$COUNTDOWN":      countdown,
		"$SERVICE_STATUS": serviceStatus,
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

	// Apply Unicode conversion if needed
	banner = bm.unicode.ConvertText(banner)

	// Convert line endings for SSH (use \r\n)
	banner = strings.ReplaceAll(banner, "\n", "\r\n")

	// Ensure it ends with a newline
	if !strings.HasSuffix(banner, "\r\n") {
		banner += "\r\n"
	}

	return banner, nil
}

// RenderHeader renders a header for the specified menu type
func (bm *BannerManager) RenderHeader(menuType string, variables map[string]string) string {
	var headerText string

	// Get menu-specific header first, fall back to global
	switch menuType {
	case "anonymous":
		headerText = bm.config.Headers.Anonymous
	case "user":
		headerText = bm.config.Headers.User
	case "game_selection":
		headerText = bm.config.Headers.GameSelection
	default:
		headerText = bm.config.Headers.Global
	}

	// If no specific header, use global
	if headerText == "" {
		headerText = bm.config.Headers.Global
	}

	// If still no header, return empty
	if headerText == "" {
		return ""
	}

	// Substitute template variables
	for variable, value := range variables {
		headerText = strings.ReplaceAll(headerText, variable, value)
	}

	// Apply Unicode conversion if needed
	headerText = bm.unicode.ConvertText(headerText)

	// Ensure proper line endings and spacing
	headerText = strings.ReplaceAll(headerText, "\n", "\r\n")
	if !strings.HasSuffix(headerText, "\r\n") {
		headerText += "\r\n"
	}

	return headerText
}

// RenderFooter renders a footer for the specified menu type
func (bm *BannerManager) RenderFooter(menuType string, variables map[string]string) string {
	var footerText string

	// Get menu-specific footer first, fall back to global
	switch menuType {
	case "anonymous":
		footerText = bm.config.Footers.Anonymous
	case "user":
		footerText = bm.config.Footers.User
	case "game_selection":
		footerText = bm.config.Footers.GameSelection
	default:
		footerText = bm.config.Footers.Global
	}

	// If no specific footer, use global
	if footerText == "" {
		footerText = bm.config.Footers.Global
	}

	// If still no footer, return empty
	if footerText == "" {
		return ""
	}

	// Substitute template variables
	for variable, value := range variables {
		footerText = strings.ReplaceAll(footerText, variable, value)
	}

	// Apply Unicode conversion if needed
	footerText = bm.unicode.ConvertText(footerText)

	// Ensure proper line endings and spacing
	footerText = strings.ReplaceAll(footerText, "\n", "\r\n")
	if !strings.HasPrefix(footerText, "\r\n") {
		footerText = "\r\n" + footerText
	}

	return footerText
}

// GetTemplateVariables returns common template variables for banner rendering
func (bm *BannerManager) GetTemplateVariables(username string) map[string]string {
	now := time.Now()
	uptime := now.Sub(bm.serverStarted)

	variables := map[string]string{
		"$SERVERID": "DungeonGate",
		"$DATE":     now.Format("2006-01-02"),
		"$TIME":     now.Format("15:04:05"),
		"$UPTIME":   formatUptime(uptime),
		"$VERSION":  bm.version,
		"$TIMEZONE": now.Format("MST"),
	}

	// Only include USERNAME variable if username is provided
	if username != "" {
		variables["$USERNAME"] = username
	}

	return variables
}

// formatUptime formats duration as human-readable uptime
func formatUptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else {
		return fmt.Sprintf("%dm", minutes)
	}
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
		{"service_unavailable", bm.config.ServiceUnavailable},
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
		"main_anon":           bm.config.MainAnon,
		"main_user":           bm.config.MainUser,
		"watch_menu":          bm.config.WatchMenu,
		"service_unavailable": bm.config.ServiceUnavailable,
	}

	for name, path := range files {
		if stat, err := os.Stat(path); err == nil {
			info[name] = stat
		}
	}

	return info
}
