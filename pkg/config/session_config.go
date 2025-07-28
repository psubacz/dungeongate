package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// SessionServiceConfig represents session service configuration
type SessionServiceConfig struct {
	Version           string                   `yaml:"version"`
	Server            *ServerConfig            `yaml:"server"`
	SSH               *SSHConfig               `yaml:"ssh"`
	SessionManagement *SessionManagementConfig `yaml:"session_management"`
	Encryption        *EncryptionConfig        `yaml:"encryption"`
	Database          *DatabaseConfig          `yaml:"database"`
	Menu              *MenuConfig              `yaml:"menu"`
	Services          *ServicesConfig          `yaml:"services"`
	Storage           *StorageConfig           `yaml:"storage"`
	Logging           *LoggingConfig           `yaml:"logging"`
	Metrics           *MetricsConfig           `yaml:"metrics"`
	Health            *HealthConfig            `yaml:"health"`
	Security          *SecurityConfig          `yaml:"security"`
	Auth              *AuthServiceConfig       `yaml:"auth"`
	User              *UserConfig              `yaml:"user"`
	Games             []*GameConfig            `yaml:"games"`
}

// SSHConfig represents SSH server configuration
type SSHConfig struct {
	Enabled        bool                `yaml:"enabled"`
	Port           int                 `yaml:"port"`
	Host           string              `yaml:"host"`
	HostKeyPath    string              `yaml:"host_key_path"`
	Banner         string              `yaml:"banner"`
	MaxSessions    int                 `yaml:"max_sessions"`
	SessionTimeout string              `yaml:"session_timeout"`
	IdleTimeout    string              `yaml:"idle_timeout"`
	Auth           *SSHAuthConfig      `yaml:"auth"`
	Terminal       *SSHTerminalConfig  `yaml:"terminal"`
	Keepalive      *SSHKeepaliveConfig `yaml:"keepalive"`
}

// SSHAuthConfig represents SSH authentication configuration
type SSHAuthConfig struct {
	PasswordAuth    bool   `yaml:"password_auth"`
	PublicKeyAuth   bool   `yaml:"public_key_auth"`
	AllowAnonymous  bool   `yaml:"allow_anonymous"`
	AllowedUsername string `yaml:"allowed_username"` // Only allow connections from this username
	SSHPassword     string `yaml:"ssh_password"`     // SSH password for the allowed username (SSH level auth)
}

// SSHTerminalConfig represents SSH terminal configuration
type SSHTerminalConfig struct {
	DefaultSize        string   `yaml:"default_size"`
	MaxSize            string   `yaml:"max_size"`
	SupportedTerminals []string `yaml:"supported_terminals"`
}

// SSHKeepaliveConfig represents SSH keepalive configuration
type SSHKeepaliveConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Interval string `yaml:"interval"`
	CountMax int    `yaml:"count_max"`
}

// MenuConfig represents menu configuration
type MenuConfig struct {
	Banners *BannersConfig `yaml:"banners"`
	Options *MenuOptions   `yaml:"options"`
}

// BannersConfig represents banner configuration
type BannersConfig struct {
	MainAnon           string `yaml:"main_anon"`
	MainUser           string `yaml:"main_user"`
	MainAdmin          string `yaml:"main_admin"`
	WatchMenu          string `yaml:"watch_menu"`
	ServiceUnavailable string `yaml:"service_unavailable"`
}

// MenuOptions represents menu options configuration
type MenuOptions struct {
	Anonymous     []*MenuOption `yaml:"anonymous"`
	Authenticated []*MenuOption `yaml:"authenticated"`
}

// MenuOption represents a menu option
type MenuOption struct {
	Key    string `yaml:"key"`
	Label  string `yaml:"label"`
	Action string `yaml:"action"`
}

// ServicesConfig represents services configuration
type ServicesConfig struct {
	AuthService string `yaml:"auth_service"`
	UserService string `yaml:"user_service"`
	GameService string `yaml:"game_service"`
}

