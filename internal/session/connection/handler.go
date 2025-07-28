package connection

import (
	"context"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/dungeongate/internal/session/client"
	"github.com/dungeongate/internal/session/menu"
	"golang.org/x/crypto/ssh"
)

// Handler handles SSH connections in a stateless manner
type Handler struct {
	manager              *Manager
	authManager          *UserAuthManager
	gameIOHandler        *GameIOHandler
	spectatingHandler    *SpectatingHandler
	menuChoiceProcessor  *MenuChoiceProcessor
	serviceHealthChecker *ServiceHealthChecker
	menuHandler          *menu.MenuHandler
	authHandler          *SSHAuthHandler
	logger               *slog.Logger
	idleRetryInterval    time.Duration
}

// NewHandler creates a new connection handler
func NewHandler(manager *Manager, gameClient *client.GameClient, authClient *client.AuthClient, menuHandler *menu.MenuHandler, logger *slog.Logger, idleRetryInterval time.Duration, authHandler *SSHAuthHandler) *Handler {
	// Create component handlers
	authManager := NewUserAuthManager(authClient, logger)
	gameIOHandler := NewGameIOHandler(gameClient, logger)
	spectatingHandler := NewSpectatingHandler(gameClient, logger)
	serviceHealthChecker := NewServiceHealthChecker(authClient, gameClient, menuHandler, logger)
	menuChoiceProcessor := NewMenuChoiceProcessor(authManager, gameIOHandler, spectatingHandler, menuHandler, logger)

	return &Handler{
		manager:              manager,
		authManager:          authManager,
		gameIOHandler:        gameIOHandler,
		spectatingHandler:    spectatingHandler,
		menuChoiceProcessor:  menuChoiceProcessor,
		serviceHealthChecker: serviceHealthChecker,
		menuHandler:          menuHandler,
		authHandler:          authHandler,
		logger:               logger,
		idleRetryInterval:    idleRetryInterval,
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
		case "env":
			// Handle environment variable setting
			success := h.handleEnvironmentRequest(req.Payload, connID)
			if req.WantReply {
				req.Reply(success, nil)
			}
		default:
			// Reject unknown requests
			if req.WantReply {
				req.Reply(false, nil)
			}
		}
	}
}

