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
	"github.com/dungeongate/internal/session/terminal"
	authv1 "github.com/dungeongate/pkg/api/auth/v1"
	gamev2 "github.com/dungeongate/pkg/api/games/v2"
	"golang.org/x/crypto/ssh"
)

// SpectatingHandler handles spectator functionality and session watching
type SpectatingHandler struct {
	gameClient *client.GameClient
	logger     *slog.Logger
}

// NewSpectatingHandler creates a new spectating handler
func NewSpectatingHandler(gameClient *client.GameClient, logger *slog.Logger) *SpectatingHandler {
	return &SpectatingHandler{
		gameClient: gameClient,
		logger:     logger,
	}
}

// HandleWatchMode handles the spectating/watching functionality
func (h *SpectatingHandler) HandleWatchMode(ctx context.Context, channel ssh.Channel, user *authv1.User) error {
	if user != nil {
		h.logger.Info("Entering watch mode", "user_id", user.Id, "username", user.Username)
	} else {
		h.logger.Info("Entering watch mode (anonymous user)")
	}

	// Get active sessions available for spectating
	sessions, err := h.gameClient.GetActiveGameSessions(ctx)
	if err != nil {
		h.logger.Error("Failed to get active sessions", "error", err)
		channel.Write([]byte("Failed to get active sessions. Please try again later.\r\n"))
		time.Sleep(2 * time.Second)
		return nil
	}

	// Filter out user's own sessions (authenticated users only)
	availableSessions := make([]*gamev2.GameSession, 0)
	if user != nil {
		// For authenticated users, filter out their own sessions
		userID, err := strconv.ParseInt(user.Id, 10, 32)
		if err != nil {
			h.logger.Error("Invalid user ID format", "user_id", user.Id, "error", err)
			channel.Write([]byte("Invalid user ID. Please contact administrator.\r\n"))
			time.Sleep(2 * time.Second)
			return nil
		}

		for _, session := range sessions {
			if session.UserId != int32(userID) {
				availableSessions = append(availableSessions, session)
			}
		}
	} else {
		// For anonymous users, show all sessions
		availableSessions = sessions
	}

	if len(availableSessions) == 0 {
		channel.Write([]byte("No active sessions available for spectating.\r\n"))
		time.Sleep(2 * time.Second)
		return nil
	}

	// Clear screen and display available sessions
	channel.Write([]byte("\033[2J\033[H"))
	channel.Write([]byte("=== Active Sessions ===\r\n"))
	for i, session := range availableSessions {
		spectatorCount := len(session.Spectators)
		channel.Write([]byte(fmt.Sprintf("%d. %s playing %s (%d spectators)\r\n",
			i+1, session.Username, session.GameId, spectatorCount)))
	}
	channel.Write([]byte(fmt.Sprintf("\r\nSelect a session to spectate (1-%d) or 'q' to quit: ", len(availableSessions))))

	// Read user's choice
	choice, err := h.readLineWithTerminal(ctx, channel)
	if err != nil {
		return err
	}

	choice = strings.TrimSpace(strings.ToLower(choice))
	if choice == "q" || choice == "quit" {
		return nil
	}

	// Parse choice
	sessionIndex, err := strconv.Atoi(choice)
	if err != nil || sessionIndex < 1 || sessionIndex > len(availableSessions) {
		channel.Write([]byte("Invalid selection.\r\n"))
		time.Sleep(2 * time.Second)
		return nil
	}

	selectedSession := availableSessions[sessionIndex-1]

	// Start spectating the selected session
	return h.StartSpectating(ctx, channel, user, selectedSession.Id)
}