// StorageConfig represents storage configuration
type StorageConfig struct {
	TTYRecPath string `yaml:"ttyrec_path"`
	TempPath   string `yaml:"temp_path"`
}

// MetricsConfig represents metrics configuration
type MetricsConfig struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
}

// HealthConfig represents health check configuration
type HealthConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

// SecurityConfig represents security configuration
type SecurityConfig struct {
	RateLimiting         *RateLimitingConfig    `yaml:"rate_limiting"`
	BruteForceProtection *BruteForceConfig      `yaml:"brute_force_protection"`
	SessionSecurity      *SessionSecurityConfig `yaml:"session_security"`
}

// RateLimitingConfig represents rate limiting configuration
type RateLimitingConfig struct {
	Enabled             bool   `yaml:"enabled"`
	MaxConnectionsPerIP int    `yaml:"max_connections_per_ip"`
	ConnectionWindow    string `yaml:"connection_window"`
}

// BruteForceConfig represents brute force protection configuration
type BruteForceConfig struct {
	Enabled           bool   `yaml:"enabled"`
	MaxFailedAttempts int    `yaml:"max_failed_attempts"`
	LockoutDuration   string `yaml:"lockout_duration"`
}

// SessionSecurityConfig represents session security configuration
type SessionSecurityConfig struct {
	RequireEncryption  bool `yaml:"require_encryption"`
	SessionTokenLength int  `yaml:"session_token_length"`
	SecureRandom       bool `yaml:"secure_random"`
}

// AuthServiceConfig represents authentication service configuration
type AuthServiceConfig struct {
	Enabled                bool   `yaml:"enabled"`
	ServiceAddress         string `yaml:"service_address"`
	GRPCAddress            string `yaml:"grpc_address"`
	JWTSecret              string `yaml:"jwt_secret"`
	JWTIssuer              string `yaml:"jwt_issuer"`
	AccessTokenExpiration  string `yaml:"access_token_expiration"`
	RefreshTokenExpiration string `yaml:"refresh_token_expiration"`
	MaxLoginAttempts       int    `yaml:"max_login_attempts"`
	LockoutDuration        string `yaml:"lockout_duration"`
	RequireTokenForAPI     bool   `yaml:"require_token_for_api"`
	RequireTokenForSSH     bool   `yaml:"require_token_for_ssh"`
}

