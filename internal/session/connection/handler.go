package connection

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
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
	defer h.manager.UnregisterConnection(connID, conn.RemoteAddr())

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
	h.handleChannels(ctx, chans, connID, sshConn)
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
func (h *Handler) handleChannels(ctx context.Context, chans <-chan ssh.NewChannel, connID string, sshConn *ssh.ServerConn) {
	for newChannel := range chans {
		h.logger.Debug("Received SSH channel", "type", newChannel.ChannelType(), "connection_id", connID)

		switch newChannel.ChannelType() {
		case "session":
			go h.handleSessionChannel(ctx, newChannel, connID, sshConn)
		default:
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
		}
	}
}

// handleSessionChannel handles SSH session channels
func (h *Handler) handleSessionChannel(ctx context.Context, newChannel ssh.NewChannel, connID string, sshConn *ssh.ServerConn) {
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
			// For anonymous connections (no password auth), userInfo will be nil
			// This allows both anonymous and authenticated workflows
			userInfo, err := h.getUserInfo(ctx, sshConn)
			if err != nil {
				h.logger.Debug("No authenticated user info available, treating as anonymous", "error", err, "username", sshConn.User())
				userInfo = nil // Treat as anonymous user
			}

			// Main menu loop
			for {
				// Check if game service is available
				if !h.gameClient.IsHealthy(ctx) {
					h.logger.Info("Game service unavailable, entering idle mode", "username", sshConn.User())
					h.handleIdleMode(ctx, channel, connID, sshConn.User())
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
					menuChoice, err = h.menuHandler.ShowAnonymousMenu(ctx, channel, sshConn.User())
				} else {
					// Show authenticated user menu
					menuChoice, err = h.menuHandler.ShowUserMenu(ctx, channel, sshConn.User())
				}

				if err != nil {
					h.logger.Error("Error in menu handler", "error", err, "username", sshConn.User())
					if ctx.Err() != nil {
						return // Context cancelled
					}
					continue // Redisplay menu
				}

				// Handle menu choice
				if err := h.handleMenuChoice(ctx, channel, menuChoice, userInfo, connID, sshConn.User(), terminalCols, terminalRows); err != nil {
					h.logger.Error("Error handling menu choice", "error", err, "choice", menuChoice.Action, "username", sshConn.User())
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
func (h *Handler) getUserInfo(ctx context.Context, sshConn *ssh.ServerConn) (*authv1.User, error) {
	// Get the access token from SSH permissions (set during authentication)
	permissions := sshConn.Permissions
	if permissions == nil || permissions.Extensions == nil {
		return nil, fmt.Errorf("no authentication token available")
	}
	
	accessToken, ok := permissions.Extensions["access_token"]
	if !ok || accessToken == "" {
		return nil, fmt.Errorf("no access token in session")
	}
	
	// Validate token with auth service
	resp, err := h.authClient.GetUserInfo(ctx, accessToken)
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
	var username string
	if conn != nil {
		username = conn.User()
	}
	a.logger.Debug("Public key authentication attempted", "username", username)
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
		// Show game selection menu for authenticated users
		if userInfo != nil {
			return h.handleGameSelection(ctx, channel, userInfo, connID, username, terminalCols, terminalRows)
		} else {
			channel.Write([]byte("Please login first to play games.\r\n"))
			channel.Write([]byte("Press any key to continue...\r\n"))
			buffer := make([]byte, 1)
			channel.Read(buffer)
			return nil
		}

	case "login":
		return h.handleLogin(ctx, channel, connID, username)

	case "register":
		return h.handleRegister(ctx, channel, connID, username)

	case "start_game":
		// Start a specific game session with the selected game ID
		if userInfo != nil {
			return h.startSpecificGameSession(ctx, channel, userInfo, connID, username, choice.Value, terminalCols, terminalRows)
		} else {
			channel.Write([]byte("Please login first to play games.\r\n"))
			channel.Write([]byte("Press any key to continue...\r\n"))
			buffer := make([]byte, 1)
			channel.Read(buffer)
			return nil
		}

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
	userID, err := strconv.ParseInt(userInfo.Id, 10, 32)
	if err != nil {
		h.logger.Error("Invalid user ID format", "user_id", userInfo.Id, "error", err)
		channel.Write([]byte("Invalid user ID. Please contact administrator.\r\n"))
		return nil
	}
	sessionInfo, err := h.gameClient.StartGameSession(ctx, int32(userID), username, gameID, terminalCols, terminalRows)
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

// handleLogin handles the login process
func (h *Handler) handleLogin(ctx context.Context, channel ssh.Channel, connID, currentUsername string) error {
	channel.Write([]byte("\r\n=== Login ===\r\n"))
	
	// Flush any pending input from menu selection
	h.flushInput(channel)
	
	// Get username
	channel.Write([]byte("Username: "))
	username, err := h.readLine(channel)
	if err != nil {
		return err
	}
	
	// Get password (hidden input)
	channel.Write([]byte("Password: "))
	password, err := h.readLine(channel)
	if err != nil {
		return err
	}
	
	// Attempt login with auth service
	resp, err := h.authClient.Login(ctx, username, password)
	if err != nil {
		h.logger.Warn("Login failed", "username", username, "error", err)
		channel.Write([]byte("\r\nLogin failed. Please check your credentials.\r\n"))
		channel.Write([]byte("Press any key to continue...\r\n"))
		buffer := make([]byte, 1)
		channel.Read(buffer)
		return nil
	}
	
	// Login successful
	h.logger.Info("User logged in successfully", "username", username, "user_id", resp.User.Id)
	channel.Write([]byte("\r\nLogin successful! Welcome back, " + resp.User.Username + "!\r\n"))
	
	// Update connection state
	h.manager.UpdateConnectionState(connID, types.ConnectionStateAuthenticated, resp.User.Id)
	
	channel.Write([]byte("Press any key to continue...\r\n"))
	buffer := make([]byte, 1)
	channel.Read(buffer)
	
	return nil
}

// handleRegister handles the registration process
func (h *Handler) handleRegister(ctx context.Context, channel ssh.Channel, connID, currentUsername string) error {
	channel.Write([]byte("\r\n=== Registration ===\r\n"))
	
	// Flush any pending input from menu selection
	h.flushInput(channel)
	
	// Get username
	channel.Write([]byte("Choose a username: "))
	username, err := h.readLine(channel)
	if err != nil {
		return err
	}
	// Get password
	channel.Write([]byte("Choose a password: "))
	password, err := h.readLine(channel)
	if err != nil {
		return err
	}
	
	// Get email (optional)
	channel.Write([]byte("Email (optional): "))
	email, err := h.readLine(channel)
	if err != nil {
		return err
	}
	
	// Attempt registration with auth service
	resp, err := h.authClient.Register(ctx, username, password, email)
	if err != nil {
		h.logger.Warn("Registration failed", "username", username, "error", err)
		channel.Write([]byte("\r\nRegistration failed. Please try again later.\r\n"))
		channel.Write([]byte("Press any key to continue...\r\n"))
		buffer := make([]byte, 1)
		channel.Read(buffer)
		return nil
	}
	
	if !resp.Success {
		h.logger.Warn("Registration rejected", "username", username, "error", resp.Error, "error_code", resp.ErrorCode)
		channel.Write([]byte("\r\nRegistration failed: " + resp.Error + "\r\n"))
		channel.Write([]byte("Press any key to continue...\r\n"))
		buffer := make([]byte, 1)
		channel.Read(buffer)
		return nil
	}
	
	// Registration successful
	h.logger.Info("User registered successfully", "username", username, "user_id", resp.User.Id)
	channel.Write([]byte("\r\nRegistration successful! Welcome, " + resp.User.Username + "!\r\n"))
	channel.Write([]byte("You are now logged in.\r\n"))
	
	// Update connection state
	h.manager.UpdateConnectionState(connID, types.ConnectionStateAuthenticated, resp.User.Id)
	
	channel.Write([]byte("Press any key to continue...\r\n"))
	buffer := make([]byte, 1)
	channel.Read(buffer)
	
	return nil
}

// handleGameSelection shows the game selection menu and handles the choice
func (h *Handler) handleGameSelection(ctx context.Context, channel ssh.Channel, userInfo *authv1.User, connID, username string, terminalCols, terminalRows int) error {
	choice, err := h.menuHandler.ShowGameSelectionMenu(ctx, channel, username)
	if err != nil {
		h.logger.Error("Game selection menu failed", "error", err, "username", username)
		channel.Write([]byte("Failed to display game selection menu.\r\n"))
		return nil
	}

	// If choice is nil, user chose to go back to main menu
	if choice == nil {
		return nil
	}

	// Handle the game selection choice
	return h.handleMenuChoice(ctx, channel, choice, userInfo, connID, username, terminalCols, terminalRows)
}

// startSpecificGameSession starts a game session with a specific game ID
func (h *Handler) startSpecificGameSession(ctx context.Context, channel ssh.Channel, userInfo *authv1.User, connID, username, gameID string, terminalCols, terminalRows int) error {
	// Convert string ID to int32 for the API
	userID, err := strconv.ParseInt(userInfo.Id, 10, 32)
	if err != nil {
		h.logger.Error("Invalid user ID format", "user_id", userInfo.Id, "error", err)
		channel.Write([]byte("Invalid user ID. Please contact administrator.\r\n"))
		return nil
	}

	sessionInfo, err := h.gameClient.StartGameSession(ctx, int32(userID), username, gameID, terminalCols, terminalRows)
	if err != nil {
		h.logger.Error("Failed to start game session", "error", err, "username", username, "game_id", gameID)
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

// flushInput drains any pending input from the channel
func (h *Handler) flushInput(channel ssh.Channel) {
	// Simple approach: try to read one character and discard it if it's a newline
	buffer := make([]byte, 1)
	n, err := channel.Read(buffer)
	if err == nil && n > 0 && (buffer[0] == '\r' || buffer[0] == '\n') {
		// Consumed a pending newline, which is what we wanted
		return
	}
	// If we read something else, we can't put it back, but this should be rare
}

// readLine reads a line of input from the SSH channel, skipping empty lines
func (h *Handler) readLine(channel ssh.Channel) (string, error) {
	for {
		var line []byte
		buffer := make([]byte, 1)
		
		for {
			n, err := channel.Read(buffer)
			if err != nil {
				return "", err
			}
			
			if n > 0 {
				char := buffer[0]
				if char == '\r' || char == '\n' {
					break
				}
				line = append(line, char)
			}
		}
		
		// If we got a non-empty line, return it
		if len(line) > 0 {
			return string(line), nil
		}
		// Otherwise, continue reading the next line
	}
}
