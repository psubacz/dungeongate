package application

import (
	"context"
	"fmt"
	"time"

	"github.com/dungeongate/internal/games/domain"
)

// GameService provides application-level game management operations
type GameService struct {
	gameRepo    domain.GameRepository
	sessionRepo domain.SessionRepository
	saveRepo    domain.SaveRepository
	eventRepo   domain.EventRepository
	uow         domain.UnitOfWork
}

// NewGameService creates a new game service
func NewGameService(
	gameRepo domain.GameRepository,
	sessionRepo domain.SessionRepository,
	saveRepo domain.SaveRepository,
	eventRepo domain.EventRepository,
	uow domain.UnitOfWork,
) *GameService {
	return &GameService{
		gameRepo:    gameRepo,
		sessionRepo: sessionRepo,
		saveRepo:    saveRepo,
		eventRepo:   eventRepo,
		uow:         uow,
	}
}

// GetGame retrieves a game by ID
func (s *GameService) GetGame(ctx context.Context, gameID string) (*domain.Game, error) {
	id := domain.NewGameID(gameID)
	return s.gameRepo.FindByID(ctx, id)
}

// ListGames retrieves all games
func (s *GameService) ListGames(ctx context.Context) ([]*domain.Game, error) {
	return s.gameRepo.FindAll(ctx)
}

// ListEnabledGames retrieves all enabled games
func (s *GameService) ListEnabledGames(ctx context.Context) ([]*domain.Game, error) {
	return s.gameRepo.FindEnabled(ctx)
}

// CreateGame creates a new game
func (s *GameService) CreateGame(ctx context.Context, req *CreateGameRequest) (*domain.Game, error) {
	// Validate request
	if err := s.validateCreateGameRequest(req); err != nil {
		return nil, fmt.Errorf("invalid create game request: %w", err)
	}

	// Check if game already exists
	id := domain.NewGameID(req.ID)
	if _, err := s.gameRepo.FindByID(ctx, id); err == nil {
		return nil, fmt.Errorf("game with ID %s already exists", req.ID)
	}

	// Create game domain object
	metadata := domain.GameMetadata{
		Name:        req.Name,
		ShortName:   req.ShortName,
		Description: req.Description,
		Category:    req.Category,
		Tags:        req.Tags,
		Version:     req.Version,
		Difficulty:  req.Difficulty,
	}

	config := domain.GameConfig{
		Binary: domain.BinaryConfig{
			Path:             req.BinaryPath,
			Args:             req.BinaryArgs,
			WorkingDirectory: req.WorkingDirectory,
		},
		Environment: req.Environment,
		Resources: domain.ResourceConfig{
			CPULimit:    req.CPULimit,
			MemoryLimit: req.MemoryLimit,
			DiskLimit:   req.DiskLimit,
			Timeout:     time.Duration(req.TimeoutSeconds) * time.Second,
		},
		Security: domain.SecurityConfig{
			RunAsUser:                req.RunAsUser,
			RunAsGroup:               req.RunAsGroup,
			ReadOnlyRootFilesystem:   req.ReadOnlyRootFilesystem,
			AllowPrivilegeEscalation: req.AllowPrivilegeEscalation,
			Capabilities:             req.Capabilities,
		},
		Networking: domain.NetworkConfig{
			Isolated:       req.NetworkIsolated,
			AllowedPorts:   req.AllowedPorts,
			AllowedDomains: req.AllowedDomains,
			BlockInternet:  req.BlockInternet,
		},
	}

	game := domain.NewGame(id, metadata, config)

	// Save the game
	if err := s.gameRepo.Save(ctx, game); err != nil {
		return nil, fmt.Errorf("failed to save game: %w", err)
	}

	return game, nil
}

