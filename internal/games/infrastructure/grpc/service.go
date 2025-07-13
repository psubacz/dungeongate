package grpc

import (
	"context"
	"log/slog"
	"os/exec"
	"syscall"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/dungeongate/internal/games/adapters"
	"github.com/dungeongate/internal/games/application"
	"github.com/dungeongate/internal/games/domain"
	"github.com/dungeongate/internal/games/infrastructure/pty"
	games_pb "github.com/dungeongate/pkg/api/games/v2"
	"github.com/dungeongate/pkg/config"
)

// GameServiceServer implements the gRPC GameService interface
type GameServiceServer struct {
	games_pb.UnimplementedGameServiceServer
	gameService    *application.GameService
	sessionService *application.SessionService
	ptyManager     *pty.PTYManager
	streamHandler  *StreamHandler
	logger         *slog.Logger
	gameConfigs    []*config.GameConfig
}

// NewGameServiceServer creates a new GameServiceServer
func NewGameServiceServer(cfg *config.GameServiceConfig, gameService *application.GameService, sessionService *application.SessionService, logger *slog.Logger) *GameServiceServer {
	// Create adapter registry with configuration
	adapterRegistry, err := adapters.NewGameAdapterRegistryWithConfig(cfg.Games)
	if err != nil {
		logger.Error("Failed to create adapter registry with config, using default", "error", err)
		adapterRegistry = adapters.NewGameAdapterRegistry()
	}

	// Create PTY manager with configured adapters
	ptyManager := pty.NewPTYManagerWithAdapters(logger, adapterRegistry)
	streamHandler := NewStreamHandler(ptyManager, logger)

	return &GameServiceServer{
		gameService:    gameService,
		sessionService: sessionService,
		ptyManager:     ptyManager,
		streamHandler:  streamHandler,
		logger:         logger,
		gameConfigs:    cfg.Games,
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

	// Get games from the application service
	domainGames, err := s.gameService.ListEnabledGames(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list games: "+err.Error())
	}

	// Convert domain games to protobuf games
	games := make([]*games_pb.Game, 0, len(domainGames))
	for _, domainGame := range domainGames {
		game := &games_pb.Game{
			Id:          domainGame.ID().String(),
			Name:        domainGame.Metadata().Name,
			ShortName:   domainGame.Metadata().ShortName,
			Description: domainGame.Metadata().Description,
			Category:    domainGame.Metadata().Category,
			Tags:        domainGame.Metadata().Tags,
			Version:     domainGame.Metadata().Version,
			Difficulty:  int32(domainGame.Metadata().Difficulty),
			Status:      games_pb.GameStatus_GAME_STATUS_ENABLED, // TODO: Convert domain status
		}
		games = append(games, game)
	}

	return &games_pb.ListGamesResponse{
		Games:      games,
		TotalCount: int32(len(games)),
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

	// Validate request
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}
	if req.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id must be greater than 0")
	}
	if req.Username == "" {
		return nil, status.Error(codes.InvalidArgument, "username cannot be empty")
	}
	if req.GameId == "" {
		return nil, status.Error(codes.InvalidArgument, "game_id cannot be empty")
	}
	if req.TerminalSize == nil {
		return nil, status.Error(codes.InvalidArgument, "terminal_size cannot be nil")
	}
	if req.TerminalSize.Width <= 0 || req.TerminalSize.Height <= 0 {
		return nil, status.Error(codes.InvalidArgument, "terminal_size must have positive width and height")
	}

	// Convert protobuf request to application request
	appReq := &application.StartSessionRequest{
		UserID:           int(req.UserId),
		Username:         req.Username,
		GameID:           req.GameId,
		TerminalWidth:    int(req.TerminalSize.Width),
		TerminalHeight:   int(req.TerminalSize.Height),
		EnableRecording:  req.EnableRecording,
		EnableStreaming:  req.EnableStreaming,
		EnableEncryption: req.EnableEncryption,
	}

	// Call the application service
	session, err := s.sessionService.StartGameSession(ctx, appReq)
	if err != nil {
		// Map domain errors to appropriate gRPC codes
		switch {
		case err.Error() == "game not found":
			return nil, status.Error(codes.NotFound, "game not found")
		case err.Error() == "user already has active session":
			return nil, status.Error(codes.AlreadyExists, "user already has active session for this game")
		default:
			return nil, status.Error(codes.Internal, "failed to start game session: "+err.Error())
		}
	}

	// Get game configuration
	var gameConfig *config.GameConfig
	for _, cfg := range s.gameConfigs {
		if cfg.ID == req.GameId {
			gameConfig = cfg
			break
		}
	}
	if gameConfig == nil {
		return nil, status.Error(codes.NotFound, "game configuration not found")
	}

	// Use the configured game path
	gamePath := gameConfig.Binary.Path
	// Let the adapter handle args and env - pass empty slices
	gameArgs := []string{}
	gameEnv := []string{}

	// Create callback to handle process exit
	processExitCallback := func(exitSession *domain.GameSession, exitCode *int, processErr error) {
		// Handle session cleanup when process exits
		s.logger.Info("Game process exited", "session_id", exitSession.ID().String(), "exit_code", exitCode)

		// Mark session as ended in the session service
		// Note: This is a simplified approach. In a full implementation,
		// we'd want to coordinate with the session manager for save creation
		var signal *string
		if processErr != nil {
			if exitError, ok := processErr.(*exec.ExitError); ok {
				if sys := exitError.Sys(); sys != nil {
					if ws, ok := sys.(syscall.WaitStatus); ok && ws.Signaled() {
						sig := ws.Signal().String()
						signal = &sig
					}
				}
			}
		}
		exitSession.End(exitCode, signal)
	}

	_, err = s.ptyManager.CreatePTYWithCallback(ctx, session, gamePath, gameArgs, gameEnv, processExitCallback)
	if err != nil {
		s.logger.Error("Failed to create PTY", "error", err, "session_id", session.ID().String())
		// TODO: Clean up the session in the database
		return nil, status.Error(codes.Internal, "failed to create PTY: "+err.Error())
	}

	// Update session status to active
	session.Start(domain.ProcessInfo{
		PID: 0, // TODO: Get actual PID from PTY
	})

	// Convert domain session to protobuf response
	pbSession := s.domainSessionToPb(session)

	return &games_pb.StartGameSessionResponse{
		Session: pbSession,
	}, nil
}

