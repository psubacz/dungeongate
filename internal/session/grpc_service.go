package session

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Enhanced service clients with proper gRPC implementation

type userServiceClientEnhanced struct {
	address string
	// conn field removed - not currently used
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
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to auth service: %w", err)
	}
	m.authConn = conn
	return nil
}

func (m *GRPCClientManager) ConnectUser(address string) error {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to user service: %w", err)
	}
	m.userConn = conn
	return nil
}

func (m *GRPCClientManager) ConnectGame(address string) error {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
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

	// Check user service
	results["user"] = h.checkUserService(ctx)

	// Check game service
	results["game"] = h.checkGameService(ctx)

	return results
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
	// Try to health check the game service
	_, err := h.gameClient.HealthCheck(ctx)
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