// StartSpectating starts spectating a game session by session ID
func (h *SpectatingHandler) StartSpectating(ctx context.Context, channel ssh.Channel, user *authv1.User, sessionID string) error {
	// First, get the session details
	session, err := h.gameClient.GetGameSessionWithSpectators(ctx, sessionID)
	if err != nil {
		h.logger.Error("Failed to get session details", "error", err, "session_id", sessionID)
		channel.Write([]byte("Failed to get session details. The session may have ended.\r\n"))
		time.Sleep(2 * time.Second)
		return nil
	}
	if user != nil {
		h.logger.Info("Starting spectating",
			"spectator_user_id", user.Id,
			"spectator_username", user.Username,
			"session_id", session.Id,
			"session_username", session.Username,
			"game_id", session.GameId)
	} else {
		h.logger.Info("Starting spectating (anonymous user)",
			"session_id", session.Id,
			"session_username", session.Username,
			"game_id", session.GameId)
	}

	// Add user as spectator to the session (authenticated users only)
	var userID int32 = 0
	var username string = "anonymous"

	if user != nil {
		// Convert user ID to int32 for Game Service API
		userIDParsed, err := strconv.ParseInt(user.Id, 10, 32)
		if err != nil {
			h.logger.Error("Invalid user ID format", "user_id", user.Id, "error", err)
			channel.Write([]byte("Invalid user ID. Please contact administrator.\r\n"))
			time.Sleep(2 * time.Second)
			return nil
		}
		userID = int32(userIDParsed)
		username = user.Username
	}

	// For authenticated users, add them as a spectator
	// For anonymous users, we'll skip this step and just watch without being tracked
	if user != nil {
		err := h.gameClient.AddSpectator(ctx, session.Id, userID, username)
		if err != nil {
			h.logger.Error("Failed to add spectator", "error", err)
			channel.Write([]byte("Failed to join as spectator. Please try again later.\r\n"))
			time.Sleep(2 * time.Second)
			return nil
		}
	}

	// Clear screen and show spectating banner
	channel.Write([]byte("\033[2J\033[H"))
	channel.Write([]byte(fmt.Sprintf("=== Spectating %s's game ===\r\n", session.Username)))
	channel.Write([]byte("Press 'q' to quit spectating\r\n"))
	channel.Write([]byte("Connecting to game stream...\r\n\r\n"))

	// Create game I/O stream
	stream, err := h.gameClient.StreamGameIO(ctx)
	if err != nil {
		h.logger.Error("Failed to create game I/O stream", "error", err)
		channel.Write([]byte("Failed to connect to game stream.\r\n"))
		// Clean up spectator (authenticated users only)
		if user != nil {
			h.gameClient.RemoveSpectator(ctx, session.Id, int32(userID))
		}
		time.Sleep(2 * time.Second)
		return nil
	}
	defer stream.CloseSend()

	// Send connect request
	connectReq := &gamev2.GameIORequest{
		Request: &gamev2.GameIORequest_Connect{
			Connect: &gamev2.ConnectPTYRequest{
				SessionId: session.Id,
				TerminalSize: &gamev2.TerminalSize{
					Width:  session.TerminalSize.Width, // Use actual game session terminal size
					Height: session.TerminalSize.Height,
				},
				TermType: "xterm",
			},
		},
	}

	if err := stream.Send(connectReq); err != nil {
		h.logger.Error("Failed to send connect request", "error", err)
		channel.Write([]byte("Failed to connect to game stream.\r\n"))
		// Clean up spectator (authenticated users only)
		if user != nil {
			h.gameClient.RemoveSpectator(ctx, session.Id, int32(userID))
		}
		time.Sleep(2 * time.Second)
		return nil
	}

	// Handle the spectating stream
	err = h.handleSpectatingStream(ctx, stream, channel, user, session)

	// Clean up spectator when done (authenticated users only)
	if user != nil {
		if removeErr := h.gameClient.RemoveSpectator(ctx, session.Id, int32(userID)); removeErr != nil {
			h.logger.Error("Failed to remove spectator", "error", removeErr)
		}
	}

	return err
}

