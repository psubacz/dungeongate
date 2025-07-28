package client

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	gamev2 "github.com/dungeongate/pkg/api/games/v2"
)

// SessionInfo represents basic session information
type SessionInfo struct {
	ID           string                 `json:"id"`
	UserID       string                 `json:"user_id"`
	Username     string                 `json:"username"`
	GameID       string                 `json:"game_id"`
	State        string                 `json:"state"`
	CreatedAt    time.Time              `json:"created_at"`
	LastActivity time.Time              `json:"last_activity"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// GameClient provides stateless access to Game Service
type GameClient struct {
	conn   *grpc.ClientConn
	client gamev2.GameServiceClient
	logger *slog.Logger
}

// NewGameClient creates a new Game Service client
func NewGameClient(address string, logger *slog.Logger) (*GameClient, error) {
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to game service: %w", err)
	}

	client := gamev2.NewGameServiceClient(conn)

	return &GameClient{
		conn:   conn,
		client: client,
		logger: logger,
	}, nil
}

// Close closes the client connection
func (c *GameClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// StartGameSession starts a new game session
func (c *GameClient) StartGameSession(ctx context.Context, userID int32, username, gameID string, terminalCols, terminalRows int) (*SessionInfo, error) {
	req := &gamev2.StartGameSessionRequest{
		UserId:   userID,
		Username: username,
		GameId:   gameID,
		TerminalSize: &gamev2.TerminalSize{
			Width:  int32(terminalCols),
			Height: int32(terminalRows),
		},
		EnableRecording:  true,
		EnableStreaming:  true,
		EnableEncryption: false,
	}

	resp, err := c.client.StartGameSession(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to start game session: %w", err)
	}

	return &SessionInfo{
		ID:           resp.Session.Id,
		UserID:       fmt.Sprintf("%d", userID),
		GameID:       gameID,
		State:        convertSessionState(resp.Session.Status),
		CreatedAt:    resp.Session.StartTime.AsTime(),
		LastActivity: resp.Session.LastActivity.AsTime(),
		Metadata:     make(map[string]interface{}),
	}, nil
}

// GetGameSession retrieves session information
func (c *GameClient) GetGameSession(ctx context.Context, sessionID string) (*SessionInfo, error) {
	req := &gamev2.GetGameSessionRequest{
		SessionId: sessionID,
	}

	resp, err := c.client.GetGameSession(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get game session: %w", err)
	}

	return &SessionInfo{
		ID:           resp.Session.Id,
		UserID:       fmt.Sprintf("%d", resp.Session.UserId),
		GameID:       resp.Session.GameId,
		State:        convertSessionState(resp.Session.Status),
		CreatedAt:    resp.Session.StartTime.AsTime(),
		LastActivity: resp.Session.LastActivity.AsTime(),
		Metadata:     make(map[string]interface{}),
	}, nil
}

// StopGameSession stops a game session
func (c *GameClient) StopGameSession(ctx context.Context, sessionID, reason string) error {
	req := &gamev2.StopGameSessionRequest{
		SessionId: sessionID,
		Reason:    reason,
		Force:     false,
	}

	_, err := c.client.StopGameSession(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to stop game session: %w", err)
	}

	return nil
}

// ListGameSessions lists sessions for a user
func (c *GameClient) ListGameSessions(ctx context.Context, userID int32) ([]*SessionInfo, error) {
	req := &gamev2.ListGameSessionsRequest{
		UserId: userID,
		Limit:  100,
		Offset: 0,
	}

	resp, err := c.client.ListGameSessions(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list game sessions: %w", err)
	}

	sessions := make([]*SessionInfo, len(resp.Sessions))
	for i, session := range resp.Sessions {
		sessions[i] = &SessionInfo{
			ID:           session.Id,
			UserID:       fmt.Sprintf("%d", session.UserId),
			GameID:       session.GameId,
			State:        convertSessionState(session.Status),
			CreatedAt:    session.StartTime.AsTime(),
			LastActivity: session.LastActivity.AsTime(),
			Metadata:     make(map[string]interface{}),
		}
	}

	return sessions, nil
}

// ListGames retrieves the list of available games
func (c *GameClient) ListGames(ctx context.Context) ([]*gamev2.Game, error) {
	req := &gamev2.ListGamesRequest{
		EnabledOnly: true, // Only show enabled games
		Limit:       100,  // Get up to 100 games
		Offset:      0,
	}

	resp, err := c.client.ListGames(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list games: %w", err)
	}

	return resp.Games, nil
}

// Health checks the health of the game service
func (c *GameClient) Health(ctx context.Context) (*gamev2.HealthResponse, error) {
	resp, err := c.client.Health(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, fmt.Errorf("failed to check game service health: %w", err)
	}

	return resp, nil
}

// IsHealthy checks if the game service is available and healthy
func (c *GameClient) IsHealthy(ctx context.Context) bool {
	resp, err := c.Health(ctx)
	if err != nil {
		c.logger.Debug("Game service health check failed", "error", err)
		return false
	}

	return resp.Status == "healthy" || resp.Status == "ok"
}

// StreamGameIO creates a bidirectional stream for game I/O
func (c *GameClient) StreamGameIO(ctx context.Context) (gamev2.GameService_StreamGameIOClient, error) {
	stream, err := c.client.StreamGameIO(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create game I/O stream: %w", err)
	}
	return stream, nil
}

// ResizeTerminal sends a terminal resize request
func (c *GameClient) ResizeTerminal(ctx context.Context, sessionID string, width, height int) error {
	req := &gamev2.ResizeTerminalRequest{
		SessionId: sessionID,
		NewSize: &gamev2.TerminalSize{
			Width:  int32(width),
			Height: int32(height),
		},
	}

	resp, err := c.client.ResizeTerminal(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to resize terminal: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("terminal resize failed: %s", resp.Error)
	}

	return nil
}

// GetActiveGameSessions returns all active game sessions for spectating
func (c *GameClient) GetActiveGameSessions(ctx context.Context) ([]*gamev2.GameSession, error) {
	req := &gamev2.ListGameSessionsRequest{
		Status: gamev2.SessionStatus_SESSION_STATUS_ACTIVE,
		Limit:  100,
		Offset: 0,
	}

	resp, err := c.client.ListGameSessions(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list active game sessions: %w", err)
	}

	return resp.Sessions, nil
}

// GetGameSessionWithSpectators retrieves detailed session information including spectators
func (c *GameClient) GetGameSessionWithSpectators(ctx context.Context, sessionID string) (*gamev2.GameSession, error) {
	req := &gamev2.GetGameSessionRequest{
		SessionId: sessionID,
	}

	resp, err := c.client.GetGameSession(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get game session with spectators: %w", err)
	}

	return resp.Session, nil
}

// AddSpectator adds a spectator to a game session
func (c *GameClient) AddSpectator(ctx context.Context, sessionID string, spectatorUserID int32, spectatorUsername string) error {
	req := &gamev2.AddSpectatorRequest{
		SessionId:         sessionID,
		SpectatorUserId:   spectatorUserID,
		SpectatorUsername: spectatorUsername,
	}

	resp, err := c.client.AddSpectator(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to add spectator: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("failed to add spectator: %s", resp.Error)
	}

	return nil
}

// RemoveSpectator removes a spectator from a game session
func (c *GameClient) RemoveSpectator(ctx context.Context, sessionID string, spectatorUserID int32) error {
	req := &gamev2.RemoveSpectatorRequest{
		SessionId:       sessionID,
		SpectatorUserId: spectatorUserID,
	}

	resp, err := c.client.RemoveSpectator(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to remove spectator: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("failed to remove spectator: %s", resp.Error)
	}

	return nil
}

// convertSessionState converts protobuf session state to string
func convertSessionState(state gamev2.SessionStatus) string {
	switch state {
	case gamev2.SessionStatus_SESSION_STATUS_STARTING:
		return "starting"
	case gamev2.SessionStatus_SESSION_STATUS_ACTIVE:
		return "active"
	case gamev2.SessionStatus_SESSION_STATUS_PAUSED:
		return "paused"
	case gamev2.SessionStatus_SESSION_STATUS_ENDING:
		return "ending"
	case gamev2.SessionStatus_SESSION_STATUS_ENDED:
		return "ended"
	default:
		return "created"
	}
}
