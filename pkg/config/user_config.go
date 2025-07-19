package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DatabaseMode represents the database operational mode
type DatabaseMode string

const (
	DatabaseModeEmbedded DatabaseMode = "embedded" // SQLite for testing/development
	DatabaseModeExternal DatabaseMode = "external" // PostgreSQL/MySQL for production
)

// DatabaseConfig with dual mode support
type DatabaseConfig struct {
	Mode       DatabaseMode           `yaml:"mode"`           // embedded or external
	Type       string                 `yaml:"type"`           // sqlite, postgresql, mysql
	Connection map[string]interface{} `yaml:"connection"`     // Legacy connection config
	Embedded   *EmbeddedDBConfig      `yaml:"embedded"`       // Embedded database config
	External   *ExternalDBConfig      `yaml:"external"`       // External database config
	Settings   *DatabaseSettings      `yaml:"settings"`       // Common settings
	Pool       *PoolConfig            `yaml:"pool,omitempty"` // Pool configuration for compatibility
}

// EmbeddedDBConfig represents embedded database configuration (SQLite)
type EmbeddedDBConfig struct {
	Type            string       `yaml:"type"`             // sqlite, leveldb, etc.
	Path            string       `yaml:"path"`             // Database file path
	MigrationPath   string       `yaml:"migration_path"`   // Migration files path
	BackupEnabled   bool         `yaml:"backup_enabled"`   // Enable automatic backups
	BackupInterval  string       `yaml:"backup_interval"`  // Backup interval
	BackupRetention int          `yaml:"backup_retention"` // Number of backups to keep
	WALMode         bool         `yaml:"wal_mode"`         // SQLite WAL mode
	Cache           *CacheConfig `yaml:"cache"`            // Cache configuration
}

// ExternalDBConfig represents external database configuration with read/write separation
type ExternalDBConfig struct {
	Type string `yaml:"type"` // postgresql, mysql

	// Writer endpoint configuration
	WriterEndpoint string `yaml:"writer_endpoint"` // Writer endpoint (host:port)

	// Reader endpoint configuration
	ReaderUseWriter bool   `yaml:"reader_use_writer"` // Use writer endpoint for reads
	ReaderEndpoint  string `yaml:"reader_endpoint"`   // Reader endpoint (host:port)

	// Legacy single endpoint support (deprecated)
	Host string `yaml:"host,omitempty"` // Database host (legacy)
	Port int    `yaml:"port,omitempty"` // Database port (legacy)

	// Database credentials and settings
	Database string `yaml:"database"` // Database name
	Username string `yaml:"username"` // Database username
	Password string `yaml:"password"` // Database password
	SSLMode  string `yaml:"ssl_mode"` // SSL mode

	// Connection pool settings
	MaxConnections  int    `yaml:"max_connections"`   // Max total connections
	MaxIdleConns    int    `yaml:"max_idle_conns"`    // Max idle connections
	ConnMaxLifetime string `yaml:"conn_max_lifetime"` // Connection max lifetime

	// Reader-specific connection pool settings
	ReaderMaxConnections int `yaml:"reader_max_connections"` // Max reader connections
	ReaderMaxIdleConns   int `yaml:"reader_max_idle_conns"`  // Max reader idle connections

	// Schema and migration settings
	MigrationPath string `yaml:"migration_path"` // Migration files path
	Schema        string `yaml:"schema"`         // Database schema

	// Additional connection options
	Options map[string]string `yaml:"options"` // Additional options

	// Failover settings
	Failover *FailoverConfig `yaml:"failover"` // Failover configuration
}

// FailoverConfig represents database failover configuration
type FailoverConfig struct {
	Enabled                bool   `yaml:"enabled"`                   // Enable automatic failover
	HealthCheckInterval    string `yaml:"health_check_interval"`     // Health check interval
	FailoverTimeout        string `yaml:"failover_timeout"`          // Timeout before failover
	RetryInterval          string `yaml:"retry_interval"`            // Retry interval
	MaxRetries             int    `yaml:"max_retries"`               // Maximum retry attempts
	ReaderToWriterFallback bool   `yaml:"reader_to_writer_fallback"` // Fallback reads to writer on failure
}

