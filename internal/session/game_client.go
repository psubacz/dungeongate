package session

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/dungeongate/pkg/config"
	games_pb "github.com/dungeongate/pkg/api/games"
)

// GameServiceGRPCClient represents a gRPC client for the game service
type GameServiceGRPCClient struct {
	conn    *grpc.ClientConn
	client  games_pb.GameServiceClient
	address string
	timeout time.Duration
}

// NewGameServiceGRPCClient creates a new game service gRPC client
func NewGameServiceGRPCClient(address string) (*GameServiceGRPCClient, error) {
	// Set up connection options
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()), // TODO: Add TLS in production
	}

	// Connect to the game service
	conn, err := grpc.NewClient(address, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to game service at %s: %w", address, err)
	}

	// Create the protobuf client
	client := games_pb.NewGameServiceClient(conn)

	return &GameServiceGRPCClient{
		conn:    conn,
		client:  client,
		address: address,
		timeout: 30 * time.Second,
	}, nil
}

// Close closes the connection to the game service
func (c *GameServiceGRPCClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// StartGameGRPCRequest represents a gRPC request to start a game
type StartGameGRPCRequest struct {
	UserID          string            `json:"user_id"`
	Username        string            `json:"username"`
	GameID          string            `json:"game_id"`
	SessionID       string            `json:"session_id"`
	Environment     map[string]string `json:"environment"`
	EnableRecording bool              `json:"enable_recording"`
}

// StartGameResponse represents the response from starting a game
type StartGameResponse struct {
	SessionID   string `json:"session_id"`
	ContainerID string `json:"container_id"`
	PodName     string `json:"pod_name"`
	Success     bool   `json:"success"`
	Error       string `json:"error,omitempty"`
}

// StopGameRequest represents a request to stop a game
type StopGameRequest struct {
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
	Force     bool   `json:"force"`
	Reason    string `json:"reason"`
}

// StopGameResponse represents the response from stopping a game
type StopGameResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// GameSessionInfo represents information about a game session
type GameSessionInfo struct {
	SessionID     string            `json:"session_id"`
	UserID        string            `json:"user_id"`
	Username      string            `json:"username"`
	GameID        string            `json:"game_id"`
	Status        string            `json:"status"`
	StartTime     time.Time         `json:"start_time"`
	LastActivity  time.Time         `json:"last_activity"`
	ContainerID   string            `json:"container_id"`
	PodName       string            `json:"pod_name"`
	RecordingPath string            `json:"recording_path"`
	Spectators    []string          `json:"spectators"`
	Metadata      map[string]string `json:"metadata"`
}

// StartGame starts a new game session
func (c *GameServiceGRPCClient) StartGame(ctx context.Context, req *StartGameRequest) (*StartGameResponse, error) {
	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Convert session service request to game service protobuf request
	grpcReq := &games_pb.StartGameSessionRequest{
		UserId:   int32(req.UserID),
		Username: req.Username,
		GameId:   req.GameID,
		TerminalSize: &games_pb.TerminalSize{
			Width:  80,  // Default terminal width
			Height: 24,  // Default terminal height
		},
		EnableRecording:  false, // TODO: Make configurable
		EnableStreaming:  false, // TODO: Make configurable
		EnableEncryption: false, // TODO: Make configurable
	}

	// Call the game service
	grpcResp, err := c.client.StartGameSession(timeoutCtx, grpcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to start game session: %w", err)
	}

	// Convert protobuf response to session service response
	response := &StartGameResponse{
		SessionID:   grpcResp.Session.Id,
		ContainerID: grpcResp.Session.ProcessInfo.ContainerId,
		PodName:     grpcResp.Session.ProcessInfo.PodName,
		Success:     true,
	}

	return response, nil
}

// StopGame stops a game session
func (c *GameServiceGRPCClient) StopGame(ctx context.Context, req *StopGameRequest) (*StopGameResponse, error) {
	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Convert session service request to game service protobuf request
	grpcReq := &games_pb.StopGameSessionRequest{
		SessionId: req.SessionID,
		Reason:    req.Reason,
		Force:     req.Force,
	}

	// Call the game service
	grpcResp, err := c.client.StopGameSession(timeoutCtx, grpcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to stop game session: %w", err)
	}

	// Convert protobuf response to session service response
	response := &StopGameResponse{
		Success: grpcResp.Success,
	}

	return response, nil
}

// ListGames lists available games
func (c *GameServiceGRPCClient) ListGames(ctx context.Context) ([]*Game, error) {
	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Create the protobuf request
	grpcReq := &games_pb.ListGamesRequest{
		EnabledOnly: true, // Only list enabled games
		Limit:       100,  // Default limit
		Offset:      0,
	}

	// Call the game service
	grpcResp, err := c.client.ListGames(timeoutCtx, grpcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to list games: %w", err)
	}

	// Convert protobuf response to session service response
	games := make([]*Game, len(grpcResp.Games))
	for i, game := range grpcResp.Games {
		games[i] = &Game{
			ID:          game.Id,
			Name:        game.Name,
			ShortName:   game.ShortName,
			Description: game.Description,
			Enabled:     game.Status == games_pb.GameStatus_GAME_STATUS_ENABLED,
			Binary:      game.Binary.Path,
			Args:        game.Binary.Args,
			WorkingDir:  game.Binary.WorkingDirectory,
			Environment: game.Environment,
			MaxPlayers:  int(game.Statistics.ActiveSessions), // Using active sessions as placeholder
			Spectatable: true,                                 // Default to true
			CreatedAt:   game.CreatedAt.AsTime(),
			UpdatedAt:   game.UpdatedAt.AsTime(),
		}
	}

	return games, nil
}

// GetGameSession gets information about a game session
func (c *GameServiceGRPCClient) GetGameSession(ctx context.Context, sessionID string) (*GameSessionInfo, error) {
	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Create the protobuf request
	grpcReq := &games_pb.GetGameSessionRequest{
		SessionId: sessionID,
	}

	// Call the game service
	grpcResp, err := c.client.GetGameSession(timeoutCtx, grpcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get game session: %w", err)
	}

	session := grpcResp.Session

	// Convert protobuf response to session service response
	info := &GameSessionInfo{
		SessionID:     session.Id,
		UserID:        fmt.Sprintf("%d", session.UserId),
		Username:      session.Username,
		GameID:        session.GameId,
		Status:        session.Status.String(),
		StartTime:     session.StartTime.AsTime(),
		LastActivity:  session.LastActivity.AsTime(),
		ContainerID:   session.ProcessInfo.ContainerId,
		PodName:       session.ProcessInfo.PodName,
		RecordingPath: session.Recording.FilePath,
		Spectators:    extractSpectatorUsernames(session.Spectators),
		Metadata:      make(map[string]string),
	}

	return info, nil
}

// extractSpectatorUsernames extracts usernames from spectator info
func extractSpectatorUsernames(spectators []*games_pb.SpectatorInfo) []string {
	usernames := make([]string, len(spectators))
	for i, spectator := range spectators {
		usernames[i] = spectator.Username
	}
	return usernames
}

// ListActiveGames lists all active game sessions
func (c *GameServiceGRPCClient) ListActiveGames(ctx context.Context, userID string) ([]*GameSessionInfo, error) {
	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Convert userID string to int32 (assuming it's a valid integer string)
	var userIDInt int32
	if userID != "" {
		if _, err := fmt.Sscanf(userID, "%d", &userIDInt); err != nil {
			userIDInt = 0 // Use 0 to list all sessions if parsing fails
		}
	}

	// Create the protobuf request
	grpcReq := &games_pb.ListGameSessionsRequest{
		UserId: userIDInt,
		Status: games_pb.SessionStatus_SESSION_STATUS_ACTIVE,
		Limit:  100, // Default limit
		Offset: 0,
	}

	// Call the game service
	grpcResp, err := c.client.ListGameSessions(timeoutCtx, grpcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to list game sessions: %w", err)
	}

	// Convert protobuf response to session service response
	sessions := make([]*GameSessionInfo, len(grpcResp.Sessions))
	for i, session := range grpcResp.Sessions {
		sessions[i] = &GameSessionInfo{
			SessionID:     session.Id,
			UserID:        fmt.Sprintf("%d", session.UserId),
			Username:      session.Username,
			GameID:        session.GameId,
			Status:        session.Status.String(),
			StartTime:     session.StartTime.AsTime(),
			LastActivity:  session.LastActivity.AsTime(),
			ContainerID:   session.ProcessInfo.ContainerId,
			PodName:       session.ProcessInfo.PodName,
			RecordingPath: session.Recording.FilePath,
			Spectators:    extractSpectatorUsernames(session.Spectators),
			Metadata:      make(map[string]string),
		}
	}

	return sessions, nil
}

// HealthCheck performs a health check on the game service
func (c *GameServiceGRPCClient) HealthCheck(ctx context.Context) (bool, error) {
	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Call the health check endpoint
	response, err := c.client.Health(timeoutCtx, &emptypb.Empty{})
	if err != nil {
		return false, fmt.Errorf("health check failed: %w", err)
	}

	return response.Status == "healthy", nil
}

// IsAvailable checks if the game service is available
func (c *GameServiceGRPCClient) IsAvailable() bool {
	if c.conn == nil {
		return false
	}

	state := c.conn.GetState()
	return state.String() == "READY"
}

// Reconnect attempts to reconnect to the game service
func (c *GameServiceGRPCClient) Reconnect() error {
	if c.conn != nil {
		c.conn.Close()
	}

	// Set up connection options
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()), // TODO: Add TLS in production
	}

	// Connect to the game service
	conn, err := grpc.NewClient(c.address, opts...)
	if err != nil {
		return fmt.Errorf("failed to reconnect to game service at %s: %w", c.address, err)
	}

	c.conn = conn
	c.client = games_pb.NewGameServiceClient(conn)
	return nil
}

