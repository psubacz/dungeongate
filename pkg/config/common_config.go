package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// CommonConfig represents shared configuration across all services
type CommonConfig struct {
	Version     string                `yaml:"version"`
	Database    *DatabaseConfig       `yaml:"database"`
	Logging     *LoggingConfig        `yaml:"logging"`
	HealthCheck *HealthConfig         `yaml:"health_check"`
	Security    *CommonSecurityConfig `yaml:"security"`
	Server      *CommonServerConfig   `yaml:"server"`
	Environment *EnvironmentConfig    `yaml:"environment"`
}

// CommonSecurityConfig represents shared security configuration
type CommonSecurityConfig struct {
	RateLimiting         *CommonRateLimitingConfig   `yaml:"rate_limiting"`
	BruteForceProtection *BruteForceProtectionConfig `yaml:"brute_force_protection"`
}

// CommonServerConfig represents shared server configuration
type CommonServerConfig struct {
	Host             string `yaml:"host"`
	Timeout          string `yaml:"timeout"`
	ReadTimeout      string `yaml:"read_timeout"`
	WriteTimeout     string `yaml:"write_timeout"`
	IdleTimeout      string `yaml:"idle_timeout"`
	GracefulShutdown bool   `yaml:"graceful_shutdown"`
	ShutdownTimeout  string `yaml:"shutdown_timeout"`
}

// CommonRateLimitingConfig represents rate limiting configuration
type CommonRateLimitingConfig struct {
	Enabled       bool   `yaml:"enabled"`
	MaxRequests   int    `yaml:"max_requests"`
	Window        string `yaml:"window"`
	SkipLocalhost bool   `yaml:"skip_localhost"`
}

// BruteForceProtectionConfig represents brute force protection configuration
type BruteForceProtectionConfig struct {
	Enabled         bool   `yaml:"enabled"`
	MaxAttempts     int    `yaml:"max_attempts"`
	LockoutDuration string `yaml:"lockout_duration"`
	ResetAfter      string `yaml:"reset_after"`
}

// EnvironmentConfig represents environment metadata
type EnvironmentConfig struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Debug       bool              `yaml:"debug"`
	Monitoring  *MonitoringConfig `yaml:"monitoring"`
	Metrics     *MetricsConfig    `yaml:"metrics"`
}

// LoadCommonConfig loads the common configuration file
func LoadCommonConfig(configPath string) (*CommonConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read common config file: %w", err)
	}

	expanded := os.ExpandEnv(string(data))

	var config CommonConfig
	if err := yaml.Unmarshal([]byte(expanded), &config); err != nil {
		return nil, fmt.Errorf("failed to parse common config: %w", err)
	}

	return &config, nil
}

// MergeWithCommonUser merges user service config with common config
// Service-specific values take precedence over common values
func MergeWithCommonUser(serviceConfig *UserServiceConfig, commonConfig *CommonConfig) {
	// Merge database configuration
	if serviceConfig.Database == nil && commonConfig.Database != nil {
		serviceConfig.Database = commonConfig.Database
	}

	// Merge logging configuration
	if serviceConfig.Logging == nil && commonConfig.Logging != nil {
		// Use common logging config if service doesn't have one
		serviceConfig.Logging = &LoggingConfig{
			Level:  commonConfig.Logging.Level,
			Format: commonConfig.Logging.Format,
			Output: commonConfig.Logging.Output,
		}
	} else if serviceConfig.Logging != nil && commonConfig.Logging != nil {
		// Merge logging fields, keeping service-specific overrides
		mergeLoggingConfigWithCommon(serviceConfig.Logging, commonConfig.Logging)
	}

	// Merge health configuration
	if serviceConfig.Health == nil && commonConfig.HealthCheck != nil {
		serviceConfig.Health = commonConfig.HealthCheck
	}

	// Merge server configuration
	if serviceConfig.Server != nil && commonConfig.Server != nil {
		mergeServerConfig(serviceConfig.Server, commonConfig.Server)
	}
}

// MergeWithCommon merges service-specific config with common config
// Service-specific values take precedence over common values
func MergeWithCommon(serviceConfig *GameServiceConfig, commonConfig *CommonConfig) {
	// Merge database configuration
	if serviceConfig.Database == nil && commonConfig.Database != nil {
		serviceConfig.Database = commonConfig.Database
	}

	// Merge logging configuration
	if serviceConfig.Logging == nil && commonConfig.Logging != nil {
		// Use common logging config if service doesn't have one
		serviceConfig.Logging = &LoggingConfig{
			Level:  commonConfig.Logging.Level,
			Format: commonConfig.Logging.Format,
			Output: commonConfig.Logging.Output,
		}
	} else if serviceConfig.Logging != nil && commonConfig.Logging != nil {
		// Merge logging fields, keeping service-specific overrides
		mergeLoggingConfigWithCommon(serviceConfig.Logging, commonConfig.Logging)
	}

	// Merge health configuration
	if serviceConfig.Health == nil && commonConfig.HealthCheck != nil {
		serviceConfig.Health = commonConfig.HealthCheck
	}

	// Merge server configuration
	if serviceConfig.Server != nil && commonConfig.Server != nil {
		mergeServerConfig(serviceConfig.Server, commonConfig.Server)
	}
}

// mergeLoggingConfigWithCommon merges logging configuration with service overrides taking precedence
func mergeLoggingConfigWithCommon(service *LoggingConfig, common *LoggingConfig) {
	if service.Level == "" {
		service.Level = common.Level
	}
	if service.Format == "" {
		service.Format = common.Format
	}
	if service.Output == "" {
		service.Output = common.Output
	}
	// Merge file config if service doesn't have one but common does
	if service.File == nil && common.File != nil {
		service.File = common.File
	}
	// Merge journald config if service doesn't have one but common does
	if service.Journald == nil && common.Journald != nil {
		service.Journald = common.Journald
	}
}

// mergeServerConfig merges server configuration
func mergeServerConfig(service *ServerConfig, common *CommonServerConfig) {
	if service.Host == "" {
		service.Host = common.Host
	}
	if service.Timeout == "" {
		service.Timeout = common.Timeout
	}
}

// FindCommonConfig finds the common.yaml file relative to a service config file
func FindCommonConfig(serviceConfigPath string) string {
	dir := filepath.Dir(serviceConfigPath)
	return filepath.Join(dir, "common.yaml")
}
