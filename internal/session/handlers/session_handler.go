package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/dungeongate/internal/session/pools"
	"github.com/dungeongate/internal/session/resources"
	"github.com/dungeongate/internal/session/menu"
	"golang.org/x/crypto/ssh"
)

// SessionHandler coordinates all session activities using pool infrastructure
type SessionHandler struct {
	connectionPool  *pools.ConnectionPool
	workerPool      *pools.WorkerPool
	ptyPool         *pools.PTYPool
	backpressure    *pools.BackpressureManager

	resourceLimiter *resources.ResourceLimiter
	resourceTracker *resources.ResourceTracker
	metricsRegistry *resources.MetricsRegistry

	authHandler   *AuthHandler
	gameHandler   *GameHandler
	streamHandler *StreamHandler
	menuHandler   *menu.MenuHandler

	logger *slog.Logger

	// Metrics
	connectionsTotal     *resources.CounterMetric
	connectionsActive    *resources.GaugeMetric
	connectionDuration   *resources.HistogramMetric
	handlerErrors        *resources.CounterMetric
}

// NewSessionHandler creates a new session handler
func NewSessionHandler(
	connectionPool *pools.ConnectionPool,
	workerPool *pools.WorkerPool,
	ptyPool *pools.PTYPool,
	backpressure *pools.BackpressureManager,
	resourceLimiter *resources.ResourceLimiter,
	resourceTracker *resources.ResourceTracker,
	metricsRegistry *resources.MetricsRegistry,
	authHandler *AuthHandler,
	gameHandler *GameHandler,
	streamHandler *StreamHandler,
	menuHandler *menu.MenuHandler,
	logger *slog.Logger,
) *SessionHandler {
	sh := &SessionHandler{
		connectionPool:  connectionPool,
		workerPool:      workerPool,
		ptyPool:         ptyPool,
		backpressure:    backpressure,
		resourceLimiter: resourceLimiter,
		resourceTracker: resourceTracker,
		metricsRegistry: metricsRegistry,
		authHandler:     authHandler,
		gameHandler:     gameHandler,
		streamHandler:   streamHandler,
		menuHandler:     menuHandler,
		logger:          logger,
	}

	sh.initializeMetrics()
	return sh
}

// initializeMetrics sets up metrics for the session handler
func (sh *SessionHandler) initializeMetrics() {
	sh.connectionsTotal = sh.metricsRegistry.RegisterCounter(
		"session_connections_total",
		"Total number of connections handled",
		map[string]string{"handler": "session"})

	sh.connectionsActive = sh.metricsRegistry.RegisterGauge(
		"session_connections_active",
		"Number of active connections",
		map[string]string{"handler": "session"})

	sh.connectionDuration = sh.metricsRegistry.RegisterHistogram(
		"session_connection_duration_seconds",
		"Time spent handling connections",
		nil,
		map[string]string{"handler": "session"})

	sh.handlerErrors = sh.metricsRegistry.RegisterCounter(
		"session_handler_errors_total",
		"Total number of handler errors",
		map[string]string{"handler": "session"})
}

// HandleNewConnection is the main entry point that replaces the old HandleConnection
func (sh *SessionHandler) HandleNewConnection(ctx context.Context, conn net.Conn, config *ssh.ServerConfig) error {
	startTime := time.Now()
	sh.connectionsTotal.Inc()
	sh.connectionsActive.Inc()
	defer sh.connectionsActive.Dec()

	// Track connection duration
	defer func() {
		duration := time.Since(startTime)
		sh.connectionDuration.Observe(duration.Seconds())
		sh.logger.Info("Connection handling completed",
			"remote_addr", conn.RemoteAddr(),
			"duration", duration)
	}()

	// Check backpressure before accepting connection
	if !sh.backpressure.CanAccept() {
		sh.logger.Warn("Connection rejected due to backpressure",
			"remote_addr", conn.RemoteAddr())
		conn.Close()
		return fmt.Errorf("server overloaded")
	}

	// Perform SSH handshake first to get SSH connection details
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		sh.logger.Error("Failed SSH handshake",
			"error", err,
			"remote_addr", conn.RemoteAddr())
		sh.handlerErrors.Inc()
		conn.Close()
		return fmt.Errorf("SSH handshake failed: %w", err)
	}
	defer sshConn.Close()

	// Request connection from pool
	poolConn, err := sh.connectionPool.RequestConnection(ctx, nil, sshConn, pools.PriorityNormal)
	if err != nil {
		sh.logger.Error("Failed to request connection from pool",
			"error", err,
			"remote_addr", conn.RemoteAddr())
		sh.handlerErrors.Inc()
		conn.Close()
		return fmt.Errorf("failed to request connection: %w", err)
	}
	defer sh.connectionPool.ReleaseConnection(poolConn.ID)

	// Handle SSH channels and requests directly
	sh.logger.Info("SSH connection established",
		"connection_id", poolConn.ID,
		"username", sshConn.User(),
		"remote_addr", conn.RemoteAddr())

	// Handle SSH requests in background  
	go sh.handleSSHRequestsInBackground(ctx, reqs, poolConn)

	// Handle channels directly (more interactive)
	return sh.handleSSHChannels(ctx, chans, poolConn)
}