// GameServiceManager manages the connection to the game service
type GameServiceManager struct {
	client      *GameServiceGRPCClient
	config      *config.SessionServiceConfig
	isConnected bool
}

// NewGameServiceManager creates a new game service manager
func NewGameServiceManager(cfg *config.SessionServiceConfig) *GameServiceManager {
	return &GameServiceManager{
		config:      cfg,
		isConnected: false,
	}
}

// Connect connects to the game service
func (m *GameServiceManager) Connect() error {
	if m.client != nil {
		m.client.Close()
	}

	client, err := NewGameServiceGRPCClient(m.config.GetServices().GameService)
	if err != nil {
		m.isConnected = false
		return fmt.Errorf("failed to connect to game service: %w", err)
	}

	m.client = client
	m.isConnected = true
	return nil
}

// Disconnect disconnects from the game service
func (m *GameServiceManager) Disconnect() error {
	if m.client != nil {
		err := m.client.Close()
		m.client = nil
		m.isConnected = false
		return err
	}
	return nil
}

// GetClient returns the game service client
func (m *GameServiceManager) GetClient() *GameServiceGRPCClient {
	return m.client
}

// IsConnected returns whether we're connected to the game service
func (m *GameServiceManager) IsConnected() bool {
	return m.isConnected && m.client != nil && m.client.IsAvailable()
}

