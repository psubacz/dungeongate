package connection

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/dungeongate/internal/session/client"
	authv1 "github.com/dungeongate/pkg/api/auth/v1"
	gamev2 "github.com/dungeongate/pkg/api/games/v2"
	"golang.org/x/crypto/ssh"
)

// GameIOHandler handles game session I/O and game lifecycle
type GameIOHandler struct {
	gameClient *client.GameClient
	logger     *slog.Logger
}

// NewGameIOHandler creates a new game I/O handler
func NewGameIOHandler(gameClient *client.GameClient, logger *slog.Logger) *GameIOHandler {
	return &GameIOHandler{
		gameClient: gameClient,
		logger:     logger,
	}
}

// HandleGameIO handles I/O between SSH channel and game session via gRPC streaming
func (h *GameIOHandler) HandleGameIO(ctx context.Context, channel ssh.Channel, sessionID, connID string) {
	h.logger.Info("Starting game I/O handling", "session_id", sessionID, "connection_id", connID)

	// Create gRPC stream to Game Service
	stream, err := h.gameClient.StreamGameIO(ctx)
	if err != nil {
		h.logger.Error("Failed to create game I/O stream", "error", err, "session_id", sessionID)
		channel.Write([]byte("Failed to connect to game session\r\n"))
		return
	}
	defer stream.CloseSend()

	h.HandleGameIOWithStream(ctx, channel, sessionID, connID, stream)
}

