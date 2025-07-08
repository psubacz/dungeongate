package session

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/dungeongate/internal/games/client"
	"github.com/dungeongate/pkg/config"
)

// Service client implementations

// authServiceClient implements AuthServiceClient
type authServiceClient struct {
	address string
}

// NewAuthServiceClient creates a new auth service client
func NewAuthServiceClient(address string) AuthServiceClient {
	return &authServiceClient{
		address: address,
	}
}

func (c *authServiceClient) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	log.Printf("Auth service login request: %s", req.Username)

	// Mock implementation
	switch req.Username {
	case "admin":
		if req.Password == "admin" {
			return &LoginResponse{
				Success: true,
				Token:   "mock-token-admin",
				User: &User{
					ID:              1,
					Username:        req.Username,
					Email:           "admin@example.com",
					IsAuthenticated: true,
					IsActive:        true,
					IsAdmin:         true,
					CreatedAt:       time.Now(),
					UpdatedAt:       time.Now(),
				},
				Message: "Login successful",
			}, nil
		}
		return &LoginResponse{
			Success: false,
			Message: "Invalid password",
		}, nil
	case "user":
		if req.Password == "password" {
			return &LoginResponse{
				Success: true,
				Token:   "mock-token-user",
				User: &User{
					ID:              2,
					Username:        req.Username,
					Email:           "user@example.com",
					IsAuthenticated: true,
					IsActive:        true,
					IsAdmin:         false,
					CreatedAt:       time.Now(),
					UpdatedAt:       time.Now(),
				},
				Message: "Login successful",
			}, nil
		}
		return &LoginResponse{
			Success: false,
			Message: "Invalid password",
		}, nil
	default:
		return &LoginResponse{
			Success: false,
			Message: "User not found",
		}, nil
	}
}

func (c *authServiceClient) Logout(ctx context.Context, token string) error {
	log.Printf("Auth service logout request: %s", token)
	return nil
}

func (c *authServiceClient) ValidateToken(ctx context.Context, token string) (*User, error) {
	log.Printf("Auth service validate token request: %s", token)

	// Mock implementation
	switch token {
	case "mock-token-admin":
		return &User{
			ID:              1,
			Username:        "admin",
			Email:           "admin@example.com",
			IsAuthenticated: true,
			IsActive:        true,
			IsAdmin:         true,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}, nil
	case "mock-token-user":
		return &User{
			ID:              2,
			Username:        "user",
			Email:           "user@example.com",
			IsAuthenticated: true,
			IsActive:        true,
			IsAdmin:         false,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}, nil
	default:
		return nil, fmt.Errorf("invalid token")
	}
}

func (c *authServiceClient) RefreshToken(ctx context.Context, refreshToken string) (*LoginResponse, error) {
	log.Printf("Auth service refresh token request: %s", refreshToken)
	return nil, fmt.Errorf("not implemented")
}

// userServiceClient implements UserServiceClient
type userServiceClient struct {
	address string
}

// NewUserServiceClient creates a new user service client
func NewUserServiceClient(address string) UserServiceClient {
	return &userServiceClient{
		address: address,
	}
}

func (c *userServiceClient) GetUser(ctx context.Context, username string) (*User, error) {
	log.Printf("User service get user request: %s", username)

	// Mock implementation
	switch username {
	case "admin":
		return &User{
			ID:              1,
			Username:        username,
			Email:           "admin@example.com",
			IsAuthenticated: false,
			IsActive:        true,
			IsAdmin:         true,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}, nil
	case "user":
		return &User{
			ID:              2,
			Username:        username,
			Email:           "user@example.com",
			IsAuthenticated: false,
			IsActive:        true,
			IsAdmin:         false,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}, nil
	default:
		return nil, fmt.Errorf("user not found")
	}
}

