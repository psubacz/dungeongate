package config

// UserConfig represents user service configuration for session service
type UserConfig struct {
	LoginAttempts *LoginAttemptsConfig `yaml:"login_attempts"`
}