// LoadSessionServiceConfig loads session service configuration
func LoadSessionServiceConfig(configPath string) (*SessionServiceConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables
	expanded := os.ExpandEnv(string(data))

	var config SessionServiceConfig
	if err := yaml.Unmarshal([]byte(expanded), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Apply defaults
	applyDefaults(&config)

	return &config, nil
}

// applyDefaults applies default values to configuration
func applyDefaults(cfg *SessionServiceConfig) {
	// Version default
	if cfg.Version == "" {
		cfg.Version = "0.0.2"
	}

	// Server defaults
	if cfg.Server == nil {
		cfg.Server = &ServerConfig{}
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8083
	}
	if cfg.Server.GRPCPort == 0 {
		cfg.Server.GRPCPort = 9093
	}
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Timeout == "" {
		cfg.Server.Timeout = "60s"
	}
	if cfg.Server.MaxConnections == 0 {
		cfg.Server.MaxConnections = 1000
	}

	// SSH defaults
	if cfg.SSH == nil {
		cfg.SSH = &SSHConfig{}
	}
	if cfg.SSH.Port == 0 {
		cfg.SSH.Port = 22
	}
	if cfg.SSH.Host == "" {
		cfg.SSH.Host = "0.0.0.0"
	}
	if cfg.SSH.HostKeyPath == "" {
		cfg.SSH.HostKeyPath = "/etc/ssh/ssh_host_rsa_key"
	}
	if cfg.SSH.Banner == "" {
		cfg.SSH.Banner = "Welcome to DungeonGate!\r\n"
	}
	if cfg.SSH.MaxSessions == 0 {
		cfg.SSH.MaxSessions = 100
	}
	if cfg.SSH.SessionTimeout == "" {
		cfg.SSH.SessionTimeout = "4h"
	}
	if cfg.SSH.IdleTimeout == "" {
		cfg.SSH.IdleTimeout = "30m"
	}
	if cfg.SSH.Auth == nil {
		cfg.SSH.Auth = &SSHAuthConfig{
			PasswordAuth:    true,
			PublicKeyAuth:   false,
			AllowAnonymous:  true,
			AllowedUsername: "dungeongate",
		}
	}
	if cfg.SSH.Keepalive == nil {
		cfg.SSH.Keepalive = &SSHKeepaliveConfig{
			Enabled:  true,
			Interval: "30s",
			CountMax: 3,
		}
	}
	if cfg.SSH.Terminal == nil {
		cfg.SSH.Terminal = &SSHTerminalConfig{
			DefaultSize:        "80x24",
			MaxSize:            "200x50",
			SupportedTerminals: []string{"xterm", "xterm-256color", "screen", "tmux", "vt100"},
		}
	}

	// Session management defaults
	if cfg.SessionManagement == nil {
		cfg.SessionManagement = &SessionManagementConfig{}
	}
	if cfg.SessionManagement.Terminal == nil {
		cfg.SessionManagement.Terminal = &TerminalConfig{
			DefaultSize: "80x24",
			MaxSize:     "200x50",
			Encoding:    "utf-8",
		}
	}
	if cfg.SessionManagement.Timeouts == nil {
		cfg.SessionManagement.Timeouts = &TimeoutsConfig{
			IdleTimeout:        "30m",
			MaxSessionDuration: "4h",
			CleanupInterval:    "5m",
		}
	}
	if cfg.SessionManagement.TTYRec == nil {
		cfg.SessionManagement.TTYRec = &TTYRecConfig{
			Enabled:       true,
			Compression:   "gzip",
			Directory:     "/dgldir/ttyrec",
			MaxFileSize:   "100MB",
			RetentionDays: 30,
		}
	}
	if cfg.SessionManagement.Monitoring == nil {
		cfg.SessionManagement.Monitoring = &MonitoringConfig{
			Enabled: true,
			Port:    8085,
		}
	}
	if cfg.SessionManagement.Spectating == nil {
		cfg.SessionManagement.Spectating = &SpectatingConfig{
			Enabled:                 true,
			MaxSpectatorsPerSession: 10,
			SpectatorTimeout:        "2h",
		}
	}
	if cfg.SessionManagement.Heartbeat == nil {
		cfg.SessionManagement.Heartbeat = &HeartbeatConfig{
			Enabled:                true,
			Interval:               "60s",
			IdleDetectionThreshold: "2m",
			IdleRetryInterval:      "5s",
			GRPCStream: &GRPCStreamConfig{
				Enabled:      true,
				PingInterval: "45s",
				PongTimeout:  "10s",
			},
		}
	}

	// Encryption defaults
	if cfg.Encryption == nil {
		cfg.Encryption = &EncryptionConfig{
			Enabled:             true,
			Algorithm:           "AES-256-GCM",
			KeyRotationInterval: "24h",
		}
	}

	// Menu defaults
	if cfg.Menu == nil {
		cfg.Menu = &MenuConfig{
			Banners: &BannersConfig{
				MainAnon:           "./assets/banners/main_anon.txt",
				MainUser:           "./assets/banners/main_user.txt",
				MainAdmin:          "./assets/banners/main_admin.txt",
				WatchMenu:          "./assets/banners/watch_menu.txt",
				ServiceUnavailable: "./assets/banners/service_unavailable.txt",
			},
			Options: &MenuOptions{
				Anonymous: []*MenuOption{
					{Key: "l", Label: "Login", Action: "login"},
					{Key: "r", Label: "Register", Action: "register"},
					{Key: "w", Label: "Watch games", Action: "watch"},
					{Key: "q", Label: "Quit", Action: "quit"},
				},
				Authenticated: []*MenuOption{
					{Key: "p", Label: "Play a game", Action: "play"},
					{Key: "w", Label: "Watch games", Action: "watch"},
					{Key: "e", Label: "Edit profile", Action: "edit_profile"},
					{Key: "r", Label: "View recordings", Action: "recordings"},
					{Key: "s", Label: "Statistics", Action: "stats"},
					{Key: "q", Label: "Quit", Action: "quit"},
				},
			},
		}
	}

	// Services defaults
	if cfg.Services == nil {
		cfg.Services = &ServicesConfig{
			AuthService: "auth-service:9090",
			UserService: "user-service:9091",
			GameService: "game-service:9092",
		}
	}

	// Storage defaults
	if cfg.Storage == nil {
		cfg.Storage = &StorageConfig{
			TTYRecPath: "/dgldir/ttyrec",
			TempPath:   "/tmp/sessions",
		}
	}

	// Logging defaults
	if cfg.Logging == nil {
		cfg.Logging = &LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		}
	}

	// Metrics defaults
	if cfg.Metrics == nil {
		cfg.Metrics = &MetricsConfig{
			Enabled: true,
			Port:    8085,
		}
	}

	// Health defaults
	if cfg.Health == nil {
		cfg.Health = &HealthConfig{
			Enabled: true,
			Path:    "/health",
		}
	}

	// Security defaults
	if cfg.Security == nil {
		cfg.Security = &SecurityConfig{
			RateLimiting: &RateLimitingConfig{
				Enabled:             true,
				MaxConnectionsPerIP: 10,
				ConnectionWindow:    "1m",
			},
			BruteForceProtection: &BruteForceConfig{
				Enabled:           true,
				MaxFailedAttempts: 5,
				LockoutDuration:   "15m",
			},
			SessionSecurity: &SessionSecurityConfig{
				RequireEncryption:  true,
				SessionTokenLength: 32,
				SecureRandom:       true,
			},
		}
	}
}

