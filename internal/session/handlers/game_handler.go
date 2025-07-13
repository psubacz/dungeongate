package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/dungeongate/internal/session-old/client"
	"github.com/dungeongate/internal/session/pools"
	"github.com/dungeongate/internal/session/resources"
	authv1 "github.com/dungeongate/pkg/api/auth/v1"
	gamev2 "github.com/dungeongate/pkg/api/games/v2"
)

// GameHandler handles game session management with PTY pool integration
type GameHandler struct {
	gameClient      *client.GameClient
	ptyPool         *pools.PTYPool
	resourceTracker *resources.ResourceTracker
	workerPool      *pools.WorkerPool
	logger          *slog.Logger

	// Metrics
	gameSessionsStarted    *resources.CounterMetric
	gameSessionsActive     *resources.GaugeMetric
	gameSessionDuration    *resources.HistogramMetric
	gameErrors             *resources.CounterMetric
	ptyOperations          *resources.CounterMetric
}

// NewGameHandler creates a new game handler
func NewGameHandler(
	gameClient *client.GameClient,
	ptyPool *pools.PTYPool,
	resourceTracker *resources.ResourceTracker,
	workerPool *pools.WorkerPool,
	metricsRegistry *resources.MetricsRegistry,
	logger *slog.Logger,
) *GameHandler {
	gh := &GameHandler{
		gameClient:      gameClient,
		ptyPool:         ptyPool,
		resourceTracker: resourceTracker,
		workerPool:      workerPool,
		logger:          logger,
	}

	gh.initializeMetrics(metricsRegistry)
	return gh
}

// initializeMetrics sets up metrics for the game handler
func (gh *GameHandler) initializeMetrics(registry *resources.MetricsRegistry) {
	gh.gameSessionsStarted = registry.RegisterCounter(
		"session_game_sessions_started_total",
		"Total number of game sessions started",
		map[string]string{"handler": "game"})

	gh.gameSessionsActive = registry.RegisterGauge(
		"session_game_sessions_active",
		"Number of active game sessions",
		map[string]string{"handler": "game"})

	gh.gameSessionDuration = registry.RegisterHistogram(
		"session_game_session_duration_seconds",
		"Time spent in game sessions",
		nil,
		map[string]string{"handler": "game"})

	gh.gameErrors = registry.RegisterCounter(
		"session_game_errors_total",
		"Total number of game errors",
		map[string]string{"handler": "game"})

	gh.ptyOperations = registry.RegisterCounter(
		"session_game_pty_operations_total",
		"Total number of PTY operations",
		map[string]string{"handler": "game", "operation": "unknown"})
}

// StartGameSession handles game session lifecycle with PTY pool
func (gh *GameHandler) StartGameSession(ctx context.Context, conn *pools.Connection, userInfo *authv1.User, gameID string) error {
	startTime := time.Now()
	gh.gameSessionsStarted.Inc()
	gh.gameSessionsActive.Inc()
	defer gh.gameSessionsActive.Dec()

	// Track session duration
	defer func() {
		duration := time.Since(startTime)
		gh.gameSessionDuration.Observe(duration.Seconds())
		gh.logger.Info("Game session completed",
			"user_id", userInfo.Id,
			"game_id", gameID,
			"duration", duration,
			"connection_id", conn.ID)
	}()

	gh.logger.Info("Starting game session",
		"user_id", userInfo.Id,
		"username", userInfo.Username,
		"game_id", gameID,
		"connection_id", conn.ID)

	// Convert string ID to int32 for the API
	userID, err := strconv.ParseInt(userInfo.Id, 10, 32)
	if err != nil {
		gh.logger.Error("Invalid user ID format",
			"user_id", userInfo.Id,
			"error", err,
			"connection_id", conn.ID)
		gh.gameErrors.Inc()
		conn.SSHChannel.Write([]byte("Invalid user ID. Please contact administrator.\r\n"))
		return fmt.Errorf("invalid user ID format: %w", err)
	}

	// Create gRPC stream FIRST to avoid race condition
	stream, err := gh.gameClient.StreamGameIO(ctx)
	if err != nil {
		gh.logger.Error("Failed to create game I/O stream",
			"error", err,
			"username", userInfo.Username,
			"game_id", gameID,
			"connection_id", conn.ID)
		gh.gameErrors.Inc()
		conn.SSHChannel.Write([]byte("Failed to connect to game session\r\n"))
		return fmt.Errorf("failed to create I/O stream: %w", err)
	}
	defer stream.CloseSend()

	// Get terminal dimensions (default if not available)
	terminalCols, terminalRows := 80, 24
	// TODO: Get actual terminal dimensions from connection or session context

	// Start the game session with PTY
	sessionInfo, err := gh.gameClient.StartGameSession(ctx, int32(userID), userInfo.Username, gameID, terminalCols, terminalRows)
	if err != nil {
		gh.logger.Error("Failed to start game session",
			"error", err,
			"username", userInfo.Username,
			"game_id", gameID,
			"connection_id", conn.ID)
		gh.gameErrors.Inc()

		// Check if the error is due to game service unavailability
		if !gh.gameClient.IsHealthy(ctx) {
			gh.logger.Info("Game service became unavailable",
				"username", userInfo.Username,
				"connection_id", conn.ID)
			conn.SSHChannel.Write([]byte("Game service temporarily unavailable. Please try again later.\r\n"))
			return fmt.Errorf("game service unavailable")
		}
		conn.SSHChannel.Write([]byte("Failed to start game session\r\n"))
		return fmt.Errorf("failed to start game session: %w", err)
	}

	// Successfully started game session
	sessionID := sessionInfo.ID
	gh.logger.Info("Started game session",
		"session_id", sessionID,
		"user", userInfo.Username,
		"game", gameID,
		"connection_id", conn.ID)

	// Update connection state to active
	conn.State = pools.ConnectionStateActive

	// Handle I/O using the pre-established stream
	return gh.handleGameIOWithStream(ctx, conn, sessionID, stream)
}

