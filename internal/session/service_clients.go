package session

import (
	"context"
	"fmt"
	"log"
	"time"

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

// gameServiceClient implements GameServiceClient
type gameServiceClient struct {
	address string
	games   []*config.GameConfig
}

// NewGameServiceClient creates a new game service client
func NewGameServiceClient(address string) GameServiceClient {
	return &gameServiceClient{
		address: address,
	}
}

// NewGameServiceClientWithConfig creates a new game service client with configuration
func NewGameServiceClientWithConfig(address string, games []*config.GameConfig) GameServiceClient {
	return &gameServiceClient{
		address: address,
		games:   games,
	}
}

func (c *gameServiceClient) ListGames(ctx context.Context) ([]*Game, error) {
	log.Printf("Game service list games request")

	// If games are configured, use them
	if len(c.games) > 0 {
		games := make([]*Game, 0, len(c.games))
		for _, cfg := range c.games {
			if cfg.Enabled {
				// Check if Binary config exists
				if cfg.Binary == nil {
					log.Printf("Warning: Game %s has no binary configuration", cfg.ID)
					continue
				}
				log.Printf("Loading game %s: binary=%s, workingDir=%s", cfg.ID, cfg.Binary.Path, cfg.Binary.WorkingDirectory)
				game := &Game{
					ID:          cfg.ID,
					Name:        cfg.Name,
					ShortName:   cfg.ShortName,
					Description: fmt.Sprintf("v%s", cfg.Version),
					Enabled:     cfg.Enabled,
					Binary:      cfg.Binary.Path,
					Args:        cfg.Binary.Args,
					WorkingDir:  cfg.Binary.WorkingDirectory,
					Environment: cfg.Environment,
					MaxPlayers:  1,
					Spectatable: true,
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				}

				// Use game settings if available
				if cfg.Settings != nil {
					game.MaxPlayers = cfg.Settings.MaxPlayers
					if cfg.Settings.Spectating != nil {
						game.Spectatable = cfg.Settings.Spectating.Enabled
					}
				}

				games = append(games, game)
			}
		}
		return games, nil
	}

	// Fallback to mock implementation - return NetHack game
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
	}, nil
}

func (c *gameServiceClient) GetGame(ctx context.Context, gameID string) (*Game, error) {
	log.Printf("Game service get game request: %s", gameID)

	// Get games and find the requested one
	games, err := c.ListGames(ctx)
	if err != nil {
		return nil, err
	}

	for _, game := range games {
		if game.ID == gameID {
			return game, nil
		}
	}

	return nil, fmt.Errorf("game not found: %s", gameID)
}

func (c *gameServiceClient) StartGame(ctx context.Context, req *StartGameRequest) (*GameSession, error) {
	log.Printf("Game service start game request: user=%s, game=%s", req.Username, req.GameID)

	// Mock implementation
	return &GameSession{
		ID:       fmt.Sprintf("game_%d", time.Now().UnixNano()),
		UserID:   req.UserID,
		Username: req.Username,
		GameID:   req.GameID,
		Status:   "starting",
	}, nil
}

func (c *gameServiceClient) StopGame(ctx context.Context, sessionID string) error {
	log.Printf("Game service stop game request: %s", sessionID)
	return nil
}

func (c *gameServiceClient) GetGameStatus(ctx context.Context, sessionID string) (*GameSession, error) {
	log.Printf("Game service get game status request: %s", sessionID)
	return nil, fmt.Errorf("not implemented")
}

func (c *gameServiceClient) UpdateGameConfig(ctx context.Context, gameID string, config *Game) error {
	log.Printf("Game service update game config request: %s", gameID)
	return fmt.Errorf("not implemented")
}