// UpdateGame updates an existing game
func (s *GameService) UpdateGame(ctx context.Context, gameID string, req *UpdateGameRequest) (*domain.Game, error) {
	id := domain.NewGameID(gameID)

	// Find existing game
	game, err := s.gameRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("game not found: %w", err)
	}

	// Update metadata if provided
	if req.Metadata != nil {
		metadata := domain.GameMetadata{
			Name:        req.Metadata.Name,
			ShortName:   req.Metadata.ShortName,
			Description: req.Metadata.Description,
			Category:    req.Metadata.Category,
			Tags:        req.Metadata.Tags,
			Version:     req.Metadata.Version,
			Difficulty:  req.Metadata.Difficulty,
		}
		game.UpdateMetadata(metadata)
	}

	// Update config if provided
	if req.Config != nil {
		config := domain.GameConfig{
			Binary: domain.BinaryConfig{
				Path:             req.Config.BinaryPath,
				Args:             req.Config.BinaryArgs,
				WorkingDirectory: req.Config.WorkingDirectory,
			},
			Environment: req.Config.Environment,
			Resources: domain.ResourceConfig{
				CPULimit:    req.Config.CPULimit,
				MemoryLimit: req.Config.MemoryLimit,
				DiskLimit:   req.Config.DiskLimit,
				Timeout:     time.Duration(req.Config.TimeoutSeconds) * time.Second,
			},
			Security: domain.SecurityConfig{
				RunAsUser:                req.Config.RunAsUser,
				RunAsGroup:               req.Config.RunAsGroup,
				ReadOnlyRootFilesystem:   req.Config.ReadOnlyRootFilesystem,
				AllowPrivilegeEscalation: req.Config.AllowPrivilegeEscalation,
				Capabilities:             req.Config.Capabilities,
			},
			Networking: domain.NetworkConfig{
				Isolated:       req.Config.NetworkIsolated,
				AllowedPorts:   req.Config.AllowedPorts,
				AllowedDomains: req.Config.AllowedDomains,
				BlockInternet:  req.Config.BlockInternet,
			},
		}
		game.UpdateConfig(config)
	}

	// Update status if provided
	if req.Status != nil {
		switch *req.Status {
		case "enabled":
			game.Enable()
		case "disabled":
			game.Disable()
		case "maintenance":
			game.SetMaintenance()
		}
	}

	// Save the updated game
	if err := s.gameRepo.Save(ctx, game); err != nil {
		return nil, fmt.Errorf("failed to update game: %w", err)
	}

	return game, nil
}

// DeleteGame deletes a game
func (s *GameService) DeleteGame(ctx context.Context, gameID string) error {
	id := domain.NewGameID(gameID)

	// Check if game has active sessions
	activeSessions, err := s.sessionRepo.FindActiveByGame(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to check active sessions: %w", err)
	}

	if len(activeSessions) > 0 {
		return fmt.Errorf("cannot delete game with active sessions")
	}

	// Delete the game
	return s.gameRepo.Delete(ctx, id)
}

// EnableGame enables a game
func (s *GameService) EnableGame(ctx context.Context, gameID string) error {
	id := domain.NewGameID(gameID)

	game, err := s.gameRepo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("game not found: %w", err)
	}

	game.Enable()
	return s.gameRepo.Save(ctx, game)
}

// DisableGame disables a game
func (s *GameService) DisableGame(ctx context.Context, gameID string) error {
	id := domain.NewGameID(gameID)

	game, err := s.gameRepo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("game not found: %w", err)
	}

	game.Disable()
	return s.gameRepo.Save(ctx, game)
}

// GetGameStatistics retrieves game statistics
func (s *GameService) GetGameStatistics(ctx context.Context, gameID string) (*domain.GameStatistics, error) {
	id := domain.NewGameID(gameID)

	game, err := s.gameRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("game not found: %w", err)
	}

	stats := game.Statistics()
	return &stats, nil
}

// validateCreateGameRequest validates a create game request
func (s *GameService) validateCreateGameRequest(req *CreateGameRequest) error {
	if req.ID == "" {
		return fmt.Errorf("game ID is required")
	}
	if req.Name == "" {
		return fmt.Errorf("game name is required")
	}
	if req.BinaryPath == "" {
		return fmt.Errorf("binary path is required")
	}
	if req.Difficulty < 1 || req.Difficulty > 10 {
		return fmt.Errorf("difficulty must be between 1 and 10")
	}
	return nil
}