// handleSSHRequestsInBackground handles SSH requests in background
func (sh *SessionHandler) handleSSHRequestsInBackground(ctx context.Context, reqs <-chan *ssh.Request, conn *pools.Connection) {
	for req := range reqs {
		sh.logger.Debug("Received SSH request",
			"type", req.Type,
			"connection_id", conn.ID)

		switch req.Type {
		case "keepalive":
			if req.WantReply {
				req.Reply(true, nil)
			}
		default:
			if req.WantReply {
				req.Reply(false, nil)
			}
		}
	}
}

// handleSSHChannels handles SSH channels and coordinates with specialized handlers
func (sh *SessionHandler) handleSSHChannels(ctx context.Context, chans <-chan ssh.NewChannel, conn *pools.Connection) error {
	for newChannel := range chans {
		sh.logger.Debug("Received SSH channel",
			"type", newChannel.ChannelType(),
			"connection_id", conn.ID)

		switch newChannel.ChannelType() {
		case "session":
			// Create work item for session channel
			work := &pools.WorkItem{
				Type:       pools.WorkTypeMenuAction,
				Connection: conn,
				Handler:    sh.handleSessionChannelWork,
				Context:    ctx,
				Priority:   pools.PriorityNormal,
				QueuedAt:   time.Now(),
				Data:       newChannel,
			}

			if err := sh.workerPool.Submit(work); err != nil {
				sh.logger.Error("Failed to submit session channel work",
					"error", err,
					"connection_id", conn.ID)
				newChannel.Reject(ssh.ResourceShortage, "server overloaded")
			}
		default:
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
		}
	}

	return nil
}

// handleSessionChannelWork processes session channels using specialized handlers
func (sh *SessionHandler) handleSessionChannelWork(ctx context.Context, conn *pools.Connection) error {
	newChannel, ok := conn.Context.Value("work_data").(ssh.NewChannel)
	if !ok {
		return fmt.Errorf("invalid session channel data")
	}

	channel, requests, err := newChannel.Accept()
	if err != nil {
		sh.logger.Error("Failed to accept channel",
			"error", err,
			"connection_id", conn.ID)
		return fmt.Errorf("failed to accept channel: %w", err)
	}
	defer func() {
		// Clear screen on exit
		channel.Write([]byte("\033[2J\033[H"))
		channel.Close()
	}()

	// Update connection with channel
	conn.SSHChannel = channel

	// Check resource limits for this user
	if !sh.resourceLimiter.CanExecute(conn.UserID, "session") {
		sh.logger.Warn("Session blocked by resource limiter",
			"user_id", conn.UserID,
			"connection_id", conn.ID)
		channel.Write([]byte("Resource limit exceeded. Please try again later.\r\n"))
		return fmt.Errorf("resource limit exceeded")
	}

	// Track this session
	sh.resourceTracker.TrackConnection(conn.ID, conn.UserID, "unknown")
	defer sh.resourceTracker.UntrackConnection(conn.ID)

	// Handle session requests and start main session loop
	return sh.handleSessionLoop(ctx, conn, channel, requests)
}

