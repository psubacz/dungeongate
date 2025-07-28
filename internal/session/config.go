package session

import "time"

// Config represents the configuration for the Session Service
type Config struct {
	// Version information
	Version string `yaml:"-"`

	// Service addresses
	GameService struct {
		Address string `yaml:"address" default:"localhost:50051"`
	} `yaml:"game_service"`

	AuthService struct {
		Address string `yaml:"address" default:"localhost:8082"`
	} `yaml:"auth_service"`

	// Server configuration
	SSH struct {
		Address         string `yaml:"address" default:"0.0.0.0"`
		Port            int    `yaml:"port" default:"2222"`
		IdleTimeout     string `yaml:"idle_timeout" default:"1h"`
		HostKey         string `yaml:"host_key" default:""`
		PasswordAuth    bool   `yaml:"password_auth" default:"true"`
		PublicKeyAuth   bool   `yaml:"public_key_auth" default:"false"`
		AllowAnonymous  bool   `yaml:"allow_anonymous" default:"true"`
		AllowedUsername string `yaml:"allowed_username" default:"dungeongate"`
		SSHPassword     string `yaml:"ssh_password" default:""`
	} `yaml:"ssh"`

	HTTP struct {
		Address string `yaml:"address" default:"0.0.0.0"`
		Port    int    `yaml:"port" default:"8083"`
	} `yaml:"http"`

	GRPC struct {
		Address string `yaml:"address" default:"0.0.0.0"`
		Port    int    `yaml:"port" default:"9093"`
	} `yaml:"grpc"`

	// Resource limits
	MaxConnections int `yaml:"max_connections" default:"1000"`
	MaxPTYs        int `yaml:"max_ptys" default:"500"`

	// Connection settings
	ConnectionTimeout time.Duration `yaml:"connection_timeout" default:"30s"`
	IdleTimeout       time.Duration `yaml:"idle_timeout" default:"1h"`

	// Rate limiting
	MaxConnectionsPerIP int           `yaml:"max_connections_per_ip" default:"10"`
	RateLimitWindow     time.Duration `yaml:"rate_limit_window" default:"1m"`

	// Terminal settings
	DefaultTerminalType string `yaml:"default_terminal_type" default:"xterm-256color"`
	MaxTerminalCols     int    `yaml:"max_terminal_cols" default:"200"`
	MaxTerminalRows     int    `yaml:"max_terminal_rows" default:"100"`

	// Streaming settings
	SpectatorBufferSize int           `yaml:"spectator_buffer_size" default:"1024"`
	StreamTimeout       time.Duration `yaml:"stream_timeout" default:"10s"`

	// Circuit breaker settings
	CircuitBreakerThreshold int           `yaml:"circuit_breaker_threshold" default:"5"`
	CircuitBreakerTimeout   time.Duration `yaml:"circuit_breaker_timeout" default:"60s"`

	// Health check settings
	HealthCheckInterval time.Duration `yaml:"health_check_interval" default:"30s"`
	HealthCheckTimeout  time.Duration `yaml:"health_check_timeout" default:"5s"`

	// Idle mode settings
	IdleRetryInterval time.Duration `yaml:"idle_retry_interval" default:"5s"`

	// Menu configuration
	Menu struct {
		Banners struct {
			MainAnon           string `yaml:"main_anon" default:"./assets/banners/main_anon.txt"`
			MainUser           string `yaml:"main_user" default:"./assets/banners/main_user.txt"`
			MainAdmin          string `yaml:"main_admin" default:"./assets/banners/main_admin.txt"`
			WatchMenu          string `yaml:"watch_menu" default:"./assets/banners/watch_menu.txt"`
			ServiceUnavailable string `yaml:"service_unavailable" default:"./assets/banners/service_unavailable.txt"`
		} `yaml:"banners"`
	} `yaml:"menu"`
}
