package application

// CreateGameRequest represents a request to create a new game
type CreateGameRequest struct {
	// Identity
	ID        string `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"short_name"`

	// Metadata
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	Version     string   `json:"version"`
	Difficulty  int      `json:"difficulty"`

	// Binary configuration
	BinaryPath       string   `json:"binary_path"`
	BinaryArgs       []string `json:"binary_args"`
	WorkingDirectory string   `json:"working_directory"`

	// Environment
	Environment map[string]string `json:"environment"`

	// Resource limits
	CPULimit       string `json:"cpu_limit"`
	MemoryLimit    string `json:"memory_limit"`
	DiskLimit      string `json:"disk_limit"`
	TimeoutSeconds int    `json:"timeout_seconds"`

	// Security settings
	RunAsUser                uint32   `json:"run_as_user"`
	RunAsGroup               uint32   `json:"run_as_group"`
	ReadOnlyRootFilesystem   bool     `json:"read_only_root_filesystem"`
	AllowPrivilegeEscalation bool     `json:"allow_privilege_escalation"`
	Capabilities             []string `json:"capabilities"`

	// Networking settings
	NetworkIsolated bool     `json:"network_isolated"`
	AllowedPorts    []int    `json:"allowed_ports"`
	AllowedDomains  []string `json:"allowed_domains"`
	BlockInternet   bool     `json:"block_internet"`
}

// UpdateGameRequest represents a request to update an existing game
type UpdateGameRequest struct {
	Metadata *UpdateGameMetadata `json:"metadata,omitempty"`
	Config   *UpdateGameConfig   `json:"config,omitempty"`
	Status   *string             `json:"status,omitempty"`
}

