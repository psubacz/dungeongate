package connection

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/dungeongate/internal/session/client"
	"github.com/dungeongate/internal/session/menu"
	"github.com/dungeongate/internal/session/types"
	authv1 "github.com/dungeongate/pkg/api/auth/v1"
	gamev2 "github.com/dungeongate/pkg/api/games/v2"
	"golang.org/x/crypto/ssh"
)

// Handler handles SSH connections in a stateless manner
type Handler struct {
	manager     *Manager
	gameClient  *client.GameClient
	authClient  *client.AuthClient
	menuHandler *menu.MenuHandler
	logger      *slog.Logger
}

// NewHandler creates a new connection handler
func NewHandler(manager *Manager, gameClient *client.GameClient, authClient *client.AuthClient, menuHandler *menu.MenuHandler, logger *slog.Logger) *Handler {
	return &Handler{
		manager:     manager,
		gameClient:  gameClient,
		authClient:  authClient,
		menuHandler: menuHandler,
		logger:      logger,
	}
}

// HandleConnection handles an SSH connection
func (h *Handler) HandleConnection(ctx context.Context, conn net.Conn, config *ssh.ServerConfig) {
	defer conn.Close()

	// Register connection
	connID := h.manager.RegisterConnection(conn)
	if connID == "" {
		h.logger.Warn("Failed to register connection", "remote_addr", conn.RemoteAddr())
		return
	}
	defer h.manager.UnregisterConnection(connID)

	// Perform SSH handshake
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		h.logger.Error("Failed SSH handshake", "error", err, "connection_id", connID)
		return
	}
	defer sshConn.Close()

	// Update connection state after successful handshake
	h.manager.UpdateConnectionState(connID, types.ConnectionStateAuthenticated, sshConn.User())

	h.logger.Info("SSH connection established", "connection_id", connID, "user", sshConn.User())

	// Handle SSH channels and requests
	go h.handleRequests(ctx, reqs, connID)
	h.handleChannels(ctx, chans, connID, sshConn.User())
}

// handleRequests handles SSH requests
func (h *Handler) handleRequests(ctx context.Context, reqs <-chan *ssh.Request, connID string) {
	for req := range reqs {
		h.logger.Debug("Received SSH request", "type", req.Type, "connection_id", connID)

		switch req.Type {
		case "keepalive":
			// Respond to keepalive
			if req.WantReply {
				req.Reply(true, nil)
			}
		default:
			// Reject unknown requests
			if req.WantReply {
				req.Reply(false, nil)
			}
		}
	}
}

// handleChannels handles SSH channels
func (h *Handler) handleChannels(ctx context.Context, chans <-chan ssh.NewChannel, connID, username string) {
	for newChannel := range chans {
		h.logger.Debug("Received SSH channel", "type", newChannel.ChannelType(), "connection_id", connID)

		switch newChannel.ChannelType() {
		case "session":
			go h.handleSessionChannel(ctx, newChannel, connID, username)
		default:
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
		}
	}
}

