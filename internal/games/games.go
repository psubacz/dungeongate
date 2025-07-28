package games

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/dungeongate/pkg/config"
	"github.com/dungeongate/pkg/database"
	"google.golang.org/grpc"
)

// GameEvent represents a game event for streaming
type GameEvent struct {
	EventID   string            `json:"event_id"`
	SessionID string            `json:"session_id"`
	EventType string            `json:"event_type"`
	EventData []byte            `json:"event_data"`
	Metadata  map[string]string `json:"metadata"`
	Timestamp time.Time         `json:"timestamp"`
}

// Service handles game management operations
type Service struct {
	db             *database.Connection
	config         *config.GameServiceConfig
	activeSessions map[string]*GameSession
	sessionMutex   sync.RWMutex
	eventStreams   map[string][]chan *GameEvent
	streamMutex    sync.RWMutex
}

// NewService creates a new game service
func NewService(db *database.Connection, cfg *config.GameServiceConfig) *Service {
	return &Service{
		db:             db,
		config:         cfg,
		activeSessions: make(map[string]*GameSession),
		eventStreams:   make(map[string][]chan *GameEvent),
	}
}

// Game represents a game instance
type Game struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	ShortName   string                  `json:"short_name"`
	Enabled     bool                    `json:"enabled"`
	Binary      *config.BinaryConfig    `json:"binary"`
	Files       *config.FilesConfig     `json:"files"`
	Settings    *config.GameSettings    `json:"settings"`
	Environment map[string]string       `json:"environment"`
	Resources   *config.ResourcesConfig `json:"resources"`
	Container   *config.ContainerConfig `json:"container"`
}

// GameSession represents an active game session
type GameSession struct {
	ID          string    `json:"id"`
	UserID      int       `json:"user_id"`
	Username    string    `json:"username"`
	GameID      string    `json:"game_id"`
	PID         int       `json:"pid"`
	StartTime   time.Time `json:"start_time"`
	IsActive    bool      `json:"is_active"`
	ContainerID string    `json:"container_id"`
	PodName     string    `json:"pod_name"`
}

// StartGameRequest represents a request to start a game
type StartGameRequest struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	GameID   string `json:"game_id"`
}

// StartGame starts a new game session
func (s *Service) StartGame(ctx context.Context, req *StartGameRequest) (*GameSession, error) {
	// Find game configuration
	game, err := s.getGameConfig(req.GameID)
	if err != nil {
		return nil, fmt.Errorf("failed to get game config: %w", err)
	}

	if !game.Enabled {
		return nil, fmt.Errorf("game %s is not enabled", req.GameID)
	}

	// Create game session
	session := &GameSession{
		ID:       generateSessionID(),
		UserID:   req.UserID,
		Username: req.Username,
		GameID:   req.GameID,
		IsActive: true,
	}

	// Start game process using namespaces instead of chroot
	cmd, err := s.startGameProcess(session, game)
	if err != nil {
		return nil, fmt.Errorf("failed to start game process: %w", err)
	}

	session.PID = cmd.Process.Pid

	// TODO: Store session in database
	// TODO: Set up TTY recording
	// TODO: Set up process monitoring

	return session, nil
}

// StopGame stops a game session
func (s *Service) StopGame(ctx context.Context, sessionID string) error {
	// TODO: Implement game stopping logic
	return fmt.Errorf("game stopping not implemented")
}

// GetActiveGames returns all active game sessions
func (s *Service) GetActiveGames(ctx context.Context) ([]*GameSession, error) {
	// TODO: Implement active games retrieval
	return nil, fmt.Errorf("active games retrieval not implemented")
}

// GetGameConfig returns game configuration
func (s *Service) GetGameConfig(ctx context.Context, gameID string) (*Game, error) {
	return s.getGameConfig(gameID)
}

// getGameConfig retrieves game configuration
func (s *Service) getGameConfig(gameID string) (*Game, error) {
	// Find game in configuration
	for _, gameConfig := range s.config.Games {
		if gameConfig.ID == gameID {
			return &Game{
				ID:          gameConfig.ID,
				Name:        gameConfig.Name,
				ShortName:   gameConfig.ShortName,
				Enabled:     gameConfig.Enabled,
				Binary:      gameConfig.Binary,
				Files:       gameConfig.Files,
				Settings:    gameConfig.Settings,
				Environment: gameConfig.Environment,
				Resources:   gameConfig.Resources,
			}, nil
		}
	}

	return nil, fmt.Errorf("game %s not found", gameID)
}

// startGameProcess starts a game process with namespace isolation
func (s *Service) startGameProcess(session *GameSession, game *Game) (*exec.Cmd, error) {
	// Create game process command
	cmd := exec.Command(game.Binary.Path, game.Binary.Args...)

	// Set up process attributes for isolation
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create new session
	}

	// Set working directory
	if game.Binary.WorkingDirectory != "" {
		cmd.Dir = game.Binary.WorkingDirectory
	}

	// Set environment variables
	cmd.Env = os.Environ()
	for key, value := range game.Environment {
		// Replace placeholders
		expandedValue := expandPlaceholders(value, session.Username, game.ID)
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, expandedValue))
	}

	// Set up user and group
	if s.config.GameEngine.Chroot != nil {
		// Drop privileges - use configured user/group
		cmd.SysProcAttr.Credential = &syscall.Credential{
			Uid: 1000, // game user
			Gid: 1000, // game group
		}
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start game process: %w", err)
	}

	return cmd, nil
}