// HandleGameSelection shows the game selection menu and handles the choice
func (gh *GameHandler) HandleGameSelection(ctx context.Context, conn *pools.Connection, userInfo *authv1.User) error {
	gh.logger.Info("Showing game selection",
		"user_id", userInfo.Id,
		"username", userInfo.Username,
		"connection_id", conn.ID)

	// For now, this is a placeholder that starts NetHack directly
	// In a real implementation, this would show a game selection menu
	gameID := "nethack"

	conn.SSHChannel.Write([]byte("Starting NetHack...\r\n"))
	time.Sleep(1 * time.Second)

	return gh.StartGameSession(ctx, conn, userInfo, gameID)
}

// StopGameSession stops a game session
func (gh *GameHandler) StopGameSession(ctx context.Context, sessionID string) error {
	gh.logger.Info("Stopping game session", "session_id", sessionID)

	// This would be implemented to stop a specific game session
	// For now, it's a placeholder
	return nil
}

// ResizeTerminal handles terminal resize requests
func (gh *GameHandler) ResizeTerminal(ctx context.Context, sessionID string, cols, rows int) error {
	gh.ptyOperations.Inc()
	gh.logger.Debug("Resizing terminal",
		"session_id", sessionID,
		"cols", cols,
		"rows", rows)

	// Send resize request to Game Service
	if err := gh.gameClient.ResizeTerminal(ctx, sessionID, cols, rows); err != nil {
		gh.logger.Error("Failed to resize terminal",
			"error", err,
			"session_id", sessionID)
		gh.gameErrors.Inc()
		return fmt.Errorf("failed to resize terminal: %w", err)
	}

	return nil
}

// handleGameIOWithStream handles I/O using a pre-established gRPC stream
func (gh *GameHandler) handleGameIOWithStream(ctx context.Context, conn *pools.Connection, sessionID string, stream gamev2.GameService_StreamGameIOClient) error {
	gh.logger.Info("Starting game I/O handling with pre-established stream",
		"session_id", sessionID,
		"connection_id", conn.ID)

	// Send connect request
	connectReq := &gamev2.GameIORequest{
		Request: &gamev2.GameIORequest_Connect{
			Connect: &gamev2.ConnectPTYRequest{
				SessionId: sessionID,
			},
		},
	}

	if err := stream.Send(connectReq); err != nil {
		gh.logger.Error("Failed to send connect request",
			"error", err,
			"session_id", sessionID,
			"connection_id", conn.ID)
		gh.gameErrors.Inc()
		conn.SSHChannel.Write([]byte("Failed to connect to game session\r\n"))
		return fmt.Errorf("failed to send connect request: %w", err)
	}

	// Wait for connect response
	resp, err := stream.Recv()
	if err != nil {
		gh.logger.Error("Failed to receive connect response",
			"error", err,
			"session_id", sessionID,
			"connection_id", conn.ID)
		gh.gameErrors.Inc()
		conn.SSHChannel.Write([]byte("Failed to connect to game session\r\n"))
		return fmt.Errorf("failed to receive connect response: %w", err)
	}

	// Check if connection was successful
	connectResp := resp.GetConnected()
	if connectResp == nil || !connectResp.Success {
		errorMsg := "Unknown error"
		if connectResp != nil {
			errorMsg = connectResp.Error
		}
		gh.logger.Error("Failed to connect to PTY",
			"error", errorMsg,
			"session_id", sessionID,
			"connection_id", conn.ID)
		gh.gameErrors.Inc()
		conn.SSHChannel.Write([]byte(fmt.Sprintf("Failed to connect to game session: %s\r\n", errorMsg)))
		return fmt.Errorf("PTY connection failed: %s", errorMsg)
	}

	gh.logger.Info("Successfully connected to PTY",
		"session_id", sessionID,
		"pty_id", connectResp.PtyId,
		"connection_id", conn.ID)

	// Set up bidirectional I/O
	done := make(chan error, 2)

	// Create work items for I/O handling
	inputWork := &pools.WorkItem{
		Type:       pools.WorkTypeGameIO,
		Connection: conn,
		Handler:    func(ctx context.Context, conn *pools.Connection) error {
			return gh.handleInput(ctx, conn, sessionID, stream, done)
		},
		Context:  ctx,
		Priority: pools.PriorityHigh,
		QueuedAt: time.Now(),
	}

	outputWork := &pools.WorkItem{
		Type:       pools.WorkTypeGameIO,
		Connection: conn,
		Handler:    func(ctx context.Context, conn *pools.Connection) error {
			return gh.handleOutput(ctx, conn, sessionID, stream, done)
		},
		Context:  ctx,
		Priority: pools.PriorityHigh,
		QueuedAt: time.Now(),
	}

	// Submit I/O work to worker pool
	if err := gh.workerPool.Submit(inputWork); err != nil {
		gh.logger.Error("Failed to submit input work",
			"error", err,
			"session_id", sessionID,
			"connection_id", conn.ID)
		gh.gameErrors.Inc()
		return fmt.Errorf("failed to submit input work: %w", err)
	}

	if err := gh.workerPool.Submit(outputWork); err != nil {
		gh.logger.Error("Failed to submit output work",
			"error", err,
			"session_id", sessionID,
			"connection_id", conn.ID)
		gh.gameErrors.Inc()
		return fmt.Errorf("failed to submit output work: %w", err)
	}

	// Wait for either I/O handler to finish
	err = <-done
	if err != nil {
		gh.logger.Error("Game I/O error",
			"error", err,
			"session_id", sessionID,
			"connection_id", conn.ID)
	}

	// Send disconnect request
	disconnectReq := &gamev2.GameIORequest{
		Request: &gamev2.GameIORequest_Disconnect{
			Disconnect: &gamev2.DisconnectPTYRequest{
				SessionId: sessionID,
				Reason:    "session ended",
			},
		},
	}
	stream.Send(disconnectReq)

	gh.logger.Info("Game I/O handling ended",
		"session_id", sessionID,
		"connection_id", conn.ID)

	return err
}