// handleSessionChannel handles SSH session channels
func (h *Handler) handleSessionChannel(ctx context.Context, newChannel ssh.NewChannel, connID, username string) {
	channel, requests, err := newChannel.Accept()
	if err != nil {
		h.logger.Error("Failed to accept channel", "error", err, "connection_id", connID)
		return
	}
	defer channel.Close()

	// Handle session requests
	var sessionID string
	var terminalCols, terminalRows int = 80, 24

	for req := range requests {
		switch req.Type {
		case "pty-req":
			// Parse PTY request
			if len(req.Payload) > 0 {
				terminalCols, terminalRows = h.parsePTYRequest(req.Payload)
			}
			req.Reply(true, nil)

		case "shell":
			// Start shell session
			req.Reply(true, nil)

			// Get user info from auth service
			userInfo, err := h.getUserInfo(ctx, username)
			if err != nil {
				h.logger.Error("Failed to get user info", "error", err, "username", username)
				channel.Write([]byte("Authentication failed\r\n"))
				return
			}

			// Main menu loop
			for {
				// Check if game service is available
				if !h.gameClient.IsHealthy(ctx) {
					h.logger.Info("Game service unavailable, entering idle mode", "username", username)
					h.handleIdleMode(ctx, channel, connID, username)
					// If idle mode returns, user either pressed 'r' to retry or 'q' to quit
					// If context is done, user quit, so we return
					if ctx.Err() != nil {
						return
					}
					// Otherwise, continue the loop to retry
					continue
				}

				// Show main menu (anonymous or authenticated)
				var menuChoice *menu.MenuChoice
				if userInfo == nil || userInfo.Id == "" {
					// Show anonymous menu
					menuChoice, err = h.menuHandler.ShowAnonymousMenu(ctx, channel, username)
				} else {
					// Show authenticated user menu
					menuChoice, err = h.menuHandler.ShowUserMenu(ctx, channel, username)
				}

				if err != nil {
					h.logger.Error("Error in menu handler", "error", err, "username", username)
					if ctx.Err() != nil {
						return // Context cancelled
					}
					continue // Redisplay menu
				}

				// Handle menu choice
				if err := h.handleMenuChoice(ctx, channel, menuChoice, userInfo, connID, username, terminalCols, terminalRows); err != nil {
					h.logger.Error("Error handling menu choice", "error", err, "choice", menuChoice.Action, "username", username)
					if ctx.Err() != nil {
						return // Context cancelled
					}
					continue // Return to menu
				}

				// If we get here, the menu choice was handled successfully
				// Some choices (like quit) will have returned from the function
				// Others (like play) will have started a game session
				break
			}

		case "window-change":
			// Handle terminal resize
			if sessionID != "" && len(req.Payload) > 0 {
				cols, rows := h.parseWindowChange(req.Payload)
				h.logger.Debug("Terminal resize", "session_id", sessionID, "cols", cols, "rows", rows)
				
				// Send resize request to Game Service
				if err := h.gameClient.ResizeTerminal(ctx, sessionID, cols, rows); err != nil {
					h.logger.Error("Failed to resize terminal", "error", err, "session_id", sessionID)
				}
			}
			req.Reply(true, nil)

		default:
			// Reject unknown requests
			req.Reply(false, nil)
		}
	}

	// Clean up session when channel closes
	if sessionID != "" {
		err := h.gameClient.StopGameSession(ctx, sessionID, "connection_closed")
		if err != nil {
			h.logger.Error("Failed to stop game session", "error", err, "session_id", sessionID)
		}
	}
}

// getUserInfo retrieves user information from the auth service
func (h *Handler) getUserInfo(ctx context.Context, username string) (*authv1.User, error) {
	// In a real implementation, we'd have a token from the SSH authentication
	// For now, we'll mock this by calling GetUserInfo with a placeholder
	resp, err := h.authClient.GetUserInfo(ctx, "mock_token")
	if err != nil {
		return nil, err
	}
	return resp.User, nil
}