// DatabaseEndpoints represents the actual connection endpoints
type DatabaseEndpoints struct {
	Writer string
	Reader string
}

// DatabaseSettings represents common database settings
type DatabaseSettings struct {
	LogQueries     bool   `yaml:"log_queries"`     // Log SQL queries
	Timeout        string `yaml:"timeout"`         // Query timeout
	RetryAttempts  int    `yaml:"retry_attempts"`  // Number of retry attempts
	RetryDelay     string `yaml:"retry_delay"`     // Delay between retries
	HealthCheck    bool   `yaml:"health_check"`    // Enable health checks
	HealthInterval string `yaml:"health_interval"` // Health check interval
	MetricsEnabled bool   `yaml:"metrics_enabled"` // Enable database metrics
}

// CacheConfig represents database cache configuration
type CacheConfig struct {
	Enabled   bool   `yaml:"enabled"`    // Enable caching
	Size      int    `yaml:"size"`       // Cache size in MB
	TTL       string `yaml:"ttl"`        // Time to live
	Type      string `yaml:"type"`       // Cache type (memory, redis)
	RedisAddr string `yaml:"redis_addr"` // Redis address for distributed cache
}

// UserServiceConfig represents user service configuration
type UserServiceConfig struct {
	InheritFrom    string              `yaml:"inherit_from"`
	Server         *ServerConfig       `yaml:"server"`
	Database       *DatabaseConfig     `yaml:"database"`
	Registration   *RegistrationConfig `yaml:"registration"`
	Authentication *AuthConfig         `yaml:"auth"`
	Validation     *ValidationConfig   `yaml:"validation"`
	Security       *SecurityConfig     `yaml:"security"`
	Logging        *LoggingConfig      `yaml:"logging"`
	Health         *HealthConfig       `yaml:"health"`
	Metrics        *MetricsConfig      `yaml:"metrics"`
}

