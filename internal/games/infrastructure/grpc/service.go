package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/dungeongate/internal/games/application"
	games_pb "github.com/dungeongate/pkg/api/games"
)

// GameServiceServer implements the gRPC GameService interface
type GameServiceServer struct {
	games_pb.UnimplementedGameServiceServer
	gameService    *application.GameService
	sessionService *application.SessionService
}

// NewGameServiceServer creates a new GameServiceServer
func NewGameServiceServer(gameService *application.GameService, sessionService *application.SessionService) *GameServiceServer {
	return &GameServiceServer{
		gameService:    gameService,
		sessionService: sessionService,
	}
}

// Health implements the health check endpoint
func (s *GameServiceServer) Health(ctx context.Context, req *emptypb.Empty) (*games_pb.HealthResponse, error) {
	return &games_pb.HealthResponse{
		Status: "healthy",
		Details: map[string]string{
			"service": "game-service",
			"version": "v1",
		},
	}, nil
}

// ListGames lists available games
func (s *GameServiceServer) ListGames(ctx context.Context, req *games_pb.ListGamesRequest) (*games_pb.ListGamesResponse, error) {
	if s.gameService == nil {
		return nil, status.Error(codes.Unavailable, "game service not available")
	}

	// For now, return an empty list until repository implementations are complete
	return &games_pb.ListGamesResponse{
		Games:      []*games_pb.Game{},
		TotalCount: 0,
	}, nil
}

// GetGame gets a specific game by ID
func (s *GameServiceServer) GetGame(ctx context.Context, req *games_pb.GetGameRequest) (*games_pb.GetGameResponse, error) {
	if s.gameService == nil {
		return nil, status.Error(codes.Unavailable, "game service not available")
	}

	if req.GameId == "" {
		return nil, status.Error(codes.InvalidArgument, "game_id is required")
	}

	return nil, status.Error(codes.Unimplemented, "method GetGame not implemented")
}

// CreateGame creates a new game
func (s *GameServiceServer) CreateGame(ctx context.Context, req *games_pb.CreateGameRequest) (*games_pb.CreateGameResponse, error) {
	if s.gameService == nil {
		return nil, status.Error(codes.Unavailable, "game service not available")
	}

	return nil, status.Error(codes.Unimplemented, "method CreateGame not implemented")
}

// UpdateGame updates an existing game
func (s *GameServiceServer) UpdateGame(ctx context.Context, req *games_pb.UpdateGameRequest) (*games_pb.UpdateGameResponse, error) {
	if s.gameService == nil {
		return nil, status.Error(codes.Unavailable, "game service not available")
	}

	return nil, status.Error(codes.Unimplemented, "method UpdateGame not implemented")
}

// DeleteGame deletes a game
func (s *GameServiceServer) DeleteGame(ctx context.Context, req *games_pb.DeleteGameRequest) (*games_pb.DeleteGameResponse, error) {
	if s.gameService == nil {
		return nil, status.Error(codes.Unavailable, "game service not available")
	}

	return nil, status.Error(codes.Unimplemented, "method DeleteGame not implemented")
}

// StartGameSession starts a new game session
func (s *GameServiceServer) StartGameSession(ctx context.Context, req *games_pb.StartGameSessionRequest) (*games_pb.StartGameSessionResponse, error) {
	if s.sessionService == nil {
		return nil, status.Error(codes.Unavailable, "session service not available")
	}

	return nil, status.Error(codes.Unimplemented, "method StartGameSession not implemented")
}

// StopGameSession stops a game session
func (s *GameServiceServer) StopGameSession(ctx context.Context, req *games_pb.StopGameSessionRequest) (*games_pb.StopGameSessionResponse, error) {
	if s.sessionService == nil {
		return nil, status.Error(codes.Unavailable, "session service not available")
	}

	return nil, status.Error(codes.Unimplemented, "method StopGameSession not implemented")
}

// GetGameSession gets a specific game session
func (s *GameServiceServer) GetGameSession(ctx context.Context, req *games_pb.GetGameSessionRequest) (*games_pb.GetGameSessionResponse, error) {
	if s.sessionService == nil {
		return nil, status.Error(codes.Unavailable, "session service not available")
	}

	return nil, status.Error(codes.Unimplemented, "method GetGameSession not implemented")
}

// ListGameSessions lists game sessions
func (s *GameServiceServer) ListGameSessions(ctx context.Context, req *games_pb.ListGameSessionsRequest) (*games_pb.ListGameSessionsResponse, error) {
	if s.sessionService == nil {
		return nil, status.Error(codes.Unavailable, "session service not available")
	}

	return nil, status.Error(codes.Unimplemented, "method ListGameSessions not implemented")
}

// SaveGame saves game data
func (s *GameServiceServer) SaveGame(ctx context.Context, req *games_pb.SaveGameRequest) (*games_pb.SaveGameResponse, error) {
	if s.gameService == nil {
		return nil, status.Error(codes.Unavailable, "game service not available")
	}

	return nil, status.Error(codes.Unimplemented, "method SaveGame not implemented")
}

// LoadGame loads game data
func (s *GameServiceServer) LoadGame(ctx context.Context, req *games_pb.LoadGameRequest) (*games_pb.LoadGameResponse, error) {
	if s.gameService == nil {
		return nil, status.Error(codes.Unavailable, "game service not available")
	}

	return nil, status.Error(codes.Unimplemented, "method LoadGame not implemented")
}

// DeleteSave deletes a game save
func (s *GameServiceServer) DeleteSave(ctx context.Context, req *games_pb.DeleteSaveRequest) (*games_pb.DeleteSaveResponse, error) {
	if s.gameService == nil {
		return nil, status.Error(codes.Unavailable, "game service not available")
	}

	return nil, status.Error(codes.Unimplemented, "method DeleteSave not implemented")
}

// ListSaves lists game saves
func (s *GameServiceServer) ListSaves(ctx context.Context, req *games_pb.ListSavesRequest) (*games_pb.ListSavesResponse, error) {
	if s.gameService == nil {
		return nil, status.Error(codes.Unavailable, "game service not available")
	}

	return nil, status.Error(codes.Unimplemented, "method ListSaves not implemented")
}