// HandleGameIOWithStream handles I/O using a pre-established gRPC stream
func (h *GameIOHandler) HandleGameIOWithStream(ctx context.Context, channel ssh.Channel, sessionID, connID string, stream gamev2.GameService_StreamGameIOClient) {
	h.logger.Info("Starting game I/O handling with pre-established stream", "session_id", sessionID, "connection_id", connID)

	// Send connect request
	connectReq := &gamev2.GameIORequest{
		Request: &gamev2.GameIORequest_Connect{
			Connect: &gamev2.ConnectPTYRequest{
				SessionId: sessionID,
			},
		},
	}

	if err := stream.Send(connectReq); err != nil {
		h.logger.Error("Failed to send connect request", "error", err, "session_id", sessionID)
		channel.Write([]byte("Failed to connect to game session\r\n"))
		return
	}

	// Wait for connect response
	resp, err := stream.Recv()
	if err != nil {
		h.logger.Error("Failed to receive connect response", "error", err, "session_id", sessionID)
		channel.Write([]byte("Failed to connect to game session\r\n"))
		return
	}

	// Check if connection was successful
	connectResp := resp.GetConnected()
	if connectResp == nil || !connectResp.Success {
		errorMsg := "Unknown error"
		if connectResp != nil {
			errorMsg = connectResp.Error
		}
		h.logger.Error("Failed to connect to PTY", "error", errorMsg, "session_id", sessionID)
		channel.Write([]byte(fmt.Sprintf("Failed to connect to game session: %s\r\n", errorMsg)))
		return
	}

	h.logger.Info("Successfully connected to PTY", "session_id", sessionID, "pty_id", connectResp.PtyId)

	// Set up bidirectional I/O
	done := make(chan error, 2)

	// Goroutine to handle SSH channel -> gRPC stream (user input)
	go func() {
		buffer := make([]byte, 4096)
		for {
			n, err := channel.Read(buffer)
			if err != nil {
				h.logger.Debug("SSH channel read error", "error", err, "session_id", sessionID)
				done <- err
				return
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
				h.logger.Error("Failed to send input to game", "error", err, "session_id", sessionID)
				done <- err
				return
			}
		}
	}()

	// Goroutine to handle gRPC stream -> SSH channel (game output)
	go func() {
		for {
			resp, err := stream.Recv()
			if err != nil {
				h.logger.Debug("gRPC stream receive error", "error", err, "session_id", sessionID)
				// Check if this is EOF or context cancellation (normal game exit)
				if err == io.EOF || strings.Contains(err.Error(), "context canceled") {
					// Game ended normally - clear terminal and show message
					channel.Write([]byte("\033[2J\033[H")) // Clear screen and move cursor to home
					channel.Write([]byte("\r\n=== Game ended ===\r\n"))
					channel.Write([]byte("Returning to main menu...\r\n\r\n"))
					time.Sleep(2 * time.Second)
					done <- io.EOF
				} else {
					done <- err
				}
				return
			}

			// Handle different response types
			switch respType := resp.Response.(type) {
			case *gamev2.GameIOResponse_Output:
				h.logger.Debug("Received bytes from game", "session_id", sessionID, "bytes", len(respType.Output.Data), "data", string(respType.Output.Data))
				// Forward output to SSH channel
				n, err := channel.Write(respType.Output.Data)
				if err != nil {
					h.logger.Debug("Failed to write bytes to SSH channel", "session_id", sessionID, "bytes", len(respType.Output.Data), "error", err)
					h.logger.Error("Failed to write to SSH channel", "error", err, "session_id", sessionID)
					done <- err
					return
				} else {
					h.logger.Debug("Successfully wrote bytes to SSH channel", "session_id", sessionID, "bytes_written", n)
				}

			case *gamev2.GameIOResponse_Event:
				// Handle PTY events
				event := respType.Event
				h.logger.Info("Received PTY event", "type", event.Type, "message", event.Message, "session_id", sessionID)

				// For process exit events, we might want to notify the user
				if event.Type == gamev2.PTYEventType_PTY_EVENT_PROCESS_EXIT {
					channel.Write([]byte("\r\n\r\nGame session ended.\r\n"))
					done <- io.EOF
					return
				}

			case *gamev2.GameIOResponse_Disconnected:
				// PTY disconnected
				h.logger.Info("PTY disconnected", "session_id", sessionID)
				done <- io.EOF
				return

			default:
				h.logger.Warn("Unknown gRPC response type", "type", fmt.Sprintf("%T", respType), "session_id", sessionID)
			}
		}
	}()

	// Wait for either goroutine to finish
	err = <-done
	if err != nil && err != io.EOF {
		h.logger.Error("Game I/O error", "error", err, "session_id", sessionID)
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

	h.logger.Info("Game I/O handling ended", "session_id", sessionID, "connection_id", connID)
}

// StartGameSession starts a game session
func (h *GameIOHandler) StartGameSession(ctx context.Context, channel ssh.Channel, userInfo *authv1.User, connID, username string, terminalCols, terminalRows int) error {
	// For now, start a NetHack session as default
	gameID := "nethack"

	// Start game session via Game Service
	// Convert string ID to int32 for the Game Service API
	userID, err := strconv.ParseInt(userInfo.Id, 10, 32)
	if err != nil {
		h.logger.Error("Invalid user ID format", "user_id", userInfo.Id, "error", err)
		channel.Write([]byte("Invalid user ID. Please contact administrator.\r\n"))
		return nil
	}
	sessionInfo, err := h.gameClient.StartGameSession(ctx, int32(userID), userInfo.Username, gameID, terminalCols, terminalRows)
	if err != nil {
		h.logger.Error("Failed to start game session", "error", err, "username", userInfo.Username)
		// Check if the error is due to game service unavailability
		if !h.gameClient.IsHealthy(ctx) {
			h.logger.Info("Game service became unavailable, entering idle mode", "username", userInfo.Username)
			return fmt.Errorf("game service unavailable")
		}
		channel.Write([]byte("Failed to start game session\r\n"))
		return nil // Return to menu
	}

	// Successfully started game session
	sessionID := sessionInfo.ID
	h.logger.Info("Started game session", "session_id", sessionID, "user", userInfo.Username, "game", gameID)

	// Handle I/O - since Game Service doesn't have direct I/O methods,
	// we'll need to implement this differently in a real implementation
	h.HandleGameIO(ctx, channel, sessionID, connID)

	return nil
}

// StartSpecificGameSession starts a game session with a specific game ID
func (h *GameIOHandler) StartSpecificGameSession(ctx context.Context, channel ssh.Channel, userInfo *authv1.User, connID, username, gameID string, terminalCols, terminalRows int) error {
	// Convert string ID to int32 for the Game Service API
	userID, err := strconv.ParseInt(userInfo.Id, 10, 32)
	if err != nil {
		h.logger.Error("Invalid user ID format", "user_id", userInfo.Id, "error", err)
		channel.Write([]byte("Invalid user ID. Please contact administrator.\r\n"))
		return nil
	}

	// Create gRPC stream FIRST to avoid race condition
	stream, err := h.gameClient.StreamGameIO(ctx)
	if err != nil {
		h.logger.Error("Failed to create game I/O stream", "error", err, "username", username)
		channel.Write([]byte("Failed to connect to game session\r\n"))
		return nil
	}
	defer stream.CloseSend()

	// Now start the game session with PTY
	sessionInfo, err := h.gameClient.StartGameSession(ctx, int32(userID), userInfo.Username, gameID, terminalCols, terminalRows)
	if err != nil {
		h.logger.Error("Failed to start game session", "error", err, "username", userInfo.Username, "game_id", gameID)
		// Check if the error is due to game service unavailability
		if !h.gameClient.IsHealthy(ctx) {
			h.logger.Info("Game service became unavailable, entering idle mode", "username", userInfo.Username)
			return fmt.Errorf("game service unavailable")
		}
		channel.Write([]byte("Failed to start game session\r\n"))
		return nil // Return to menu
	}

	// Successfully started game session
	sessionID := sessionInfo.ID
	h.logger.Info("Started game session", "session_id", sessionID, "user", userInfo.Username, "game", gameID)

	// Handle I/O using the pre-established stream
	h.HandleGameIOWithStream(ctx, channel, sessionID, connID, stream)

	return nil
}

// ParsePTYRequest parses PTY request payload
func (h *GameIOHandler) ParsePTYRequest(payload []byte) (int, int) {
	// PTY request format: term_name (string) + width (uint32) + height (uint32) + ...
	if len(payload) < 8 {
		return 80, 24
	}

	// Skip term name (4 bytes length + string)
	if len(payload) < 4 {
		return 80, 24
	}

	termNameLen := int(payload[3])
	if len(payload) < 4+termNameLen+8 {
		return 80, 24
	}

	offset := 4 + termNameLen
	cols := int(payload[offset])<<24 | int(payload[offset+1])<<16 | int(payload[offset+2])<<8 | int(payload[offset+3])
	rows := int(payload[offset+4])<<24 | int(payload[offset+5])<<16 | int(payload[offset+6])<<8 | int(payload[offset+7])

	return cols, rows
}

// ParseWindowChange parses window change request payload
func (h *GameIOHandler) ParseWindowChange(payload []byte) (int, int) {
	if len(payload) < 8 {
		return 80, 24
	}

	cols := int(payload[0])<<24 | int(payload[1])<<16 | int(payload[2])<<8 | int(payload[3])
	rows := int(payload[4])<<24 | int(payload[5])<<16 | int(payload[6])<<8 | int(payload[7])

	return cols, rows
}

// ResizeTerminal sends a resize request to the game service
func (h *GameIOHandler) ResizeTerminal(ctx context.Context, sessionID string, cols, rows int) error {
	return h.gameClient.ResizeTerminal(ctx, sessionID, cols, rows)
}