// expandPlaceholders expands placeholders in strings
func expandPlaceholders(template, username, gameID string) string {
	// TODO: Implement proper placeholder expansion
	// For now, just return the template
	return template
}

// generateSessionID generates a unique session ID
func generateSessionID() string {
	// TODO: Implement proper session ID generation
	return "session_123"
}

// InitializeNamespaces initializes namespace isolation
func InitializeNamespaces(config *config.ChrootConfig) error {
	// TODO: Implement namespace initialization
	// This replaces traditional chroot with Linux namespaces for better isolation
	return nil
}

// NewHTTPHandler creates a new HTTP handler for the game service
func NewHTTPHandler(service *Service) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	mux.HandleFunc("/games", func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement game endpoints
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte("Game endpoints not implemented"))
	})

	mux.HandleFunc("/sessions", func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement session endpoints
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte("Session endpoints not implemented"))
	})

	return mux
}

// RegisterGameServiceServer registers the gRPC service
func RegisterGameServiceServer(server *grpc.Server, service *Service) {
	// Register the gRPC service implementation
	// This will be implemented when we generate the gRPC code from proto
}

// gRPC Service Implementation Methods

// StartGameGRPC starts a new game session via gRPC
func (s *Service) StartGameGRPC(ctx context.Context, req *StartGameRequestGRPC) (*StartGameResponseGRPC, error) {
	// Convert gRPC request to internal request
	startReq := &StartGameRequest{
		UserID:   1, // Convert from string to int if needed
		Username: req.Username,
		GameID:   req.GameID,
	}

	session, err := s.StartGame(ctx, startReq)
	if err != nil {
		return &StartGameResponseGRPC{
			Error: err.Error(),
		}, nil
	}

	return &StartGameResponseGRPC{
		Session: session,
	}, nil
}