func (c *userServiceClient) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	log.Printf("User service get user by username request: %s", username)

	// Mock implementation
	switch username {
	case "admin":
		return &User{
			ID:              1,
			Username:        username,
			Email:           "admin@example.com",
			IsAuthenticated: false,
			IsActive:        true,
			IsAdmin:         true,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}, nil
	case "user":
		return &User{
			ID:              2,
			Username:        username,
			Email:           "user@example.com",
			IsAuthenticated: false,
			IsActive:        true,
			IsAdmin:         false,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}, nil
	default:
		return nil, fmt.Errorf("user not found")
	}
}

func (c *userServiceClient) RegisterUser(ctx context.Context, req *RegistrationRequest) (*RegistrationResponse, error) {
	log.Printf("User service register user request: %s", req.Username)

	// Mock implementation - in real implementation would validate and create user
	return &RegistrationResponse{
		Success: true,
		User: &User{
			ID:              int(time.Now().Unix()),
			Username:        req.Username,
			Email:           req.Email,
			IsAuthenticated: false,
			IsActive:        true,
			IsAdmin:         false,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
		Message:              "Registration successful",
		RequiresVerification: false,
	}, nil
}

func (c *userServiceClient) GetUserByID(ctx context.Context, userID int) (*User, error) {
	log.Printf("User service get user by ID request: %d", userID)
	return nil, fmt.Errorf("not implemented")
}

func (c *userServiceClient) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
	log.Printf("User service create user request: %s", req.Username)

	// Mock implementation - in real implementation would validate and create user
	return &User{
		ID:              int(time.Now().Unix()),
		Username:        req.Username,
		Email:           req.Email,
		IsAuthenticated: false,
		IsActive:        true,
		IsAdmin:         false,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}, nil
}

func (c *userServiceClient) UpdateUser(ctx context.Context, userID int, updates map[string]interface{}) (*User, error) {
	log.Printf("User service update user request: %d", userID)
	return nil, fmt.Errorf("not implemented")
}

func (c *userServiceClient) DeleteUser(ctx context.Context, userID int) error {
	log.Printf("User service delete user request: %d", userID)
	return fmt.Errorf("not implemented")
}

func (c *userServiceClient) ListUsers(ctx context.Context, limit, offset int) ([]*User, error) {
	log.Printf("User service list users request: limit=%d, offset=%d", limit, offset)
	return nil, fmt.Errorf("not implemented")
}

func (c *userServiceClient) UpdateLastLogin(ctx context.Context, userID int) error {
	log.Printf("User service update last login request: %d", userID)
	return nil
}

// gameClientAdapter adapts the real game client to the session service interface
type gameClientAdapter struct {
	realClient *client.GameServiceGRPCClient
}

// Game service client factory function using the real client from games/client
func NewGameServiceClient(address string) GameServiceClient {
	realClient, err := client.NewGameServiceGRPCClient(address)
	if err != nil {
		log.Printf("Failed to create game service client: %v", err)
		return nil
	}
	
	return &gameClientAdapter{
		realClient: realClient,
	}
}

// NewGameServiceClientWithConfig is deprecated - configuration should be handled by the game service itself
func NewGameServiceClientWithConfig(address string, games []*config.GameConfig) GameServiceClient {
	// Configuration should be handled by the game service, not the session service
	log.Printf("Warning: NewGameServiceClientWithConfig is deprecated, ignoring games config")
	return NewGameServiceClient(address)
}

// Adapter methods that implement the GameServiceClient interface

func (a *gameClientAdapter) StartGame(ctx context.Context, req *StartGameRequest) (*StartGameResponse, error) {
	// Convert session service request to game client request
	clientReq := &client.StartGameRequest{
		UserID:   req.UserID,
		Username: req.Username,
		GameID:   req.GameID,
	}
	
	resp, err := a.realClient.StartGame(ctx, clientReq)
	if err != nil {
		return nil, err
	}
	
	// Convert game client response to session service response
	return &StartGameResponse{
		SessionID:   resp.SessionID,
		ContainerID: resp.ContainerID,
		PodName:     resp.PodName,
		Success:     resp.Success,
		Error:       resp.Error,
	}, nil
}