// RegistrationConfig represents registration configuration
type RegistrationConfig struct {
	Enabled           bool               `yaml:"enabled"`
	RequireEmail      bool               `yaml:"require_email"`
	RequireTerms      bool               `yaml:"require_terms"`
	EmailVerification bool               `yaml:"email_verification"`
	ManualApproval    bool               `yaml:"manual_approval"`
	DefaultRoles      []string           `yaml:"default_roles"`
	RateLimiting      *RateLimitConfig   `yaml:"rate_limiting"`
	Email             *EmailConfig       `yaml:"email"`
	Captcha           *CaptchaConfig     `yaml:"captcha"`
	Hooks             *RegistrationHooks `yaml:"hooks"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	PasswordExpiry        string               `yaml:"password_expiry"`
	SessionTimeout        string               `yaml:"session_timeout"`
	MaxConcurrentSessions int                  `yaml:"max_concurrent_sessions"`
	RequirePasswordChange bool                 `yaml:"require_password_change"`
	TwoFactorAuth         *TwoFactorConfig     `yaml:"two_factor_auth"`
	LoginAttempts         *LoginAttemptsConfig `yaml:"login_attempts"`
	RootAdminUser         *AdminUserConfig     `yaml:"root_admin_user"`
	AdminUsers            []AdminUserConfig    `yaml:"admin_users"`
}

// AdminUserConfig represents configuration for creating admin users
type AdminUserConfig struct {
	Enabled         bool   `yaml:"enabled"`
	Name            string `yaml:"name"`
	OneTimePassword string `yaml:"one_time_password"`
	RecoveryEmail   string `yaml:"recovery_email"`
}

// ValidationConfig represents validation configuration
type ValidationConfig struct {
	Username *UsernameValidation `yaml:"username"`
	Password *PasswordValidation `yaml:"password"`
	Email    *EmailValidation    `yaml:"email"`
}

// UsernameValidation represents username validation rules
type UsernameValidation struct {
	MinLength int      `yaml:"min_length"`
	MaxLength int      `yaml:"max_length"`
	Pattern   string   `yaml:"pattern"`
	Reserved  []string `yaml:"reserved"`
	Blacklist []string `yaml:"blacklist"`
}

// PasswordValidation represents password validation rules
type PasswordValidation struct {
	MinLength        int      `yaml:"min_length"`
	MaxLength        int      `yaml:"max_length"`
	RequireSpecial   bool     `yaml:"require_special"`
	RequireNumber    bool     `yaml:"require_number"`
	RequireUppercase bool     `yaml:"require_uppercase"`
	RequireLowercase bool     `yaml:"require_lowercase"`
	Forbidden        []string `yaml:"forbidden"`
	MinEntropy       float64  `yaml:"min_entropy"`
}

// EmailValidation represents email validation rules
type EmailValidation struct {
	Required       bool     `yaml:"required"`
	MaxLength      int      `yaml:"max_length"`
	DomainsAllowed []string `yaml:"domains_allowed"`
	DomainsBlocked []string `yaml:"domains_blocked"`
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	Enabled       bool   `yaml:"enabled"`
	MaxAttempts   int    `yaml:"max_attempts"`
	Window        string `yaml:"window"`
	BlockDuration string `yaml:"block_duration"`
}

// EmailConfig represents email configuration for registration
type EmailConfig struct {
	VerificationRequired bool     `yaml:"verification_required"`
	DomainsAllowed       []string `yaml:"domains_allowed"`
	DomainsBlocked       []string `yaml:"domains_blocked"`
	TemplatesPath        string   `yaml:"templates_path"`
}

// CaptchaConfig represents captcha configuration
type CaptchaConfig struct {
	Enabled   bool    `yaml:"enabled"`
	Provider  string  `yaml:"provider"` // recaptcha, hcaptcha
	SiteKey   string  `yaml:"site_key"`
	SecretKey string  `yaml:"secret_key"`
	Threshold float64 `yaml:"threshold"`
}

// RegistrationHooks represents registration hook configuration
type RegistrationHooks struct {
	PreRegistration  []string `yaml:"pre_registration"`
	PostRegistration []string `yaml:"post_registration"`
	OnFailure        []string `yaml:"on_failure"`
}

// TwoFactorConfig represents two-factor authentication configuration
type TwoFactorConfig struct {
	Enabled  bool     `yaml:"enabled"`
	Required bool     `yaml:"required"`
	Methods  []string `yaml:"methods"` // totp, sms, email
	Issuer   string   `yaml:"issuer"`
}

// LoginAttemptsConfig represents login attempts configuration
type LoginAttemptsConfig struct {
	MaxAttempts  int    `yaml:"max_attempts"`
	LockDuration string `yaml:"lock_duration"`
	ResetWindow  string `yaml:"reset_window"`
	Progressive  bool   `yaml:"progressive"` // Progressive delays
}

// ConvertLegacyToNew converts a legacy LegacyDatabaseConfig to a DatabaseConfig
// This handles the case where the YAML has the new structure but is loaded into a legacy config
func ConvertLegacyToNew(legacy *LegacyDatabaseConfig) (*DatabaseConfig, error) {
	if legacy == nil {
		return nil, fmt.Errorf("legacy database config is nil")
	}

	// Try to extract mode from the connection map
	mode, _ := legacy.Connection["mode"].(string)
	if mode == "" {
		// Default to embedded mode if not specified
		mode = string(DatabaseModeEmbedded)
	}

	config := &DatabaseConfig{
		Mode: DatabaseMode(mode),
		Type: legacy.Type,
		Pool: legacy.Pool,
		Settings: &DatabaseSettings{
			LogQueries:    false,
			Timeout:       "30s",
			RetryAttempts: 3,
			RetryDelay:    "1s",
		},
	}

	// Handle embedded configuration
	if embeddedMap, ok := legacy.Connection["embedded"].(map[string]interface{}); ok {
		config.Embedded = &EmbeddedDBConfig{
			Type:          getString(embeddedMap, "type", "sqlite"),
			Path:          getString(embeddedMap, "path", "./data/default.db"),
			MigrationPath: getString(embeddedMap, "migration_path", "./migrations"),
			BackupEnabled: getBool(embeddedMap, "backup_enabled", false),
			WALMode:       getBool(embeddedMap, "wal_mode", true),
		}

		// Handle cache config
		if cacheMap, ok := embeddedMap["cache"].(map[string]interface{}); ok {
			config.Embedded.Cache = &CacheConfig{
				Enabled: getBool(cacheMap, "enabled", true),
				Size:    getInt(cacheMap, "size", 64),
				TTL:     getString(cacheMap, "ttl", "1h"),
				Type:    getString(cacheMap, "type", "memory"),
			}
		}
	}

	// Handle external configuration
	if externalMap, ok := legacy.Connection["external"].(map[string]interface{}); ok {
		config.External = &ExternalDBConfig{
			Type:                 getString(externalMap, "type", "postgresql"),
			WriterEndpoint:       getString(externalMap, "writer_endpoint", "localhost:5432"),
			ReaderUseWriter:      getBool(externalMap, "reader_use_writer", true),
			ReaderEndpoint:       getString(externalMap, "reader_endpoint", "localhost:5432"),
			Database:             getString(externalMap, "database", "postgres"),
			Username:             getString(externalMap, "username", "postgres"),
			Password:             getString(externalMap, "password", ""),
			SSLMode:              getString(externalMap, "ssl_mode", "disable"),
			MaxConnections:       getInt(externalMap, "max_connections", 25),
			MaxIdleConns:         getInt(externalMap, "max_idle_conns", 10),
			ConnMaxLifetime:      getString(externalMap, "conn_max_lifetime", "1h"),
			ReaderMaxConnections: getInt(externalMap, "reader_max_connections", 15),
			ReaderMaxIdleConns:   getInt(externalMap, "reader_max_idle_conns", 5),
			MigrationPath:        getString(externalMap, "migration_path", "./migrations"),
			Schema:               getString(externalMap, "schema", "public"),
		}

		// Handle failover config
		if failoverMap, ok := externalMap["failover"].(map[string]interface{}); ok {
			config.External.Failover = &FailoverConfig{
				Enabled:                getBool(failoverMap, "enabled", true),
				HealthCheckInterval:    getString(failoverMap, "health_check_interval", "30s"),
				FailoverTimeout:        getString(failoverMap, "failover_timeout", "10s"),
				RetryInterval:          getString(failoverMap, "retry_interval", "5s"),
				MaxRetries:             getInt(failoverMap, "max_retries", 3),
				ReaderToWriterFallback: getBool(failoverMap, "reader_to_writer_fallback", true),
			}
		}
	}

	// Handle settings
	if settingsMap, ok := legacy.Connection["settings"].(map[string]interface{}); ok {
		config.Settings = &DatabaseSettings{
			LogQueries:    getBool(settingsMap, "log_queries", false),
			Timeout:       getString(settingsMap, "timeout", "30s"),
			RetryAttempts: getInt(settingsMap, "retry_attempts", 3),
			RetryDelay:    getString(settingsMap, "retry_delay", "1s"),
		}
	}

	return config, nil
}

// Helper functions for type conversion
func getString(m map[string]interface{}, key, defaultValue string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return defaultValue
}

func getBool(m map[string]interface{}, key string, defaultValue bool) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return defaultValue
}

func getInt(m map[string]interface{}, key string, defaultValue int) int {
	if v, ok := m[key].(int); ok {
		return v
	}
	// Handle float64 from YAML parsing
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return defaultValue
}

// GetConnectionString returns the appropriate connection string based on mode
func (c *DatabaseConfig) GetConnectionString() (string, error) {
	switch c.Mode {
	case DatabaseModeEmbedded:
		if c.Embedded == nil {
			return "", fmt.Errorf("embedded configuration is required for embedded mode")
		}
		return c.getEmbeddedConnectionString()
	case DatabaseModeExternal:
		if c.External == nil {
			return "", fmt.Errorf("external configuration is required for external mode")
		}
		return c.getExternalConnectionString("writer")
	default:
		return "", fmt.Errorf("unsupported database mode: %s", c.Mode)
	}
}

// GetWriterConnectionString returns the writer connection string
func (c *DatabaseConfig) GetWriterConnectionString() (string, error) {
	if c.Mode != DatabaseModeExternal {
		return c.GetConnectionString()
	}
	return c.getExternalConnectionString("writer")
}

// GetReaderConnectionString returns the reader connection string
func (c *DatabaseConfig) GetReaderConnectionString() (string, error) {
	if c.Mode != DatabaseModeExternal {
		return c.GetConnectionString()
	}
	return c.getExternalConnectionString("reader")
}

// GetEndpoints returns the database endpoints configuration
func (c *DatabaseConfig) GetEndpoints() (*DatabaseEndpoints, error) {
	if c.Mode != DatabaseModeExternal {
		connStr, err := c.GetConnectionString()
		return &DatabaseEndpoints{
			Writer: connStr,
			Reader: connStr,
		}, err
	}

	endpoints := &DatabaseEndpoints{}

	// Parse writer endpoint
	if c.External.WriterEndpoint == "" {
		// Fallback to legacy host:port format
		if c.External.Host != "" && c.External.Port > 0 {
			endpoints.Writer = fmt.Sprintf("%s:%d", c.External.Host, c.External.Port)
		} else {
			return nil, fmt.Errorf("writer endpoint not configured")
		}
	} else {
		endpoints.Writer = c.External.WriterEndpoint
	}

	// Parse reader endpoint
	if c.External.ReaderUseWriter {
		endpoints.Reader = endpoints.Writer
	} else {
		if c.External.ReaderEndpoint == "" {
			return nil, fmt.Errorf("reader endpoint not configured when reader_use_writer is false")
		}
		endpoints.Reader = c.External.ReaderEndpoint
	}

	return endpoints, nil
}

// getEmbeddedConnectionString returns connection string for embedded database
func (c *DatabaseConfig) getEmbeddedConnectionString() (string, error) {
	switch c.Embedded.Type {
	case "sqlite":
		params := "?_journal_mode=WAL&_sync=NORMAL&_cache_size=1000"
		if !c.Embedded.WALMode {
			params = "?_journal_mode=DELETE"
		}
		return c.Embedded.Path + params, nil
	default:
		return "", fmt.Errorf("unsupported embedded database type: %s", c.Embedded.Type)
	}
}

// getExternalConnectionString returns connection string for external database
func (c *DatabaseConfig) getExternalConnectionString(endpoint string) (string, error) {
	endpoints, err := c.GetEndpoints()
	if err != nil {
		return "", err
	}

	var hostPort string
	switch endpoint {
	case "writer":
		hostPort = endpoints.Writer
	case "reader":
		hostPort = endpoints.Reader
	default:
		return "", fmt.Errorf("invalid endpoint type: %s", endpoint)
	}

	// Parse host and port from endpoint
	parts := strings.Split(hostPort, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid endpoint format: %s", hostPort)
	}

	host := parts[0]
	port := parts[1]

	switch c.External.Type {
	case "postgresql":
		connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			host, port, c.External.Username, c.External.Password,
			c.External.Database, c.External.SSLMode)

		// Add additional options
		for key, value := range c.External.Options {
			connStr += fmt.Sprintf(" %s=%s", key, value)
		}

		return connStr, nil

	case "mysql":
		connStr := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
			c.External.Username, c.External.Password, host, port, c.External.Database)

		// Add SSL mode for MySQL
		if c.External.SSLMode != "" {
			connStr += "&tls=" + c.External.SSLMode
		}

		// Add additional options
		for key, value := range c.External.Options {
			connStr += fmt.Sprintf("&%s=%s", key, value)
		}

		return connStr, nil

	default:
		return "", fmt.Errorf("unsupported external database type: %s", c.External.Type)
	}
}

// GetDatabaseType returns the database type based on mode
func (c *DatabaseConfig) GetDatabaseType() string {
	switch c.Mode {
	case DatabaseModeEmbedded:
		if c.Embedded != nil {
			return c.Embedded.Type
		}
	case DatabaseModeExternal:
		if c.External != nil {
			return c.External.Type
		}
	}
	return c.Type // Fallback to legacy type
}

// IsEmbedded returns true if using embedded database
func (c *DatabaseConfig) IsEmbedded() bool {
	return c.Mode == DatabaseModeEmbedded
}

// IsExternal returns true if using external database
func (c *DatabaseConfig) IsExternal() bool {
	return c.Mode == DatabaseModeExternal
}

// GetMigrationPath returns the migration path
func (c *DatabaseConfig) GetMigrationPath() string {
	switch c.Mode {
	case DatabaseModeEmbedded:
		if c.Embedded != nil && c.Embedded.MigrationPath != "" {
			return c.Embedded.MigrationPath
		}
	case DatabaseModeExternal:
		if c.External != nil && c.External.MigrationPath != "" {
			return c.External.MigrationPath
		}
	}
	return "./migrations"
}

// NewDatabaseConfig creates a new enhanced database configuration with defaults
func NewDatabaseConfig() *DatabaseConfig {
	return &DatabaseConfig{
		Mode: DatabaseModeEmbedded,
		Type: "sqlite",
		Embedded: &EmbeddedDBConfig{
			Type:            "sqlite",
			Path:            "./data/users.db",
			MigrationPath:   "./migrations",
			BackupEnabled:   true,
			BackupInterval:  "24h",
			BackupRetention: 7,
			WALMode:         true,
			Cache: &CacheConfig{
				Enabled: true,
				Size:    64,
				TTL:     "1h",
				Type:    "memory",
			},
		},
		External: &ExternalDBConfig{
			Type:                 "postgresql",
			WriterEndpoint:       "localhost:5432",
			ReaderUseWriter:      true,
			ReaderEndpoint:       "",
			Database:             "dungeongate",
			Username:             "dungeongate",
			Password:             "",
			SSLMode:              "require",
			MaxConnections:       100,
			MaxIdleConns:         10,
			ReaderMaxConnections: 50,
			ReaderMaxIdleConns:   5,
			ConnMaxLifetime:      "1h",
			MigrationPath:        "./migrations",
			Schema:               "public",
			Options:              make(map[string]string),
			Failover: &FailoverConfig{
				Enabled:                true,
				HealthCheckInterval:    "30s",
				FailoverTimeout:        "10s",
				RetryInterval:          "5s",
				MaxRetries:             3,
				ReaderToWriterFallback: true,
			},
		},
		Settings: &DatabaseSettings{
			LogQueries:     false,
			Timeout:        "30s",
			RetryAttempts:  3,
			RetryDelay:     "1s",
			HealthCheck:    true,
			HealthInterval: "30s",
			MetricsEnabled: true,
		},
	}
}

// NewUserServiceConfig creates a new user service configuration with defaults
func NewUserServiceConfig() *UserServiceConfig {
	return &UserServiceConfig{
		Server: &ServerConfig{
			Port:           8084,
			GRPCPort:       9084,
			Host:           "localhost",
			Timeout:        "30s",
			MaxConnections: 100,
		},
		Database: NewDatabaseConfig(),
		Registration: &RegistrationConfig{
			Enabled:           true,
			RequireEmail:      false,
			RequireTerms:      true,
			EmailVerification: false,
			ManualApproval:    false,
			DefaultRoles:      []string{"user"},
			RateLimiting: &RateLimitConfig{
				Enabled:       true,
				MaxAttempts:   5,
				Window:        "1h",
				BlockDuration: "15m",
			},
			Email: &EmailConfig{
				VerificationRequired: false,
				DomainsAllowed:       []string{},
				DomainsBlocked:       []string{},
			},
			Captcha: &CaptchaConfig{
				Enabled:   false,
				Provider:  "recaptcha",
				Threshold: 0.5,
			},
		},
		Authentication: &AuthConfig{
			PasswordExpiry:        "0", // Never expire
			SessionTimeout:        "24h",
			MaxConcurrentSessions: 5,
			RequirePasswordChange: false,
			TwoFactorAuth: &TwoFactorConfig{
				Enabled:  false,
				Required: false,
				Methods:  []string{"totp"},
				Issuer:   "DungeonGate",
			},
			LoginAttempts: &LoginAttemptsConfig{
				MaxAttempts:  5,
				LockDuration: "15m",
				ResetWindow:  "1h",
				Progressive:  true,
			},
			RootAdminUser: &AdminUserConfig{
				Enabled:         false,
				Name:            "admin",
				OneTimePassword: "",
				RecoveryEmail:   "",
			},
			AdminUsers: []AdminUserConfig{},
		},
		Validation: &ValidationConfig{
			Username: &UsernameValidation{
				MinLength: 2,
				MaxLength: 20,
				Pattern:   "^[a-zA-Z0-9_]+$",
				Reserved:  []string{"admin", "root", "guest", "anonymous", "system"},
				Blacklist: []string{},
			},
			Password: &PasswordValidation{
				MinLength:        6,
				MaxLength:        128,
				RequireSpecial:   false,
				RequireNumber:    false,
				RequireUppercase: false,
				RequireLowercase: false,
				Forbidden:        []string{"password", "123456", "admin", "guest"},
				MinEntropy:       0.0,
			},
			Email: &EmailValidation{
				Required:       false,
				MaxLength:      80,
				DomainsAllowed: []string{},
				DomainsBlocked: []string{},
			},
		},
		Security: &SecurityConfig{
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
		},
	}
}

// LoadUserServiceConfig loads user service configuration from file
func LoadUserServiceConfig(configPath string) (*UserServiceConfig, error) {
	// If no config file provided, return default configuration
	if configPath == "" {
		return NewUserServiceConfig(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables
	expanded := os.ExpandEnv(string(data))

	var config UserServiceConfig
	if err := yaml.Unmarshal([]byte(expanded), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Load and merge common configuration if specified
	if config.InheritFrom != "" {
		commonConfigPath := FindCommonConfig(configPath)
		if config.InheritFrom == "common.yaml" {
			// Use the common.yaml in the same directory
			commonConfig, err := LoadCommonConfig(commonConfigPath)
			if err != nil {
				return nil, fmt.Errorf("failed to load common config: %w", err)
			}
			MergeWithCommonUser(&config, commonConfig)
		}
	}

	// Apply defaults for missing fields
	if config.Database == nil {
		config.Database = NewDatabaseConfig()
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &config, nil
}

// Validate validates the user service configuration
func (c *UserServiceConfig) Validate() error {
	if c.Database == nil {
		return fmt.Errorf("database configuration is required")
	}

	if err := c.Database.Validate(); err != nil {
		return fmt.Errorf("database configuration validation failed: %w", err)
	}

	// Validate other sections...
	return nil
}

// Validate validates the enhanced database configuration
func (c *DatabaseConfig) Validate() error {
	if c.Mode == "" {
		return fmt.Errorf("database mode is required")
	}

	switch c.Mode {
	case DatabaseModeEmbedded:
		if c.Embedded == nil {
			return fmt.Errorf("embedded configuration is required for embedded mode")
		}
		return c.validateEmbedded()
	case DatabaseModeExternal:
		if c.External == nil {
			return fmt.Errorf("external configuration is required for external mode")
		}
		return c.validateExternal()
	default:
		return fmt.Errorf("unsupported database mode: %s", c.Mode)
	}
}

// validateEmbedded validates embedded database configuration
func (c *DatabaseConfig) validateEmbedded() error {
	if c.Embedded.Type == "" {
		return fmt.Errorf("embedded database type is required")
	}
	if c.Embedded.Path == "" {
		return fmt.Errorf("embedded database path is required")
	}

	// Ensure directory exists
	if err := os.MkdirAll(getDir(c.Embedded.Path), 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	// Validate backup interval if backups are enabled
	if c.Embedded.BackupEnabled && c.Embedded.BackupInterval != "" {
		if _, err := time.ParseDuration(c.Embedded.BackupInterval); err != nil {
			return fmt.Errorf("invalid backup interval: %w", err)
		}
	}

	return nil
}

// validateExternal validates external database configuration
func (c *DatabaseConfig) validateExternal() error {
	if c.External.Type == "" {
		return fmt.Errorf("external database type is required")
	}

	// Validate writer endpoint
	if c.External.WriterEndpoint == "" {
		// Check legacy host/port configuration
		if c.External.Host == "" {
			return fmt.Errorf("writer endpoint or legacy host is required")
		}
		if c.External.Port == 0 {
			return fmt.Errorf("writer endpoint or legacy port is required")
		}
	} else {
		// Validate endpoint format
		if err := c.validateEndpointFormat(c.External.WriterEndpoint); err != nil {
			return fmt.Errorf("invalid writer endpoint format: %w", err)
		}
	}

	// Validate reader endpoint if not using writer
	if !c.External.ReaderUseWriter {
		if c.External.ReaderEndpoint == "" {
			return fmt.Errorf("reader endpoint is required when reader_use_writer is false")
		}
		if err := c.validateEndpointFormat(c.External.ReaderEndpoint); err != nil {
			return fmt.Errorf("invalid reader endpoint format: %w", err)
		}
	}

	if c.External.Database == "" {
		return fmt.Errorf("database name is required")
	}
	if c.External.Username == "" {
		return fmt.Errorf("database username is required")
	}

	// Validate connection lifetime
	if c.External.ConnMaxLifetime != "" {
		if _, err := time.ParseDuration(c.External.ConnMaxLifetime); err != nil {
			return fmt.Errorf("invalid connection max lifetime: %w", err)
		}
	}

	// Validate failover configuration
	if c.External.Failover != nil && c.External.Failover.Enabled {
		if err := c.validateFailoverConfig(); err != nil {
			return fmt.Errorf("failover configuration validation failed: %w", err)
		}
	}

	return nil
}

// validateEndpointFormat validates the format of a database endpoint
func (c *DatabaseConfig) validateEndpointFormat(endpoint string) error {
	parts := strings.Split(endpoint, ":")
	if len(parts) != 2 {
		return fmt.Errorf("endpoint must be in format 'host:port'")
	}

	host := strings.TrimSpace(parts[0])
	port := strings.TrimSpace(parts[1])

	if host == "" {
		return fmt.Errorf("host cannot be empty")
	}

	if port == "" {
		return fmt.Errorf("port cannot be empty")
	}

	// Validate port is numeric
	if _, err := strconv.Atoi(port); err != nil {
		return fmt.Errorf("port must be numeric: %w", err)
	}

	return nil
}

// validateFailoverConfig validates failover configuration
func (c *DatabaseConfig) validateFailoverConfig() error {
	failover := c.External.Failover

	if failover.HealthCheckInterval != "" {
		if _, err := time.ParseDuration(failover.HealthCheckInterval); err != nil {
			return fmt.Errorf("invalid health check interval: %w", err)
		}
	}

	if failover.FailoverTimeout != "" {
		if _, err := time.ParseDuration(failover.FailoverTimeout); err != nil {
			return fmt.Errorf("invalid failover timeout: %w", err)
		}
	}

	if failover.RetryInterval != "" {
		if _, err := time.ParseDuration(failover.RetryInterval); err != nil {
			return fmt.Errorf("invalid retry interval: %w", err)
		}
	}

	if failover.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative")
	}

	return nil
}

// Helper function to get directory from file path
func getDir(filePath string) string {
	for i := len(filePath) - 1; i >= 0; i-- {
		if filePath[i] == '/' || filePath[i] == '\\' {
			return filePath[:i]
		}
	}
	return "."
}