// StopGameGRPC stops a game session via gRPC
func (s *Service) StopGameGRPC(ctx context.Context, req *StopGameRequestGRPC) (*StopGameResponseGRPC, error) {
	err := s.StopGame(ctx, req.SessionID)
	if err != nil {
		return &StopGameResponseGRPC{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &StopGameResponseGRPC{
		Success: true,
	}, nil
}

// ListActiveSessionsGRPC lists active sessions via gRPC
func (s *Service) ListActiveSessionsGRPC(ctx context.Context, req *ListActiveSessionsRequestGRPC) (*ListActiveSessionsResponseGRPC, error) {
	sessions, err := s.GetActiveGames(ctx)
	if err != nil {
		return &ListActiveSessionsResponseGRPC{
			Error: err.Error(),
		}, nil
	}

	return &ListActiveSessionsResponseGRPC{
		Sessions:   sessions,
		TotalCount: int32(len(sessions)),
	}, nil
}

// GetGameSessionGRPC gets a game session via gRPC
func (s *Service) GetGameSessionGRPC(ctx context.Context, req *GetGameSessionRequestGRPC) (*GetGameSessionResponseGRPC, error) {
	s.sessionMutex.RLock()
	session, exists := s.activeSessions[req.SessionID]
	s.sessionMutex.RUnlock()

	if !exists {
		return &GetGameSessionResponseGRPC{
			Error: "session not found",
		}, nil
	}

	return &GetGameSessionResponseGRPC{
		Session: session,
	}, nil
}

// HealthGRPC performs health check via gRPC
func (s *Service) HealthGRPC(ctx context.Context, req *HealthRequestGRPC) (*HealthResponseGRPC, error) {
	return &HealthResponseGRPC{
		Status: "healthy",
		Details: map[string]string{
			"active_sessions": fmt.Sprintf("%d", len(s.activeSessions)),
			"timestamp":       time.Now().Format(time.RFC3339),
		},
	}, nil
}

// Container Management Methods

// StartGameContainer starts a game in a container
func (s *Service) StartGameContainer(ctx context.Context, session *GameSession, game *Game) error {
	// Implementation depends on container runtime (Docker, Podman, containerd)
	switch s.config.GameEngine.ContainerRuntime.Runtime {
	case "docker":
		return s.startDockerContainer(ctx, session, game)
	case "podman":
		return s.startPodmanContainer(ctx, session, game)
	case "kubernetes":
		return s.startKubernetesPod(ctx, session, game)
	default:
		return fmt.Errorf("unsupported container runtime: %s", s.config.GameEngine.ContainerRuntime.Runtime)
	}
}

// startDockerContainer starts a Docker container for the game
func (s *Service) startDockerContainer(ctx context.Context, session *GameSession, game *Game) error {
	// Build Docker command
	args := []string{
		"run", "-d",
		"--name", fmt.Sprintf("dungeongate-%s-%s", game.ID, session.ID),
		"--rm", // Remove container when it stops
	}

	// Add resource limits
	if game.Resources != nil {
		if game.Resources.CPULimit != "" {
			args = append(args, "--cpus", game.Resources.CPULimit)
		}
		if game.Resources.MemoryLimit != "" {
			args = append(args, "--memory", game.Resources.MemoryLimit)
		}
	}

	// Add security context
	if game.Container != nil && game.Container.SecurityContext != nil {
		if game.Container.SecurityContext.RunAsUser > 0 {
			args = append(args, "--user", fmt.Sprintf("%d:%d",
				game.Container.SecurityContext.RunAsUser,
				game.Container.SecurityContext.RunAsGroup))
		}
		if game.Container.SecurityContext.ReadOnlyRootFilesystem {
			args = append(args, "--read-only")
		}
	}

	// Add volumes
	if game.Container != nil {
		for _, volume := range game.Container.Volumes {
			volumeArg := fmt.Sprintf("%s:%s", volume.HostPath, volume.MountPath)
			if volume.ReadOnly {
				volumeArg += ":ro"
			}
			args = append(args, "-v", volumeArg)
		}
	}

	// Add environment variables
	if game.Environment != nil {
		for key, value := range game.Environment {
			expandedValue := expandPlaceholders(value, session.Username, game.ID)
			args = append(args, "-e", fmt.Sprintf("%s=%s", key, expandedValue))
		}
	}

	// Add container image
	if game.Container != nil {
		image := game.Container.Image
		if game.Container.Tag != "" {
			image += ":" + game.Container.Tag
		}
		args = append(args, image)
	}

	// Add binary and arguments
	args = append(args, game.Binary.Path)
	args = append(args, game.Binary.Args...)

	// Execute Docker command
	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start Docker container: %w, output: %s", err, string(output))
	}

	// Store container ID
	session.ContainerID = string(output[:len(output)-1]) // Remove trailing newline

	return nil
}

// startPodmanContainer starts a Podman container for the game
func (s *Service) startPodmanContainer(ctx context.Context, session *GameSession, game *Game) error {
	// Similar to Docker but using Podman
	// Implementation would be similar to startDockerContainer but with Podman-specific options
	return fmt.Errorf("Podman container runtime not yet implemented")
}

// startKubernetesPod starts a Kubernetes pod for the game
func (s *Service) startKubernetesPod(ctx context.Context, session *GameSession, game *Game) error {
	// Implementation for Kubernetes pod creation
	// This would use the Kubernetes client to create a pod
	return fmt.Errorf("Kubernetes pod runtime not yet implemented")
}

// Event streaming methods

// PublishGameEvent publishes an event to all listening streams
func (s *Service) PublishGameEvent(event *GameEvent) {
	s.streamMutex.RLock()
	defer s.streamMutex.RUnlock()

	if streams, exists := s.eventStreams[event.SessionID]; exists {
		for _, stream := range streams {
			select {
			case stream <- event:
				// Event sent successfully
			default:
				// Stream buffer full, skip this event
			}
		}
	}
}

// AddEventStream adds an event stream for a session
func (s *Service) AddEventStream(sessionID string, stream chan *GameEvent) {
	s.streamMutex.Lock()
	defer s.streamMutex.Unlock()

	if s.eventStreams[sessionID] == nil {
		s.eventStreams[sessionID] = make([]chan *GameEvent, 0)
	}
	s.eventStreams[sessionID] = append(s.eventStreams[sessionID], stream)
}

// RemoveEventStream removes an event stream for a session
func (s *Service) RemoveEventStream(sessionID string, stream chan *GameEvent) {
	s.streamMutex.Lock()
	defer s.streamMutex.Unlock()

	if streams, exists := s.eventStreams[sessionID]; exists {
		for i, existingStream := range streams {
			if existingStream == stream {
				// Remove stream from slice
				s.eventStreams[sessionID] = append(streams[:i], streams[i+1:]...)
				close(stream)
				break
			}
		}
	}
}

// gRPC type definitions for compilation (will be replaced by generated proto code)
type StartGameRequestGRPC struct {
	UserID   string
	Username string
	GameID   string
}

type StartGameResponseGRPC struct {
	Session *GameSession
	Error   string
}

type StopGameRequestGRPC struct {
	SessionID string
}

type StopGameResponseGRPC struct {
	Success bool
	Error   string
}

type GetGameSessionRequestGRPC struct {
	SessionID string
}

type GetGameSessionResponseGRPC struct {
	Session *GameSession
	Error   string
}

type ListActiveSessionsRequestGRPC struct {
	UserID string
	GameID string
	Limit  int32
	Offset int32
}

type ListActiveSessionsResponseGRPC struct {
	Sessions   []*GameSession
	TotalCount int32
	Error      string
}

type HealthRequestGRPC struct{}

type HealthResponseGRPC struct {
	Status  string
	Details map[string]string
}