// Accessor methods for configuration

// GetSSH returns SSH configuration with defaults applied
func (c *SessionServiceConfig) GetSSH() *SSHConfig {
	if c.SSH == nil {
		return &SSHConfig{
			Enabled:        true,
			Port:           22,
			Host:           "0.0.0.0",
			HostKeyPath:    "/etc/ssh/ssh_host_rsa_key",
			Banner:         "Welcome to DungeonGate!\r\n",
			MaxSessions:    100,
			SessionTimeout: "4h",
			IdleTimeout:    "30m",
			Auth: &SSHAuthConfig{
				PasswordAuth:    true,
				PublicKeyAuth:   false,
				AllowAnonymous:  true,
				AllowedUsername: "dungeongate",
			},
			Terminal: &SSHTerminalConfig{
				DefaultSize:        "80x24",
				MaxSize:            "200x50",
				SupportedTerminals: []string{"xterm", "xterm-256color", "screen", "tmux", "vt100"},
			},
		}
	}
	return c.SSH
}

// GetMenu returns menu configuration with defaults applied
func (c *SessionServiceConfig) GetMenu() *MenuConfig {
	if c.Menu == nil {
		return &MenuConfig{
			Banners: &BannersConfig{
				MainAnon:           "./assets/banners/main_anon.txt",
				MainUser:           "./assets/banners/main_user.txt",
				MainAdmin:          "./assets/banners/main_admin.txt",
				WatchMenu:          "./assets/banners/watch_menu.txt",
				ServiceUnavailable: "./assets/banners/service_unavailable.txt",
			},
			Options: &MenuOptions{
				Anonymous: []*MenuOption{
					{Key: "l", Label: "Login", Action: "login"},
					{Key: "r", Label: "Register", Action: "register"},
					{Key: "w", Label: "Watch games", Action: "watch"},
					{Key: "q", Label: "Quit", Action: "quit"},
				},
				Authenticated: []*MenuOption{
					{Key: "p", Label: "Play a game", Action: "play"},
					{Key: "w", Label: "Watch games", Action: "watch"},
					{Key: "e", Label: "Edit profile", Action: "edit_profile"},
					{Key: "r", Label: "View recordings", Action: "recordings"},
					{Key: "s", Label: "Statistics", Action: "stats"},
					{Key: "q", Label: "Quit", Action: "quit"},
				},
			},
		}
	}
	return c.Menu
}