// handleGameIO handles I/O between SSH channel and game session via gRPC streaming
func (h *Handler) handleGameIO(ctx context.Context, channel ssh.Channel, sessionID, connID string) {
	h.logger.Info("Starting game I/O handling", "session_id", sessionID, "connection_id", connID)

	// Create gRPC stream to Game Service
	stream, err := h.gameClient.StreamGameIO(ctx)
	if err != nil {
		h.logger.Error("Failed to create game I/O stream", "error", err, "session_id", sessionID)
		channel.Write([]byte("Failed to connect to game session\r\n"))
		return
	}
	defer stream.CloseSend()

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
				done <- err
				return
			}

			// Handle different response types
			switch respType := resp.Response.(type) {
			case *gamev2.GameIOResponse_Output:
				// Forward output to SSH channel
				if _, err := channel.Write(respType.Output.Data); err != nil {
					h.logger.Error("Failed to write to SSH channel", "error", err, "session_id", sessionID)
					done <- err
					return
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

// parsePTYRequest parses PTY request payload
func (h *Handler) parsePTYRequest(payload []byte) (int, int) {
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

// parseWindowChange parses window change request payload
func (h *Handler) parseWindowChange(payload []byte) (int, int) {
	if len(payload) < 8 {
		return 80, 24
	}

	cols := int(payload[0])<<24 | int(payload[1])<<16 | int(payload[2])<<8 | int(payload[3])
	rows := int(payload[4])<<24 | int(payload[5])<<16 | int(payload[6])<<8 | int(payload[7])

	return cols, rows
}

// AuthHandler provides SSH authentication
type AuthHandler struct {
	authClient *client.AuthClient
	logger     *slog.Logger
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(authClient *client.AuthClient, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		authClient: authClient,
		logger:     logger,
	}
}

// PasswordCallback handles password authentication
func (a *AuthHandler) PasswordCallback(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
	username := conn.User()

	// Authenticate with auth service
	ctx := context.Background()
	resp, err := a.authClient.Login(ctx, username, string(password))
	if err != nil {
		a.logger.Warn("Login failed", "username", username, "error", err)
		return nil, fmt.Errorf("authentication failed")
	}

	// Validate response
	if resp == nil {
		a.logger.Warn("Login failed: empty response", "username", username)
		return nil, fmt.Errorf("authentication failed")
	}
	if resp.User == nil {
		a.logger.Warn("Login failed: empty user in response", "username", username)
		return nil, fmt.Errorf("authentication failed")
	}

	// Store user info in permissions
	permissions := &ssh.Permissions{
		Extensions: map[string]string{
			"user_id":      resp.User.Id,
			"username":     resp.User.Username,
			"access_token": resp.AccessToken,
		},
	}

	a.logger.Info("Authentication successful", "username", username, "user_id", resp.User.Id)
	return permissions, nil
}

// PublicKeyCallback handles public key authentication
func (a *AuthHandler) PublicKeyCallback(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	// For now, reject public key authentication
	// In a real implementation, we'd validate the key against stored public keys
	a.logger.Debug("Public key authentication attempted", "username", conn.User())
	return nil, fmt.Errorf("public key authentication not supported")
}

// handleIdleMode handles the idle state when game service is unavailable
func (h *Handler) handleIdleMode(ctx context.Context, channel ssh.Channel, connID, username string) {
	// Display idle banner with status message
	banner := fmt.Sprintf("\r\n=== DungeonGate Terminal ===\r\n")
	banner += fmt.Sprintf("Connected as: %s\r\n", username)
	banner += fmt.Sprintf("Status: Idle\r\n")
	banner += fmt.Sprintf("\r\n")
	banner += fmt.Sprintf("┌─────────────────────────────────────────────────────┐\r\n")
	banner += fmt.Sprintf("│                  SERVICE NOTICE                     │\r\n")
	banner += fmt.Sprintf("├─────────────────────────────────────────────────────┤\r\n")
	banner += fmt.Sprintf("│ No games are currently available.                   │\r\n")
	banner += fmt.Sprintf("│ The game service is temporarily unavailable.       │\r\n")
	banner += fmt.Sprintf("│                                                     │\r\n")
	banner += fmt.Sprintf("│ Please try again later or contact an administrator │\r\n")
	banner += fmt.Sprintf("│ if this problem persists.                          │\r\n")
	banner += fmt.Sprintf("└─────────────────────────────────────────────────────┘\r\n")
	banner += fmt.Sprintf("\r\n")
	banner += fmt.Sprintf("Commands:\r\n")
	banner += fmt.Sprintf("  [r] - Retry connecting to game service\r\n")
	banner += fmt.Sprintf("  [q] - Quit\r\n")
	banner += fmt.Sprintf("\r\n")
	banner += fmt.Sprintf("Choice: ")
	
	channel.Write([]byte(banner))
	
	// Set up periodic health checks
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	// Handle user input and periodic health checks
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Periodic health check
			if h.gameClient.IsHealthy(ctx) {
				h.logger.Info("Game service became available, notifying user", "username", username)
				channel.Write([]byte("\r\n\r\n✓ Game service is now available! Press 'r' to retry or 'q' to quit.\r\nChoice: "))
			}
		default:
			// Check for user input
			buffer := make([]byte, 1)
			// Use a simple blocking read with timeout via goroutine
			readChan := make(chan struct{})
			var n int
			var err error
			
			go func() {
				n, err = channel.Read(buffer)
				close(readChan)
			}()
			
			select {
			case <-readChan:
				if err != nil {
					if err == io.EOF {
						return // Connection closed
					}
					h.logger.Debug("Error reading from channel in idle mode", "error", err, "username", username)
					return
				}
				
				if n > 0 {
					input := string(buffer[:n])
					switch strings.ToLower(input) {
					case "r":
						if h.gameClient.IsHealthy(ctx) {
							h.logger.Info("Game service available, retrying game start", "username", username)
							channel.Write([]byte("\r\n\r\n✓ Game service is available! Starting game...\r\n"))
							// Return to main handler to retry game start
							return
						} else {
							channel.Write([]byte("\r\n\r\n✗ Game service is still unavailable. Please try again later.\r\nChoice: "))
						}
					case "q":
						channel.Write([]byte("\r\n\r\nGoodbye!\r\n"))
						return
					default:
						channel.Write([]byte("\r\n\r\nInvalid option. Use 'r' to retry or 'q' to quit.\r\nChoice: "))
					}
				}
			case <-time.After(100 * time.Millisecond):
				// Timeout, continue loop
				continue
			}
		}
	}
}

// handleMenuChoice handles the user's menu choice
func (h *Handler) handleMenuChoice(ctx context.Context, channel ssh.Channel, choice *menu.MenuChoice, userInfo *authv1.User, connID, username string, terminalCols, terminalRows int) error {
	switch choice.Action {
	case "quit":
		channel.Write([]byte("Goodbye!\r\n"))
		return fmt.Errorf("user quit") // This will exit the session

	case "play":
		// Start a game session
		return h.startGameSession(ctx, channel, userInfo, connID, username, terminalCols, terminalRows)

	case "login":
		channel.Write([]byte("Login functionality not yet implemented.\r\n"))
		channel.Write([]byte("Press any key to continue...\r\n"))
		buffer := make([]byte, 1)
		channel.Read(buffer)
		return nil

	case "register":
		channel.Write([]byte("Registration functionality not yet implemented.\r\n"))
		channel.Write([]byte("Press any key to continue...\r\n"))
		buffer := make([]byte, 1)
		channel.Read(buffer)
		return nil

	case "watch":
		channel.Write([]byte("Spectating functionality not yet implemented.\r\n"))
		channel.Write([]byte("Press any key to continue...\r\n"))
		buffer := make([]byte, 1)
		channel.Read(buffer)
		return nil

	case "list_games":
		channel.Write([]byte("Game listing functionality not yet implemented.\r\n"))
		channel.Write([]byte("Press any key to continue...\r\n"))
		buffer := make([]byte, 1)
		channel.Read(buffer)
		return nil

	case "edit_profile":
		channel.Write([]byte("Profile editing functionality not yet implemented.\r\n"))
		channel.Write([]byte("Press any key to continue...\r\n"))
		buffer := make([]byte, 1)
		channel.Read(buffer)
		return nil

	case "view_recordings":
		channel.Write([]byte("Recording viewing functionality not yet implemented.\r\n"))
		channel.Write([]byte("Press any key to continue...\r\n"))
		buffer := make([]byte, 1)
		channel.Read(buffer)
		return nil

	case "statistics":
		channel.Write([]byte("Statistics functionality not yet implemented.\r\n"))
		channel.Write([]byte("Press any key to continue...\r\n"))
		buffer := make([]byte, 1)
		channel.Read(buffer)
		return nil

	default:
		channel.Write([]byte(fmt.Sprintf("Unknown action: %s\r\n", choice.Action)))
		channel.Write([]byte("Press any key to continue...\r\n"))
		buffer := make([]byte, 1)
		channel.Read(buffer)
		return nil
	}
}

// startGameSession starts a game session
func (h *Handler) startGameSession(ctx context.Context, channel ssh.Channel, userInfo *authv1.User, connID, username string, terminalCols, terminalRows int) error {
	// For now, start a NetHack session as default
	gameID := "nethack"

	// Start game session via Game Service
	// Convert string ID to int32 for the API
	// TODO: Properly parse userInfo.Id string to int32
	userID := int32(1)
	_ = userInfo // TODO: Use userInfo.Id for actual user ID
	sessionInfo, err := h.gameClient.StartGameSession(ctx, userID, username, gameID, terminalCols, terminalRows)
	if err != nil {
		h.logger.Error("Failed to start game session", "error", err, "username", username)
		// Check if the error is due to game service unavailability
		if !h.gameClient.IsHealthy(ctx) {
			h.logger.Info("Game service became unavailable, entering idle mode", "username", username)
			h.handleIdleMode(ctx, channel, connID, username)
			return nil // Return to menu
		}
		channel.Write([]byte("Failed to start game session\r\n"))
		return nil // Return to menu
	}

	// Successfully started game session
	sessionID := sessionInfo.ID
	h.logger.Info("Started game session", "session_id", sessionID, "user", username, "game", gameID)

	// Update connection state
	h.manager.UpdateConnectionState(connID, types.ConnectionStateActive, username)

	// Handle I/O - since Game Service doesn't have direct I/O methods,
	// we'll need to implement this differently in a real implementation
	h.handleGameIO(ctx, channel, sessionID, connID)

	return nil
}
