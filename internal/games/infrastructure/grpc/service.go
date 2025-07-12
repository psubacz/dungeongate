package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/dungeongate/internal/games/application"
	"github.com/dungeongate/internal/games/domain"
	"github.com/dungeongate/internal/games/infrastructure/pty"
	games_pb "github.com/dungeongate/pkg/api/games/v2"
)

// GameServiceServer implements the gRPC GameService interface
type GameServiceServer struct {
	games_pb.UnimplementedGameServiceServer
	gameService    *application.GameService
	sessionService *application.SessionService
	ptyManager     *pty.PTYManager
	streamHandler  *StreamHandler
	logger         *slog.Logger
}

// NewGameServiceServer creates a new GameServiceServer
func NewGameServiceServer(gameService *application.GameService, sessionService *application.SessionService, logger *slog.Logger) *GameServiceServer {
	ptyManager := pty.NewPTYManager(logger)
	streamHandler := NewStreamHandler(ptyManager, logger)

	return &GameServiceServer{
		gameService:    gameService,
		sessionService: sessionService,
		ptyManager:     ptyManager,
		streamHandler:  streamHandler,
		logger:         logger,
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

	// Create PTY for the game session
	// TODO: Get actual game binary path and args from game configuration
	gamePath := "/usr/games/nethack" // Hardcoded for now
	gameArgs := []string{}
	gameEnv := append(os.Environ(),
		fmt.Sprintf("TERM=%s", "xterm-256color"),
		fmt.Sprintf("USER=%s", req.Username),
	)

	_, err = s.ptyManager.CreatePTY(ctx, session, gamePath, gameArgs, gameEnv)
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