// handleSessionLoop manages the main session lifecycle
func (sh *SessionHandler) handleSessionLoop(ctx context.Context, conn *pools.Connection, channel ssh.Channel, requests <-chan *ssh.Request) error {
	var sessionID string
	var terminalCols, terminalRows int = 80, 24

	// Process initial session requests (PTY, shell, etc.)
	for req := range requests {
		switch req.Type {
		case "pty-req":
			if len(req.Payload) > 0 {
				terminalCols, terminalRows = sh.parsePTYRequest(req.Payload)
			}
			req.Reply(true, nil)

		case "shell":
			req.Reply(true, nil)

			// Clear the terminal
			channel.Write([]byte("\033[2J\033[H"))

			// Check service health using auth handler
			if err := sh.authHandler.CheckServiceHealth(ctx); err != nil {
				sh.logger.Warn("Required services unavailable",
					"username", conn.Username,
					"error", err)
				return sh.handleServiceUnavailable(ctx, conn, channel)
			}

			// Get user info through auth handler
			userInfo, err := sh.authHandler.GetUserInfo(ctx, conn.SSHConn)
			if err != nil {
				sh.logger.Debug("No authenticated user info, treating as anonymous",
					"error", err,
					"username", conn.Username)
				userInfo = nil
			}

			// Main menu loop - delegate to enhanced menu handler
			return sh.handleMainMenuLoop(ctx, conn, channel, userInfo, terminalCols, terminalRows)

		case "window-change":
			if sessionID != "" && len(req.Payload) > 0 {
				cols, rows := sh.parseWindowChange(req.Payload)
				sh.logger.Debug("Terminal resize",
					"session_id", sessionID,
					"cols", cols,
					"rows", rows)

				// Delegate to game handler for terminal resize
				if err := sh.gameHandler.ResizeTerminal(ctx, sessionID, cols, rows); err != nil {
					sh.logger.Error("Failed to resize terminal",
						"error", err,
						"session_id", sessionID)
				}
			}
			req.Reply(true, nil)

		default:
			req.Reply(false, nil)
		}
	}

	return nil
}

// handleMainMenuLoop delegates to the enhanced menu handler
func (sh *SessionHandler) handleMainMenuLoop(ctx context.Context, conn *pools.Connection, channel ssh.Channel, userInfo interface{}, terminalCols, terminalRows int) error {
	// Create enhanced menu handler that's pool-aware
	poolAwareMenu := NewPoolAwareMenuHandler(
		sh.menuHandler,
		sh.workerPool,
		sh.resourceLimiter,
		sh.connectionPool,
		sh.authHandler,
		sh.gameHandler,
		sh.streamHandler,
		sh.metricsRegistry,
		sh.logger,
	)

	return poolAwareMenu.HandleMenuLoop(ctx, conn, channel, userInfo, terminalCols, terminalRows)
}

// handleServiceUnavailable delegates to the enhanced menu handler
func (sh *SessionHandler) handleServiceUnavailable(ctx context.Context, conn *pools.Connection, channel ssh.Channel) error {
	// Use the menu handler's existing service unavailable functionality
	// but track it as a connection state change
	sh.resourceTracker.UpdateConnectionState(conn.ID, "service_unavailable")
	defer sh.resourceTracker.UpdateConnectionState(conn.ID, "available")

	// This would need to be implemented based on the existing handler logic
	// For now, return a simple error
	channel.Write([]byte("Services temporarily unavailable. Please try again later.\r\n"))
	time.Sleep(2 * time.Second)
	return fmt.Errorf("service unavailable")
}

// Helper functions for parsing SSH requests (copied from original handler)
func (sh *SessionHandler) parsePTYRequest(payload []byte) (int, int) {
	if len(payload) < 8 {
		return 80, 24
	}

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

func (sh *SessionHandler) parseWindowChange(payload []byte) (int, int) {
	if len(payload) < 8 {
		return 80, 24
	}

	cols := int(payload[0])<<24 | int(payload[1])<<16 | int(payload[2])<<8 | int(payload[3])
	rows := int(payload[4])<<24 | int(payload[5])<<16 | int(payload[6])<<8 | int(payload[7])

	return cols, rows
}

// Shutdown gracefully shuts down the session handler
func (sh *SessionHandler) Shutdown(ctx context.Context) error {
	sh.logger.Info("Shutting down session handler")

	// Stop accepting new work
	if err := sh.workerPool.Stop(ctx); err != nil {
		sh.logger.Error("Error stopping worker pool", "error", err)
	}

	// Close all connections
	if err := sh.connectionPool.Stop(ctx); err != nil {
		sh.logger.Error("Error stopping connection pool", "error", err)
	}

	sh.logger.Info("Session handler shutdown complete")
	return nil
}