func (a *gameClientAdapter) StopGame(ctx context.Context, req *StopGameRequest) (*StopGameResponse, error) {
	// Convert session service request to game client request
	clientReq := &client.StopGameRequest{
		SessionID: req.SessionID,
		UserID:    req.UserID,
		Force:     req.Force,
		Reason:    req.Reason,
	}
	
	resp, err := a.realClient.StopGame(ctx, clientReq)
	if err != nil {
		return nil, err
	}
	
	// Convert game client response to session service response
	return &StopGameResponse{
		Success: resp.Success,
		Error:   resp.Error,
	}, nil
}

func (a *gameClientAdapter) GetGameSession(ctx context.Context, sessionID string) (*GameSessionInfo, error) {
	resp, err := a.realClient.GetGameSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	
	// Convert game client response to session service response
	return &GameSessionInfo{
		SessionID:     resp.SessionID,
		UserID:        resp.UserID,
		Username:      resp.Username,
		GameID:        resp.GameID,
		Status:        resp.Status,
		StartTime:     resp.StartTime,
		LastActivity:  resp.LastActivity,
		ContainerID:   resp.ContainerID,
		PodName:       resp.PodName,
		RecordingPath: resp.RecordingPath,
		Spectators:    resp.Spectators,
		Metadata:      resp.Metadata,
	}, nil
}

func (a *gameClientAdapter) ListActiveGames(ctx context.Context, userID string) ([]*GameSessionInfo, error) {
	resp, err := a.realClient.ListActiveGames(ctx, userID)
	if err != nil {
		return nil, err
	}
	
	// Convert game client response to session service response
	sessions := make([]*GameSessionInfo, len(resp))
	for i, session := range resp {
		sessions[i] = &GameSessionInfo{
			SessionID:     session.SessionID,
			UserID:        session.UserID,
			Username:      session.Username,
			GameID:        session.GameID,
			Status:        session.Status,
			StartTime:     session.StartTime,
			LastActivity:  session.LastActivity,
			ContainerID:   session.ContainerID,
			PodName:       session.PodName,
			RecordingPath: session.RecordingPath,
			Spectators:    session.Spectators,
			Metadata:      session.Metadata,
		}
	}
	
	return sessions, nil
}

func (a *gameClientAdapter) ListGames(ctx context.Context) ([]*Game, error) {
	// The real client doesn't have ListGames, so we need to implement this
	// For now, return a simple hardcoded list - this should eventually call the game service
	return []*Game{
		{
			ID:          "nethack",
			Name:        "NetHack",
			ShortName:   "NH",
			Description: "The classic roguelike dungeon crawler",
			Enabled:     true,
			Binary:      "/opt/homebrew/bin/nethack",
			Args:        []string{"-u"},
			WorkingDir:  "/tmp/nethack-saves",
			Environment: map[string]string{
				"TERM":       "xterm-256color",
				"HACKDIR":    "/tmp/nethack-saves",
				"NETHACKDIR": "/tmp/nethack-saves",
				"HOME":       "/tmp/nethack-saves",
			},
			MaxPlayers:  1,
			Spectatable: true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "bash",
			Name:        "Bash Shell",
			ShortName:   "bash",
			Description: "Interactive command line",
			Enabled:     true,
			Binary:      "/bin/bash",
			Args:        []string{},
			WorkingDir:  "/tmp",
			Environment: map[string]string{
				"TERM": "xterm-256color",
			},
			MaxPlayers:  1,
			Spectatable: true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}, nil
}

func (a *gameClientAdapter) HealthCheck(ctx context.Context) (bool, error) {
	return a.realClient.HealthCheck(ctx)
}

func (a *gameClientAdapter) Close() error {
	return a.realClient.Close()
}