// handleInput handles SSH channel -> gRPC stream (user input)
func (gh *GameHandler) handleInput(ctx context.Context, conn *pools.Connection, sessionID string, stream gamev2.GameService_StreamGameIOClient, done chan<- error) error {
	buffer := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			done <- ctx.Err()
			return ctx.Err()
		default:
		}

		n, err := conn.SSHChannel.Read(buffer)
		if err != nil {
			gh.logger.Debug("SSH channel read error",
				"error", err,
				"session_id", sessionID,
				"connection_id", conn.ID)
			done <- err
			return err
		}

		// Send input to game via gRPC
		inputReq := &gamev2.GameIORequest{
			Request: &gamev2.GameIORequest_Input{
				Input: &gamev2.PTYInput{
					SessionId: sessionID,
					Data:      buffer[:n],
				},
			},
		}

		if err := stream.Send(inputReq); err != nil {
			gh.logger.Error("Failed to send input to game",
				"error", err,
				"session_id", sessionID,
				"connection_id", conn.ID)
			gh.gameErrors.Inc()
			done <- err
			return err
		}
	}
}

// handleOutput handles gRPC stream -> SSH channel (game output)
func (gh *GameHandler) handleOutput(ctx context.Context, conn *pools.Connection, sessionID string, stream gamev2.GameService_StreamGameIOClient, done chan<- error) error {
	for {
		select {
		case <-ctx.Done():
			done <- ctx.Err()
			return ctx.Err()
		default:
		}

		resp, err := stream.Recv()
		if err != nil {
			gh.logger.Debug("gRPC stream receive error",
				"error", err,
				"session_id", sessionID,
				"connection_id", conn.ID)
			done <- err
			return err
		}

		// Handle different response types
		switch respType := resp.Response.(type) {
		case *gamev2.GameIOResponse_Output:
			// Forward output to SSH channel
			n, err := conn.SSHChannel.Write(respType.Output.Data)
			if err != nil {
				gh.logger.Error("Failed to write to SSH channel",
					"error", err,
					"session_id", sessionID,
					"connection_id", conn.ID)
				gh.gameErrors.Inc()
				done <- err
				return err
			}
			gh.logger.Debug("Forwarded game output to SSH channel",
				"bytes", n,
				"session_id", sessionID,
				"connection_id", conn.ID)

		case *gamev2.GameIOResponse_Event:
			// Handle PTY events
			event := respType.Event
			gh.logger.Info("Received PTY event",
				"type", event.Type,
				"message", event.Message,
				"session_id", sessionID,
				"connection_id", conn.ID)

			// For process exit events, notify the user and end the session
			if event.Type == gamev2.PTYEventType_PTY_EVENT_PROCESS_EXIT {
				conn.SSHChannel.Write([]byte("\r\n\r\nGame session ended.\r\n"))
				done <- fmt.Errorf("game session ended")
				return nil
			}

		case *gamev2.GameIOResponse_Disconnected:
			// PTY disconnected
			gh.logger.Info("PTY disconnected",
				"session_id", sessionID,
				"connection_id", conn.ID)
			done <- fmt.Errorf("PTY disconnected")
			return nil

		default:
			gh.logger.Warn("Unknown gRPC response type",
				"type", fmt.Sprintf("%T", respType),
				"session_id", sessionID,
				"connection_id", conn.ID)
		}
	}
}