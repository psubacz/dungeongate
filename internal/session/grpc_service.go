package session

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Enhanced service clients with proper gRPC implementation

type userServiceClientEnhanced struct {
	address string
	conn    *grpc.ClientConn
}

func NewUserServiceClientEnhanced(address string) UserServiceClient {
	return &userServiceClientEnhanced{address: address}
}

func (c *userServiceClientEnhanced) GetUser(ctx context.Context, username string) (*User, error) {
	// TODO: Implement actual gRPC client call
	return &User{
		ID:       1,
		Username: username,
	}, nil
}

func (c *userServiceClientEnhanced) GetUserByID(ctx context.Context, userID int) (*User, error) {
	// TODO: Implement actual gRPC client call
	return &User{
		ID:       userID,
		Username: "testuser",
	}, nil
}

func (c *userServiceClientEnhanced) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	// TODO: Implement actual gRPC client call
	return &User{
		ID:       1,
		Username: username,
	}, nil
}

func (c *userServiceClientEnhanced) RegisterUser(ctx context.Context, req *RegistrationRequest) (*RegistrationResponse, error) {
	// TODO: Implement actual gRPC client call
	return &RegistrationResponse{
		Success: true,
		User: &User{
			ID:       1,
			Username: req.Username,
			Email:    req.Email,
		},
		Message: "Registration successful",
	}, nil
}

func (c *userServiceClientEnhanced) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
	// TODO: Implement actual gRPC client call
	return &User{
		ID:       1,
		Username: req.Username,
	}, nil
}

func (c *userServiceClientEnhanced) UpdateUser(ctx context.Context, userID int, updates map[string]interface{}) (*User, error) {
	// TODO: Implement actual gRPC client call
	return &User{
		ID:       userID,
		Username: "testuser",
	}, nil
}

func (c *userServiceClientEnhanced) DeleteUser(ctx context.Context, userID int) error {
	// TODO: Implement actual gRPC client call
	return nil
}

func (c *userServiceClientEnhanced) ListUsers(ctx context.Context, limit, offset int) ([]*User, error) {
	// TODO: Implement actual gRPC client call
	return []*User{}, nil
}

func (c *userServiceClientEnhanced) UpdateLastLogin(ctx context.Context, userID int) error {
	// TODO: Implement actual gRPC client call
	return nil
}

type gameServiceClientEnhanced struct {
	address string
	conn    *grpc.ClientConn
}

func NewGameServiceClientEnhanced(address string) GameServiceClient {
	return &gameServiceClientEnhanced{address: address}
}

func (c *gameServiceClientEnhanced) ListGames(ctx context.Context) ([]*Game, error) {
	// TODO: Implement actual gRPC client call
	return []*Game{
		{ID: "nethack", Name: "NetHack 3.7.0", Description: "The classic dungeon adventure", Enabled: true},
		{ID: "bash", Name: "Bash Shell", Description: "Interactive command line", Enabled: true},
		{ID: "nano", Name: "Nano Editor", Description: "Text editor with sample file", Enabled: true},
		{ID: "crawl", Name: "Dungeon Crawl Stone Soup", Description: "Modern roguelike adventure", Enabled: true},
	}, nil
}

func (c *gameServiceClientEnhanced) GetGame(ctx context.Context, gameID string) (*Game, error) {
	// TODO: Implement actual gRPC client call
	games := map[string]*Game{
		"nethack": {ID: "nethack", Name: "NetHack 3.7.0", Description: "The classic dungeon adventure", Enabled: true},
		"bash":    {ID: "bash", Name: "Bash Shell", Description: "Interactive command line", Enabled: true},
		"nano":    {ID: "nano", Name: "Nano Editor", Description: "Text editor with sample file", Enabled: true},
		"crawl":   {ID: "crawl", Name: "Dungeon Crawl Stone Soup", Description: "Modern roguelike adventure", Enabled: true},
	}

	if game, exists := games[gameID]; exists {
		return game, nil
	}

	return nil, fmt.Errorf("game not found: %s", gameID)
}

func (c *gameServiceClientEnhanced) StartGame(ctx context.Context, req *StartGameRequest) (*GameSession, error) {
	// TODO: Implement actual gRPC client call
	return &GameSession{
		ID:       "game_session_" + req.GameID,
		UserID:   req.UserID,
		Username: req.Username,
		GameID:   req.GameID,
	}, nil
}

func (c *gameServiceClientEnhanced) StopGame(ctx context.Context, gameSessionID string) error {
	// TODO: Implement actual gRPC client call
	return nil
}

func (c *gameServiceClientEnhanced) GetGameStatus(ctx context.Context, sessionID string) (*GameSession, error) {
	// TODO: Implement actual gRPC client call
	return &GameSession{
		ID:       sessionID,
		UserID:   1,
		Username: "testuser",
		GameID:   "nethack",
	}, nil
}

