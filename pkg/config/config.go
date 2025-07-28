package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// ServerConfig represents common server configuration
type ServerConfig struct {
	Port           int    `yaml:"port"`
	GRPCPort       int    `yaml:"grpc_port"`
	Host           string `yaml:"host"`
	Timeout        string `yaml:"timeout"`
	MaxConnections int    `yaml:"max_connections"`
}

// LegacyDatabaseConfig represents basic database configuration (legacy compatibility)
// For full database features, use DatabaseConfig from user_config.go
type LegacyDatabaseConfig struct {
	Type       string                 `yaml:"type"`
	Connection map[string]interface{} `yaml:"connection"`
	Pool       *PoolConfig            `yaml:"pool,omitempty"`
}

// PoolConfig represents database pool configuration
type PoolConfig struct {
	MaxConnections        int    `yaml:"max_connections"`
	MaxIdleConnections    int    `yaml:"max_idle_connections"`
	ConnectionMaxLifetime string `yaml:"connection_max_lifetime"`
}

// EncryptionConfig represents encryption configuration
type EncryptionConfig struct {
	Enabled             bool   `yaml:"enabled"`
	Algorithm           string `yaml:"algorithm"`
	KeyRotationInterval string `yaml:"key_rotation_interval"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level    string          `yaml:"level"`
	Format   string          `yaml:"format"`
	Output   string          `yaml:"output"`
	File     *FileConfig     `yaml:"file,omitempty"`
	Journald *JournaldConfig `yaml:"journald,omitempty"`
}

// FileConfig represents file logging configuration
type FileConfig struct {
	Directory string `yaml:"directory"`
	Filename  string `yaml:"filename"`
	MaxSize   string `yaml:"max_size"`
	MaxFiles  int    `yaml:"max_files"`
	MaxAge    string `yaml:"max_age"`
	Compress  bool   `yaml:"compress"`
}

// JournaldConfig represents journald logging configuration
type JournaldConfig struct {
	Identifier string            `yaml:"identifier"`
	Fields     map[string]string `yaml:"fields"`
}

// MonitoringConfig represents monitoring configuration
type MonitoringConfig struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
}

// SessionManagementConfig represents session management configuration
type SessionManagementConfig struct {
	Terminal   *TerminalConfig   `yaml:"terminal"`
	Timeouts   *TimeoutsConfig   `yaml:"timeouts"`
	TTYRec     *TTYRecConfig     `yaml:"ttyrec"`
	Monitoring *MonitoringConfig `yaml:"monitoring"`
	Spectating *SpectatingConfig `yaml:"spectating"`
	Heartbeat  *HeartbeatConfig  `yaml:"heartbeat"`
}

// TerminalConfig represents terminal configuration
type TerminalConfig struct {
	DefaultSize string `yaml:"default_size"`
	MaxSize     string `yaml:"max_size"`
	Encoding    string `yaml:"encoding"`
}

// TimeoutsConfig represents timeout configuration
type TimeoutsConfig struct {
	IdleTimeout        string `yaml:"idle_timeout"`
	MaxSessionDuration string `yaml:"max_session_duration"`
	CleanupInterval    string `yaml:"cleanup_interval"`
}

// TTYRecConfig represents TTYRec configuration
type TTYRecConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Compression   string `yaml:"compression"`
	Directory     string `yaml:"directory"`
	MaxFileSize   string `yaml:"max_file_size"`
	RetentionDays int    `yaml:"retention_days"`
}

// SpectatingConfig represents spectating configuration
type SpectatingConfig struct {
	Enabled                 bool   `yaml:"enabled"`
	MaxSpectatorsPerSession int    `yaml:"max_spectators_per_session"`
	SpectatorTimeout        string `yaml:"spectator_timeout"`
}

// HeartbeatConfig represents general heartbeat configuration
type HeartbeatConfig struct {
	Enabled                bool              `yaml:"enabled"`
	Interval               string            `yaml:"interval"`
	IdleDetectionThreshold string            `yaml:"idle_detection_threshold"`
	IdleRetryInterval      string            `yaml:"idle_retry_interval"`
	GRPCStream             *GRPCStreamConfig `yaml:"grpc_stream"`
}

// GRPCStreamConfig represents gRPC stream heartbeat configuration
type GRPCStreamConfig struct {
	Enabled      bool   `yaml:"enabled"`
	PingInterval string `yaml:"ping_interval"`
	PongTimeout  string `yaml:"pong_timeout"`
}

// Load configuration from file
func Load(configPath string) (map[string]interface{}, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables
	expanded := os.ExpandEnv(string(data))

	var config map[string]interface{}
	if err := yaml.Unmarshal([]byte(expanded), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return config, nil
}

// Helper functions

// ParseTerminalSize parses terminal size string like "80x24"
func ParseTerminalSize(sizeStr string) (width, height int, err error) {
	n, err := fmt.Sscanf(sizeStr, "%dx%d", &width, &height)
	if err != nil || n != 2 {
		return 0, 0, fmt.Errorf("invalid terminal size format: %s", sizeStr)
	}
	return width, height, nil
}

// FormatTerminalSize formats terminal size as string
func FormatTerminalSize(width, height int) string {
	return fmt.Sprintf("%dx%d", width, height)
}

// GetDefaultTerminalSize returns default terminal size as width and height
func GetDefaultTerminalSize() (width, height int) {
	return 80, 24
}

// ParseDuration parses duration string with fallback
func ParseDuration(durationStr string, fallback time.Duration) time.Duration {
	if duration, err := time.ParseDuration(durationStr); err == nil {
		return duration
	}
	return fallback
}