// EnsureConnected ensures we have a connection to the game service
func (m *GameServiceManager) EnsureConnected() error {
	if !m.IsConnected() {
		return m.Connect()
	}
	return nil
}

// StartGameWithRetry starts a game with retry logic
func (m *GameServiceManager) StartGameWithRetry(ctx context.Context, req *StartGameRequest, maxRetries int) (*StartGameResponse, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Ensure we're connected
		if err := m.EnsureConnected(); err != nil {
			lastErr = err
			continue
		}

		// Try to start the game
		response, err := m.client.StartGame(ctx, req)
		if err == nil {
			return response, nil
		}

		// Check if it's a connection error
		if st, ok := status.FromError(err); ok {
			if st.Code() == codes.Unavailable || st.Code() == codes.DeadlineExceeded {
				// Connection error, try to reconnect
				lastErr = err
				m.isConnected = false
				continue
			}
		}

		// Other errors are not retryable
		return nil, err
	}

	return nil, fmt.Errorf("failed to start game after %d attempts: %w", maxRetries, lastErr)
}

// StopGameWithRetry stops a game with retry logic
func (m *GameServiceManager) StopGameWithRetry(ctx context.Context, req *StopGameRequest, maxRetries int) (*StopGameResponse, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Ensure we're connected
		if err := m.EnsureConnected(); err != nil {
			lastErr = err
			continue
		}

		// Try to stop the game
		response, err := m.client.StopGame(ctx, req)
		if err == nil {
			return response, nil
		}

		// Check if it's a connection error
		if st, ok := status.FromError(err); ok {
			if st.Code() == codes.Unavailable || st.Code() == codes.DeadlineExceeded {
				// Connection error, try to reconnect
				lastErr = err
				m.isConnected = false
				continue
			}
		}

		// Other errors are not retryable
		return nil, err
	}

	return nil, fmt.Errorf("failed to stop game after %d attempts: %w", maxRetries, lastErr)
}