// handleSpectatingStream handles the bidirectional stream for spectating
func (h *SpectatingHandler) handleSpectatingStream(ctx context.Context, stream gamev2.GameService_StreamGameIOClient, channel ssh.Channel, user *authv1.User, session *gamev2.GameSession) error {
	// Channel for communicating between goroutines
	done := make(chan error, 2)

	// Goroutine to read from game stream and write to SSH channel
	go func() {
		defer func() {
			done <- nil
		}()

		for {
			resp, err := stream.Recv()
			if err != nil {
				if err == io.EOF || strings.Contains(err.Error(), "context canceled") {
					h.logger.Info("Game stream closed", "session_id", session.Id)
					// Game ended - clear terminal and show message for spectators
					channel.Write([]byte("\033[2J\033[H")) // Clear screen and move cursor to home
					channel.Write([]byte("\r\n=== Game ended ===\r\n"))
					channel.Write([]byte("The game you were spectating has ended.\r\n"))
					channel.Write([]byte("Returning to main menu...\r\n\r\n"))
					time.Sleep(2 * time.Second)
					return
				}
				h.logger.Error("Failed to receive from game stream", "error", err)
				return
			}

			switch response := resp.Response.(type) {
			case *gamev2.GameIOResponse_Connected:
				if response.Connected.Success {
					h.logger.Info("Successfully connected to game stream", "session_id", session.Id)
				} else {
					h.logger.Error("Failed to connect to game stream", "error", response.Connected.Error)
					channel.Write([]byte("Failed to connect to game stream.\r\n"))
					return
				}

			case *gamev2.GameIOResponse_Output:
				// Write game output to SSH channel
				if _, err := channel.Write(response.Output.Data); err != nil {
					h.logger.Error("Failed to write to SSH channel", "error", err)
					return
				}

			case *gamev2.GameIOResponse_Event:
				// Handle game events
				switch response.Event.Type {
				case gamev2.PTYEventType_PTY_EVENT_PROCESS_EXIT:
					// Clear terminal and show clean message for spectators
					channel.Write([]byte("\033[2J\033[H")) // Clear screen and move cursor to home
					channel.Write([]byte("\r\n=== Game ended ===\r\n"))
					channel.Write([]byte("The game you were spectating has ended.\r\n"))
					channel.Write([]byte("Returning to main menu...\r\n\r\n"))
					time.Sleep(2 * time.Second)
					return
				case gamev2.PTYEventType_PTY_EVENT_SESSION_TERMINATED:
					// Clear terminal and show clean message for spectators
					channel.Write([]byte("\033[2J\033[H")) // Clear screen and move cursor to home
					channel.Write([]byte("\r\n=== Session terminated ===\r\n"))
					channel.Write([]byte("The session you were spectating has been terminated.\r\n"))
					channel.Write([]byte("Returning to main menu...\r\n\r\n"))
					time.Sleep(2 * time.Second)
					return
				}

			case *gamev2.GameIOResponse_Disconnected:
				h.logger.Info("Disconnected from game stream", "session_id", session.Id)
				return
			}
		}
	}()

	// Goroutine to read from SSH channel and handle spectator input
	go func() {
		defer func() {
			done <- nil
		}()

		buffer := make([]byte, 1024)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Read from SSH channel
				n, err := channel.Read(buffer)
				if err != nil {
					if err == io.EOF {
						return
					}
					h.logger.Error("Failed to read from SSH channel", "error", err)
					return
				}

				// Check for quit command
				input := string(buffer[:n])
				if strings.Contains(strings.ToLower(input), "q") {
					// Send disconnect request
					disconnectReq := &gamev2.GameIORequest{
						Request: &gamev2.GameIORequest_Disconnect{
							Disconnect: &gamev2.DisconnectPTYRequest{
								SessionId: session.Id,
								Reason:    "Spectator quit",
							},
						},
					}
					stream.Send(disconnectReq)
					return
				}

				// For spectators, we typically don't forward input to the game
				// Only the player should be able to control the game
				// But we could add special spectator commands here if needed
			}
		}
	}()

	// Wait for either goroutine to finish
	<-done
	return nil
}

// Terminal input helper methods
func (h *SpectatingHandler) readLineWithTerminal(ctx context.Context, channel ssh.Channel) (string, error) {
	editor := terminal.NewLineEditor(channel, terminal.InputTypeText)
	return editor.ReadLine(ctx)
}
