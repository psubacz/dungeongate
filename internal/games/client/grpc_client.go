package client

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/dungeongate/pkg/config"
)

// GameServiceGRPCClient represents a gRPC client for the game service
type GameServiceGRPCClient struct {
	conn    *grpc.ClientConn
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

	return &GameServiceGRPCClient{
		conn:    conn,
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

// StartGame starts a new game session
func (c *GameServiceGRPCClient) StartGame(ctx context.Context, req *StartGameRequest) (*StartGameResponse, error) {
	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	_ = timeoutCtx // TODO: Use this context when implementing gRPC call

	// This would normally call the gRPC service
	// For now, we'll simulate the response

	// TODO: Replace with actual gRPC call when protobuf is generated
	// client := games.NewGameServiceClient(c.conn)
	// response, err := client.StartGame(ctx, &games.StartGameRequest{...})

	// Generate a session ID since the existing StartGameRequest doesn't have one
	sessionID := fmt.Sprintf("session_%d_%s", req.UserID, req.GameID)

	// Simulate game service response
	response := &StartGameResponse{
		SessionID:   sessionID,
		ContainerID: fmt.Sprintf("container_%s", sessionID),
		PodName:     fmt.Sprintf("nethack-%s", sessionID),
		Success:     true,
	}

	return response, nil
}

// StopGame stops a game session
func (c *GameServiceGRPCClient) StopGame(ctx context.Context, req *StopGameRequest) (*StopGameResponse, error) {
	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	_ = timeoutCtx // TODO: Use this context when implementing gRPC call

	// TODO: Replace with actual gRPC call when protobuf is generated
	// client := games.NewGameServiceClient(c.conn)
	// response, err := client.StopGame(ctx, &games.StopGameRequest{...})

	// Simulate game service response
	response := &StopGameResponse{
		Success: true,
	}

	return response, nil
}

// GetGameSession gets information about a game session
func (c *GameServiceGRPCClient) GetGameSession(ctx context.Context, sessionID string) (*GameSessionInfo, error) {
	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	_ = timeoutCtx // TODO: Use this context when implementing gRPC call

	// TODO: Replace with actual gRPC call when protobuf is generated
	// client := games.NewGameServiceClient(c.conn)
	// response, err := client.GetGameSession(ctx, &games.GetGameSessionRequest{...})

	// Simulate game service response
	info := &GameSessionInfo{
		SessionID:    sessionID,
		Status:       "running",
		StartTime:    time.Now().Add(-time.Hour),
		LastActivity: time.Now(),
		ContainerID:  fmt.Sprintf("container_%s", sessionID),
		PodName:      fmt.Sprintf("nethack-%s", sessionID),
		Spectators:   []string{},
		Metadata:     make(map[string]string),
	}

	return info, nil
}

// ListActiveGames lists all active game sessions
func (c *GameServiceGRPCClient) ListActiveGames(ctx context.Context, userID string) ([]*GameSessionInfo, error) {
	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	_ = timeoutCtx // TODO: Use this context when implementing gRPC call

	// TODO: Replace with actual gRPC call when protobuf is generated
	// client := games.NewGameServiceClient(c.conn)
	// response, err := client.ListActiveSessions(ctx, &games.ListActiveSessionsRequest{...})

	// Simulate game service response
	sessions := []*GameSessionInfo{
		{
			SessionID:    "session_123",
			UserID:       userID,
			Username:     "testuser",
			GameID:       "nethack",
			Status:       "running",
			StartTime:    time.Now().Add(-time.Hour),
			LastActivity: time.Now(),
			ContainerID:  "container_session_123",
			PodName:      "nethack-session_123",
			Spectators:   []string{},
			Metadata:     make(map[string]string),
		},
	}

	return sessions, nil
}

// HealthCheck performs a health check on the game service
func (c *GameServiceGRPCClient) HealthCheck(ctx context.Context) (bool, error) {
	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_ = timeoutCtx // TODO: Use this context when implementing gRPC call

	// TODO: Replace with actual gRPC call when protobuf is generated
	// client := games.NewGameServiceClient(c.conn)
	// response, err := client.Health(ctx, &empty.Empty{})

	// For now, just check if connection is alive
	state := c.conn.GetState()
	return state.String() == "READY", nil
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