// GetServices returns services configuration with defaults applied
func (c *SessionServiceConfig) GetServices() *ServicesConfig {
	if c.Services == nil {
		return &ServicesConfig{
			AuthService: "auth-service:9090",
			UserService: "user-service:9091",
			GameService: "game-service:9092",
		}
	}
	return c.Services
}

// GetStorage returns storage configuration with defaults applied
func (c *SessionServiceConfig) GetStorage() *StorageConfig {
	if c.Storage == nil {
		return &StorageConfig{
			TTYRecPath: "/dgldir/ttyrec",
			TempPath:   "/tmp/sessions",
		}
	}
	return c.Storage
}

// Validation methods

// ValidateSSHConfig validates SSH configuration
func (c *SSHConfig) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid SSH port: %d", c.Port)
	}
	if c.Host == "" {
		return fmt.Errorf("SSH host cannot be empty")
	}
	if c.MaxSessions < 1 {
		return fmt.Errorf("max sessions must be at least 1")
	}
	if c.SessionTimeout == "" {
		return fmt.Errorf("session timeout cannot be empty")
	}
	if c.IdleTimeout == "" {
		return fmt.Errorf("idle timeout cannot be empty")
	}

	// Validate timeout durations
	if _, err := time.ParseDuration(c.SessionTimeout); err != nil {
		return fmt.Errorf("invalid session timeout format: %w", err)
	}
	if _, err := time.ParseDuration(c.IdleTimeout); err != nil {
		return fmt.Errorf("invalid idle timeout format: %w", err)
	}

	return nil
}

// ValidateSessionServiceConfig validates the entire session service configuration
func (c *SessionServiceConfig) Validate() error {
	if c.Server == nil {
		return fmt.Errorf("server configuration is required")
	}
	if c.SSH == nil {
		return fmt.Errorf("SSH configuration is required")
	}
	if c.SessionManagement == nil {
		return fmt.Errorf("session management configuration is required")
	}
	if c.Services == nil {
		return fmt.Errorf("services configuration is required")
	}

	// Validate SSH configuration
	if err := c.SSH.Validate(); err != nil {
		return fmt.Errorf("SSH configuration validation failed: %w", err)
	}

	// Validate ports don't conflict
	if c.Server.Port == c.SSH.Port {
		return fmt.Errorf("HTTP and SSH ports cannot be the same")
	}
	if c.Server.GRPCPort == c.SSH.Port {
		return fmt.Errorf("gRPC and SSH ports cannot be the same")
	}
	if c.Server.Port == c.Server.GRPCPort {
		return fmt.Errorf("HTTP and gRPC ports cannot be the same")
	}

	// Validate directories exist or can be created
	if c.SessionManagement.TTYRec.Enabled {
		if c.SessionManagement.TTYRec.Directory == "" {
			return fmt.Errorf("TTY recording directory is required when recording is enabled")
		}
		if err := os.MkdirAll(c.SessionManagement.TTYRec.Directory, 0755); err != nil {
			return fmt.Errorf("failed to create TTY recording directory: %w", err)
		}
	}

	if c.Storage.TTYRecPath != "" {
		if err := os.MkdirAll(c.Storage.TTYRecPath, 0755); err != nil {
			return fmt.Errorf("failed to create TTY recording path: %w", err)
		}
	}

	if c.Storage.TempPath != "" {
		if err := os.MkdirAll(c.Storage.TempPath, 0755); err != nil {
			return fmt.Errorf("failed to create temporary path: %w", err)
		}
	}

	return nil
}

// Helper functions

// GetSessionTimeoutDuration returns session timeout as duration
func (c *SSHConfig) GetSessionTimeoutDuration() time.Duration {
	if duration, err := time.ParseDuration(c.SessionTimeout); err == nil {
		return duration
	}
	return 4 * time.Hour // Default fallback
}