// StopGameSession stops a game session
func (s *GameServiceServer) StopGameSession(ctx context.Context, req *games_pb.StopGameSessionRequest) (*games_pb.StopGameSessionResponse, error) {
	if s.sessionService == nil {
		return nil, status.Error(codes.Unavailable, "session service not available")
	}

	// Stop the session through the session service
	err := s.sessionService.StopGameSession(ctx, req.SessionId, req.Reason)
	if err != nil {
		s.logger.Error("Failed to stop game session", "error", err, "session_id", req.SessionId)
		return nil, status.Error(codes.Internal, "failed to stop session: "+err.Error())
	}

	// Stop the PTY if it exists
	err = s.ptyManager.ClosePTY(req.SessionId)
	if err != nil {
		s.logger.Warn("Failed to close PTY", "error", err, "session_id", req.SessionId)
		// Don't return error for PTY cleanup failure, session is already stopped
	}

	return &games_pb.StopGameSessionResponse{
		Success: true,
	}, nil
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

// domainSessionToPb converts a domain GameSession to protobuf GameSession
func (s *GameServiceServer) domainSessionToPb(session *domain.GameSession) *games_pb.GameSession {
	if session == nil {
		return nil
	}

	pbSession := &games_pb.GameSession{
		Id:           session.ID().String(),
		UserId:       int32(session.UserID().Int()),
		Username:     session.Username(),
		GameId:       session.GameID().String(),
		Status:       s.domainStatusToPb(session.Status()),
		StartTime:    timestamppb.New(session.StartTime()),
		LastActivity: timestamppb.New(session.LastActivity()),
		TerminalSize: &games_pb.TerminalSize{
			Width:  int32(session.TerminalSize().Width),
			Height: int32(session.TerminalSize().Height),
		},
		Encoding: session.Encoding(),
	}

	// Set end time if session has ended
	if session.EndTime() != nil {
		pbSession.EndTime = timestamppb.New(*session.EndTime())
	}

	// Set process info if available
	if session.ProcessInfo().PID != 0 {
		pbSession.ProcessInfo = &games_pb.ProcessInfo{
			Pid:         int32(session.ProcessInfo().PID),
			ContainerId: session.ProcessInfo().ContainerID,
			PodName:     session.ProcessInfo().PodName,
		}
		if session.ProcessInfo().ExitCode != nil {
			pbSession.ProcessInfo.ExitCode = int32(*session.ProcessInfo().ExitCode)
		}
		if session.ProcessInfo().Signal != nil {
			pbSession.ProcessInfo.Signal = *session.ProcessInfo().Signal
		}
	}

	// Set recording info if available
	if session.RecordingInfo() != nil {
		pbSession.Recording = &games_pb.RecordingInfo{
			Enabled:    session.RecordingInfo().Enabled,
			FilePath:   session.RecordingInfo().FilePath,
			Format:     session.RecordingInfo().Format,
			StartTime:  timestamppb.New(session.RecordingInfo().StartTime),
			FileSize:   session.RecordingInfo().FileSize,
			Compressed: session.RecordingInfo().Compressed,
		}
	}

	// Set streaming info if available
	if session.StreamingInfo() != nil {
		pbSession.Streaming = &games_pb.StreamingInfo{
			Enabled:       session.StreamingInfo().Enabled,
			Protocol:      session.StreamingInfo().Protocol,
			Encrypted:     session.StreamingInfo().Encrypted,
			FrameCount:    session.StreamingInfo().FrameCount,
			BytesStreamed: session.StreamingInfo().BytesStreamed,
		}
	}

	// Convert spectators
	spectators := session.Spectators()
	if len(spectators) > 0 {
		pbSession.Spectators = make([]*games_pb.SpectatorInfo, len(spectators))
		for i, spectator := range spectators {
			pbSession.Spectators[i] = &games_pb.SpectatorInfo{
				UserId:    int32(spectator.UserID.Int()),
				Username:  spectator.Username,
				JoinTime:  timestamppb.New(spectator.JoinTime),
				BytesSent: spectator.BytesSent,
				IsActive:  spectator.IsActive,
			}
		}
	}

	return pbSession
}

// domainStatusToPb converts domain SessionStatus to protobuf SessionStatus
func (s *GameServiceServer) domainStatusToPb(status domain.SessionStatus) games_pb.SessionStatus {
	switch status {
	case domain.SessionStatusStarting:
		return games_pb.SessionStatus_SESSION_STATUS_STARTING
	case domain.SessionStatusActive:
		return games_pb.SessionStatus_SESSION_STATUS_ACTIVE
	case domain.SessionStatusPaused:
		return games_pb.SessionStatus_SESSION_STATUS_PAUSED
	case domain.SessionStatusEnding:
		return games_pb.SessionStatus_SESSION_STATUS_ENDING
	case domain.SessionStatusEnded:
		return games_pb.SessionStatus_SESSION_STATUS_ENDED
	case domain.SessionStatusFailed:
		return games_pb.SessionStatus_SESSION_STATUS_FAILED
	default:
		return games_pb.SessionStatus_SESSION_STATUS_UNSPECIFIED
	}
}

// StreamGameIO handles bidirectional streaming for PTY I/O
func (s *GameServiceServer) StreamGameIO(stream games_pb.GameService_StreamGameIOServer) error {
	if s.streamHandler == nil {
		return status.Error(codes.Internal, "stream handler not initialized")
	}

	return s.streamHandler.HandleStream(stream)
}

// ResizeTerminal handles terminal resize requests
func (s *GameServiceServer) ResizeTerminal(ctx context.Context, req *games_pb.ResizeTerminalRequest) (*games_pb.ResizeTerminalResponse, error) {
	if req.SessionId == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}
	if req.NewSize == nil {
		return nil, status.Error(codes.InvalidArgument, "new_size is required")
	}

	// Resize the PTY
	err := s.ptyManager.ResizePTY(req.SessionId, uint16(req.NewSize.Height), uint16(req.NewSize.Width))
	if err != nil {
		s.logger.Error("Failed to resize PTY", "error", err, "session_id", req.SessionId)
		return &games_pb.ResizeTerminalResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	s.logger.Info("Resized PTY", "session_id", req.SessionId, "rows", req.NewSize.Height, "cols", req.NewSize.Width)

	return &games_pb.ResizeTerminalResponse{
		Success: true,
	}, nil
}