// handleEnvironmentRequest handles SSH environment variable requests
func (h *Handler) handleEnvironmentRequest(payload []byte, connID string) bool {
	if len(payload) < 8 {
		h.logger.Warn("Invalid environment request payload", "connection_id", connID, "payload_length", len(payload))
		return false
	}

	// Parse SSH env request format: 4 bytes name length + name + 4 bytes value length + value
	nameLen := int(payload[0])<<24 | int(payload[1])<<16 | int(payload[2])<<8 | int(payload[3])
	if len(payload) < 4+nameLen+4 {
		h.logger.Warn("Invalid environment request: insufficient data for name", "connection_id", connID)
		return false
	}

	name := string(payload[4 : 4+nameLen])
	
	valueStart := 4 + nameLen
	valueLen := int(payload[valueStart])<<24 | int(payload[valueStart+1])<<16 | int(payload[valueStart+2])<<8 | int(payload[valueStart+3])
	if len(payload) < valueStart+4+valueLen {
		h.logger.Warn("Invalid environment request: insufficient data for value", "connection_id", connID)
		return false
	}

	value := string(payload[valueStart+4 : valueStart+4+valueLen])

	h.logger.Debug("Environment variable received", "connection_id", connID, "name", name, "value_length", len(value))

	// Store the environment variable in the auth handler
	h.authHandler.SetEnvironmentVariable(name, value)

	return true
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
	defer func() {
		// Clear screen on exit
		channel.Write([]byte("\033[2J")) // Clear screen
		channel.Write([]byte("\033[H"))  // Move cursor to home
		channel.Close()
	}()

	// Handle session requests
	var sessionID string
	var terminalCols, terminalRows int = 80, 24

	for req := range requests {
		switch req.Type {
		case "pty-req":
			// Parse PTY request
			if len(req.Payload) > 0 {
				terminalCols, terminalRows = h.gameIOHandler.ParsePTYRequest(req.Payload)
			}
			req.Reply(true, nil)

		case "env":
			// Handle environment variable setting
			success := h.handleEnvironmentRequest(req.Payload, connID)
			if req.WantReply {
				req.Reply(success, nil)
			}

		case "shell":
			// Start shell session
			req.Reply(true, nil)

			// Clear the terminal
			channel.Write([]byte("\033[2J")) // Clear entire screen
			channel.Write([]byte("\033[H"))  // Move cursor to home position (1,1)

			// Check service health at connection time
			servicesHealthy, serviceStatus := h.serviceHealthChecker.CheckServiceHealth(ctx)
			if !servicesHealthy {
				h.logger.Warn("Required services unavailable at connection time", "username", sshConn.User(), "services", serviceStatus)
				err := h.serviceHealthChecker.HandleServiceUnavailable(ctx, channel, connID, sshConn.User())
				if err != nil && err.Error() == "user quit" {
					return // User quit or timeout
				}
				return // Service unavailable handled
			}

			// Get user info from auth service
			userInfo, err := h.authManager.GetUserInfo(ctx, sshConn)
			if err != nil {
				h.logger.Debug("No authenticated user info available, checking for DGAUTH", "error", err, "username", sshConn.User())
				
				// Try DGAUTH environment-based authentication if not already authenticated
				permissions, dgauthErr := h.authHandler.authenticateWithDGAUTH(ctx)
				if dgauthErr == nil && permissions != nil {
					// DGAUTH authentication successful, update SSH connection permissions
					if sshConn.Permissions == nil {
						sshConn.Permissions = permissions
					} else {
						// Merge permissions
						for k, v := range permissions.Extensions {
							sshConn.Permissions.Extensions[k] = v
						}
					}
					// Try to get user info again with the new auth
					userInfo, err = h.authManager.GetUserInfo(ctx, sshConn)
					if err != nil {
						h.logger.Warn("Failed to get user info after DGAUTH", "error", err)
						userInfo = nil
					}
				} else {
					h.logger.Debug("DGAUTH authentication not available or failed", "error", dgauthErr)
					userInfo = nil // Treat as anonymous user
				}
			}

			// Main menu loop
			for {
				// Refresh user info before showing menu (in case user just logged in)
				currentUserInfo, err := h.authManager.GetUserInfo(ctx, sshConn)
				if err == nil && currentUserInfo != nil {
					userInfo = currentUserInfo // Update user info if available
				}

				// Show main menu (anonymous or authenticated)
				var menuChoice *menu.MenuChoice
				if userInfo == nil || userInfo.Id == "" {
					// Show anonymous menu
					menuChoice, err = h.menuHandler.ShowAnonymousMenu(ctx, channel, sshConn.User())
				} else {
					// Check if user needs to change password (one-time password)
					if userInfo.Metadata != nil && userInfo.Metadata["require_password_change"] == "true" {
						// Force password change for one-time passwords
						menuChoice, err = h.authManager.HandleRequiredPasswordChange(ctx, channel, userInfo, sshConn)
						if err != nil {
							h.logger.Error("Error handling required password change", "error", err, "username", userInfo.Username)
							continue // Return to menu
						}
						if menuChoice != nil && menuChoice.Action == "quit" {
							return // User chose to quit
						}
						// Password changed successfully, refresh user info and continue
						continue
					}

					// Show authenticated user menu (or admin menu if user is admin)
					menuChoice, err = h.menuHandler.ShowUserMenu(ctx, channel, userInfo)
				}

				if err != nil {
					// Check if this is a graceful disconnection (EOF) or user quit
					if strings.Contains(err.Error(), "EOF") || err.Error() == "user quit" {
						return // Normal disconnect, don't log as error
					}
					h.logger.Error("Error in menu handler", "error", err, "username", sshConn.User())
					if ctx.Err() != nil {
						return // Context cancelled
					}
					continue // Redisplay menu
				}

				// Handle menu choice
				if err := h.menuChoiceProcessor.HandleMenuChoice(ctx, channel, menuChoice, userInfo, connID, sshConn.User(), terminalCols, terminalRows, sshConn); err != nil {
					if err.Error() == "user quit" {
						return // User chose to quit
					}
					// Check for service unavailable error
					if err.Error() == "game service unavailable" {
						err := h.serviceHealthChecker.HandleServiceUnavailable(ctx, channel, connID, sshConn.User())
						if err != nil && err.Error() == "user quit" {
							return // User quit or timeout
						}
						continue // Return to menu after service unavailable handling
					}
					h.logger.Error("Error handling menu choice", "error", err, "choice", menuChoice.Action, "username", sshConn.User())
					if ctx.Err() != nil {
						return // Context cancelled
					}
					continue // Return to menu
				}

				// If we get here, the menu choice was handled successfully
				// Some choices (like quit) will have returned from the function
				// Others (like play) will have started a game session
				// Continue the loop to show the menu again (unless user quit)
			}

		case "window-change":
			// Handle terminal resize
			if sessionID != "" && len(req.Payload) > 0 {
				cols, rows := h.gameIOHandler.ParseWindowChange(req.Payload)
				h.logger.Debug("Terminal resize", "session_id", sessionID, "cols", cols, "rows", rows)

				// Send resize request to Game Service
				if err := h.gameIOHandler.ResizeTerminal(ctx, sessionID, cols, rows); err != nil {
					h.logger.Error("Failed to resize terminal", "error", err, "session_id", sessionID)
				}
			}
			req.Reply(true, nil)

		default:
			// Reject unknown requests
			req.Reply(false, nil)
		}
	}

	// DON'T automatically stop game sessions when channels close
	// This allows users to reconnect to ongoing NetHack sessions after temporary disconnections
	// Game sessions should only be stopped on explicit user quit or timeout
	if sessionID != "" {
		h.logger.Info("SSH channel closed but keeping game session alive for reconnection", "session_id", sessionID)
		// Future: Implement session timeout/cleanup after extended inactivity
	}
}