// GetIdleTimeoutDuration returns idle timeout as duration
func (c *SSHConfig) GetIdleTimeoutDuration() time.Duration {
	if duration, err := time.ParseDuration(c.IdleTimeout); err == nil {
		return duration
	}
	return 30 * time.Minute // Default fallback
}

// GetDefaultTerminalSize returns default terminal size as width and height
func (c *SSHTerminalConfig) GetDefaultTerminalSize() (width, height int) {
	width, height, err := ParseTerminalSize(c.DefaultSize)
	if err != nil {
		return 80, 24 // Safe fallback
	}
	return width, height
}

// GetMaxTerminalSize returns maximum terminal size as width and height
func (c *SSHTerminalConfig) GetMaxTerminalSize() (width, height int) {
	width, height, err := ParseTerminalSize(c.MaxSize)
	if err != nil {
		return 200, 50 // Safe fallback
	}
	return width, height
}

// IsTerminalSupported checks if a terminal type is supported
func (c *SSHTerminalConfig) IsTerminalSupported(termType string) bool {
	for _, supported := range c.SupportedTerminals {
		if supported == termType {
			return true
		}
	}
	return false
}

// GetDefaultDevelopmentConfig returns a development configuration
func GetDefaultDevelopmentConfig() *SessionServiceConfig {
	return &SessionServiceConfig{
		Server: &ServerConfig{
			Port:           8083,
			GRPCPort:       9093,
			Host:           "localhost",
			Timeout:        "30s",
			MaxConnections: 100,
		},
		SSH: &SSHConfig{
			Enabled:        true,
			Port:           2222, // Non-privileged port for development
			Host:           "localhost",
			HostKeyPath:    "./ssh_host_rsa_key",
			Banner:         "Welcome to DungeonGate Development Server!\r\n",
			MaxSessions:    10,
			SessionTimeout: "1h",
			IdleTimeout:    "15m",
			Auth: &SSHAuthConfig{
				PasswordAuth:    true,
				PublicKeyAuth:   false,
				AllowAnonymous:  true,
				AllowedUsername: "dungeongate",
			},
			Terminal: &SSHTerminalConfig{
				DefaultSize:        "80x24",
				MaxSize:            "120x40",
				SupportedTerminals: []string{"xterm", "xterm-256color"},
			},
			Keepalive: &SSHKeepaliveConfig{
				Enabled:  true,
				Interval: "30s",
				CountMax: 3,
			},
		},
		SessionManagement: &SessionManagementConfig{
			Terminal: &TerminalConfig{
				DefaultSize: "80x24",
				MaxSize:     "120x40",
				Encoding:    "utf-8",
			},
			Timeouts: &TimeoutsConfig{
				IdleTimeout:        "15m",
				MaxSessionDuration: "1h",
				CleanupInterval:    "1m",
			},
			TTYRec: &TTYRecConfig{
				Enabled:       true,
				Compression:   "gzip",
				Directory:     "./ttyrec",
				MaxFileSize:   "10MB",
				RetentionDays: 7,
			},
			Spectating: &SpectatingConfig{
				Enabled:                 true,
				MaxSpectatorsPerSession: 3,
				SpectatorTimeout:        "30m",
			},
			Heartbeat: &HeartbeatConfig{
				Enabled:                true,
				Interval:               "60s",
				IdleDetectionThreshold: "2m",
				IdleRetryInterval:      "5s",
				GRPCStream: &GRPCStreamConfig{
					Enabled:      true,
					PingInterval: "45s",
					PongTimeout:  "10s",
				},
			},
		},
		Services: &ServicesConfig{
			AuthService: "localhost:9090",
			UserService: "localhost:9091",
			GameService: "localhost:9092",
		},
		Storage: &StorageConfig{
			TTYRecPath: "./ttyrec",
			TempPath:   "./tmp",
		},
		Logging: &LoggingConfig{
			Level:  "debug",
			Format: "text",
			Output: "stdout",
		},
		Metrics: &MetricsConfig{
			Enabled: true,
			Port:    8085,
		},
		Health: &HealthConfig{
			Enabled: true,
			Path:    "/health",
		},
	}
}