// UpdateGameMetadata represents metadata updates for a game
type UpdateGameMetadata struct {
	Name        string   `json:"name"`
	ShortName   string   `json:"short_name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	Version     string   `json:"version"`
	Difficulty  int      `json:"difficulty"`
}

// UpdateGameConfig represents configuration updates for a game
type UpdateGameConfig struct {
	// Binary configuration
	BinaryPath       string   `json:"binary_path"`
	BinaryArgs       []string `json:"binary_args"`
	WorkingDirectory string   `json:"working_directory"`

	// Environment
	Environment map[string]string `json:"environment"`

	// Resource limits
	CPULimit       string `json:"cpu_limit"`
	MemoryLimit    string `json:"memory_limit"`
	DiskLimit      string `json:"disk_limit"`
	TimeoutSeconds int    `json:"timeout_seconds"`

	// Security settings
	RunAsUser                uint32   `json:"run_as_user"`
	RunAsGroup               uint32   `json:"run_as_group"`
	ReadOnlyRootFilesystem   bool     `json:"read_only_root_filesystem"`
	AllowPrivilegeEscalation bool     `json:"allow_privilege_escalation"`
	Capabilities             []string `json:"capabilities"`

	// Networking settings
	NetworkIsolated bool     `json:"network_isolated"`
	AllowedPorts    []int    `json:"allowed_ports"`
	AllowedDomains  []string `json:"allowed_domains"`
	BlockInternet   bool     `json:"block_internet"`
}

// StartSessionRequest represents a request to start a new game session
type StartSessionRequest struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	GameID   string `json:"game_id"`

	// Terminal configuration
	TerminalWidth  int `json:"terminal_width"`
	TerminalHeight int `json:"terminal_height"`

	// Session features
	EnableRecording  bool `json:"enable_recording"`
	EnableStreaming  bool `json:"enable_streaming"`
	EnableEncryption bool `json:"enable_encryption"`
}

// StopSessionRequest represents a request to stop a game session
type StopSessionRequest struct {
	SessionID string `json:"session_id"`
	Reason    string `json:"reason"`
	Force     bool   `json:"force"`
}

// AddSpectatorRequest represents a request to add a spectator to a session
type AddSpectatorRequest struct {
	SessionID         string `json:"session_id"`
	SpectatorUserID   int    `json:"spectator_user_id"`
	SpectatorUsername string `json:"spectator_username"`
}

// RemoveSpectatorRequest represents a request to remove a spectator from a session
type RemoveSpectatorRequest struct {
	SessionID       string `json:"session_id"`
	SpectatorUserID int    `json:"spectator_user_id"`
}

// SaveGameRequest represents a request to save a game
type SaveGameRequest struct {
	UserID   int                    `json:"user_id"`
	GameID   string                 `json:"game_id"`
	Data     []byte                 `json:"data"`
	Metadata map[string]interface{} `json:"metadata"`
}

// LoadGameRequest represents a request to load a game save
type LoadGameRequest struct {
	UserID int    `json:"user_id"`
	GameID string `json:"game_id"`
	SaveID string `json:"save_id,omitempty"`
}

// DeleteSaveRequest represents a request to delete a game save
type DeleteSaveRequest struct {
	UserID int    `json:"user_id"`
	GameID string `json:"game_id"`
	SaveID string `json:"save_id"`
}

// CreateBackupRequest represents a request to create a save backup
type CreateBackupRequest struct {
	UserID int    `json:"user_id"`
	GameID string `json:"game_id"`
	SaveID string `json:"save_id"`
	Reason string `json:"reason"`
}

// RestoreBackupRequest represents a request to restore from a backup
type RestoreBackupRequest struct {
	UserID   int    `json:"user_id"`
	GameID   string `json:"game_id"`
	SaveID   string `json:"save_id"`
	BackupID string `json:"backup_id"`
}

// ListSessionsRequest represents a request to list sessions
type ListSessionsRequest struct {
	UserID *int    `json:"user_id,omitempty"`
	GameID *string `json:"game_id,omitempty"`
	Status *string `json:"status,omitempty"`
	Limit  int     `json:"limit"`
	Offset int     `json:"offset"`
}

// ListGamesRequest represents a request to list games
type ListGamesRequest struct {
	Category    *string `json:"category,omitempty"`
	Tag         *string `json:"tag,omitempty"`
	Status      *string `json:"status,omitempty"`
	EnabledOnly bool    `json:"enabled_only"`
	Limit       int     `json:"limit"`
	Offset      int     `json:"offset"`
}

// Response types

// GameResponse represents a game in API responses
type GameResponse struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	ShortName   string            `json:"short_name"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	Tags        []string          `json:"tags"`
	Version     string            `json:"version"`
	Difficulty  int               `json:"difficulty"`
	Status      string            `json:"status"`
	Environment map[string]string `json:"environment"`
	Statistics  GameStatsResponse `json:"statistics"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

// GameStatsResponse represents game statistics in API responses
type GameStatsResponse struct {
	TotalSessions      int     `json:"total_sessions"`
	ActiveSessions     int     `json:"active_sessions"`
	TotalPlayTime      string  `json:"total_play_time"`
	AverageSessionTime string  `json:"average_session_time"`
	UniqueUsers        int     `json:"unique_users"`
	LastPlayed         *string `json:"last_played"`
	PopularityRank     int     `json:"popularity_rank"`
	Rating             float32 `json:"rating"`
}

// SessionResponse represents a session in API responses
type SessionResponse struct {
	ID           string              `json:"id"`
	UserID       int                 `json:"user_id"`
	Username     string              `json:"username"`
	GameID       string              `json:"game_id"`
	Status       string              `json:"status"`
	StartTime    string              `json:"start_time"`
	EndTime      *string             `json:"end_time"`
	Duration     string              `json:"duration"`
	TerminalSize string              `json:"terminal_size"`
	Spectators   []SpectatorResponse `json:"spectators"`
	ProcessPID   int                 `json:"process_pid,omitempty"`
	Recording    *RecordingResponse  `json:"recording,omitempty"`
	Streaming    *StreamingResponse  `json:"streaming,omitempty"`
}

// SpectatorResponse represents a spectator in API responses
type SpectatorResponse struct {
	UserID    int    `json:"user_id"`
	Username  string `json:"username"`
	JoinTime  string `json:"join_time"`
	BytesSent int64  `json:"bytes_sent"`
	IsActive  bool   `json:"is_active"`
}

// RecordingResponse represents recording info in API responses
type RecordingResponse struct {
	Enabled    bool   `json:"enabled"`
	FilePath   string `json:"file_path"`
	Format     string `json:"format"`
	StartTime  string `json:"start_time"`
	FileSize   int64  `json:"file_size"`
	Compressed bool   `json:"compressed"`
}

// StreamingResponse represents streaming info in API responses
type StreamingResponse struct {
	Enabled       bool   `json:"enabled"`
	Protocol      string `json:"protocol"`
	Encrypted     bool   `json:"encrypted"`
	FrameCount    uint64 `json:"frame_count"`
	BytesStreamed int64  `json:"bytes_streamed"`
}

// SaveResponse represents a save in API responses
type SaveResponse struct {
	ID        string                 `json:"id"`
	UserID    int                    `json:"user_id"`
	GameID    string                 `json:"game_id"`
	Status    string                 `json:"status"`
	FileSize  int64                  `json:"file_size"`
	Checksum  string                 `json:"checksum"`
	Metadata  map[string]interface{} `json:"metadata"`
	Backups   []BackupResponse       `json:"backups"`
	CreatedAt string                 `json:"created_at"`
	UpdatedAt string                 `json:"updated_at"`
}

// BackupResponse represents a backup in API responses
type BackupResponse struct {
	ID        string `json:"id"`
	FilePath  string `json:"file_path"`
	FileSize  int64  `json:"file_size"`
	Checksum  string `json:"checksum"`
	CreatedAt string `json:"created_at"`
}

// ErrorResponse represents an error in API responses
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// SuccessResponse represents a success response
type SuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}