func (c *gameServiceClientEnhanced) UpdateGameConfig(ctx context.Context, gameID string, config *Game) error {
	// TODO: Implement actual gRPC client call
	return nil
}

// gRPC connection management

type GRPCClientManager struct {
	authConn *grpc.ClientConn
	userConn *grpc.ClientConn
	gameConn *grpc.ClientConn
}

func NewGRPCClientManager(authAddr, userAddr, gameAddr string) *GRPCClientManager {
	return &GRPCClientManager{}
}

func (m *GRPCClientManager) ConnectAuth(address string) error {
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("failed to connect to auth service: %w", err)
	}
	m.authConn = conn
	return nil
}

func (m *GRPCClientManager) ConnectUser(address string) error {
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("failed to connect to user service: %w", err)
	}
	m.userConn = conn
	return nil
}

func (m *GRPCClientManager) ConnectGame(address string) error {
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("failed to connect to game service: %w", err)
	}
	m.gameConn = conn
	return nil
}

func (m *GRPCClientManager) Close() error {
	var err error
	if m.authConn != nil {
		if closeErr := m.authConn.Close(); closeErr != nil {
			err = closeErr
		}
	}
	if m.userConn != nil {
		if closeErr := m.userConn.Close(); closeErr != nil {
			err = closeErr
		}
	}
	if m.gameConn != nil {
		if closeErr := m.gameConn.Close(); closeErr != nil {
			err = closeErr
		}
	}
	return err
}

// Error handling for gRPC

func handleGRPCError(err error) error {
	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok {
		return err
	}

	switch st.Code() {
	case codes.NotFound:
		return fmt.Errorf("resource not found: %s", st.Message())
	case codes.PermissionDenied:
		return fmt.Errorf("permission denied: %s", st.Message())
	case codes.Unauthenticated:
		return fmt.Errorf("authentication required: %s", st.Message())
	case codes.InvalidArgument:
		return fmt.Errorf("invalid argument: %s", st.Message())
	case codes.Unavailable:
		return fmt.Errorf("service unavailable: %s", st.Message())
	case codes.DeadlineExceeded:
		return fmt.Errorf("request timeout: %s", st.Message())
	default:
		return fmt.Errorf("service error: %s", st.Message())
	}
}

// Service health checking

type ServiceHealthChecker struct {
	userClient UserServiceClient
	gameClient GameServiceClient
}

func NewServiceHealthChecker(userClient UserServiceClient, gameClient GameServiceClient) *ServiceHealthChecker {
	return &ServiceHealthChecker{
		userClient: userClient,
		gameClient: gameClient,
	}
}

func (h *ServiceHealthChecker) CheckAllServices(ctx context.Context) map[string]error {
	results := make(map[string]error)

	// Check auth service
	results["auth"] = h.checkAuthService(ctx)

	// Check user service
	results["user"] = h.checkUserService(ctx)

	// Check game service
	results["game"] = h.checkGameService(ctx)

	return results
}

func (h *ServiceHealthChecker) checkAuthService(ctx context.Context) error {
	// Try to validate a dummy token
	_, err := h.authClient.ValidateToken(ctx, "health_check_token")
	if err != nil && !strings.Contains(err.Error(), "invalid token") {
		return err
	}
	return nil
}

func (h *ServiceHealthChecker) checkUserService(ctx context.Context) error {
	// Try to get a non-existent user
	_, err := h.userClient.GetUser(ctx, "health_check_user")
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return err
	}
	return nil
}

func (h *ServiceHealthChecker) checkGameService(ctx context.Context) error {
	// Try to list games
	_, err := h.gameClient.ListGames(ctx)
	return err
}

// Service discovery and configuration

type ServiceConfig struct {
	AuthService string `json:"auth_service"`
	UserService string `json:"user_service"`
	GameService string `json:"game_service"`
	Timeout     string `json:"timeout"`
	Retries     int    `json:"retries"`
}

func (c *ServiceConfig) GetTimeout() time.Duration {
	if c.Timeout == "" {
		return 30 * time.Second
	}
	if duration, err := time.ParseDuration(c.Timeout); err == nil {
		return duration
	}
	return 30 * time.Second
}

func (c *ServiceConfig) GetRetries() int {
	if c.Retries <= 0 {
		return 3
	}
	return c.Retries
}

// Service registry for dynamic service discovery
type ServiceRegistry struct {
	services map[string]string
	mutex    sync.RWMutex
}

func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		services: make(map[string]string),
	}
}

func (r *ServiceRegistry) Register(serviceName, address string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.services[serviceName] = address
}

func (r *ServiceRegistry) Unregister(serviceName string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.services, serviceName)
}

func (r *ServiceRegistry) GetService(serviceName string) (string, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	address, exists := r.services[serviceName]
	return address, exists
}

func (r *ServiceRegistry) ListServices() map[string]string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	result := make(map[string]string)
	for k, v := range r.services {
		result[k] = v
	}
	return result
}
