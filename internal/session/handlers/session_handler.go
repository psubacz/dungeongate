package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/dungeongate/internal/session/pools"
	"github.com/dungeongate/internal/session/resources"
	"github.com/dungeongate/internal/session/menu"
	"golang.org/x/crypto/ssh"
)

// ServerConfig holds configuration for all servers
type ServerConfig struct {
	SSH  SSHServerConfig  `yaml:"ssh"`
	HTTP HTTPServerConfig `yaml:"http"`
	GRPC GRPCServerConfig `yaml:"grpc"`
}

// SSHServerConfig holds SSH server configuration
type SSHServerConfig struct {
	Address     string `yaml:"address"`
	Port        int    `yaml:"port"`
	HostKeyPath string `yaml:"host_key_path"`
	Banner      string `yaml:"banner"`
}

// HTTPServerConfig holds HTTP server configuration  
type HTTPServerConfig struct {
	Address string `yaml:"address"`
	Port    int    `yaml:"port"`
}

// GRPCServerConfig holds gRPC server configuration
type GRPCServerConfig struct {
	Address string `yaml:"address"`
	Port    int    `yaml:"port"`
}

// PoolBasedServer interface for servers managed by pool infrastructure
type PoolBasedServer interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Name() string
}

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

	// Server configuration for pool-based architecture
	serverConfig *ServerConfig
	
	// Server management
	servers      map[string]PoolBasedServer
	serverWg     sync.WaitGroup
	shutdownCtx  context.Context
	shutdownFunc context.CancelFunc

	// Metrics
	connectionsTotal     *resources.CounterMetric
	connectionsActive    *resources.GaugeMetric
	connectionDuration   *resources.HistogramMetric
	handlerErrors        *resources.CounterMetric
}

// NewSessionHandler creates a new session handler with server management
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
	serverConfig *ServerConfig,
	logger *slog.Logger,
) *SessionHandler {
	// Create shutdown context
	shutdownCtx, shutdownFunc := context.WithCancel(context.Background())
	
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
		serverConfig:    serverConfig,
		servers:         make(map[string]PoolBasedServer),
		shutdownCtx:     shutdownCtx,
		shutdownFunc:    shutdownFunc,
		logger:          logger,
	}

	sh.initializeMetrics()
	sh.initializeServers()
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
		[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		map[string]string{"handler": "session"})

	sh.handlerErrors = sh.metricsRegistry.RegisterCounter(
		"session_handler_errors_total",
		"Total number of handler errors",
		map[string]string{"handler": "session"})
}

// initializeServers creates and configures pool-based servers
func (sh *SessionHandler) initializeServers() {
	if sh.serverConfig == nil {
		sh.logger.Warn("No server configuration provided, servers will not be created")
		return
	}

	// Create pool-based SSH server
	sshServer := NewPoolBasedSSHServer(sh.serverConfig.SSH, sh, sh.logger)
	sh.servers["ssh"] = sshServer

	// Create pool-based HTTP server  
	httpServer := NewPoolBasedHTTPServer(sh.serverConfig.HTTP, sh, sh.logger)
	sh.servers["http"] = httpServer

	// Create pool-based gRPC server
	grpcServer := NewPoolBasedGRPCServer(sh.serverConfig.GRPC, sh, sh.logger)
	sh.servers["grpc"] = grpcServer

	sh.logger.Info("Pool-based servers initialized", 
		"ssh_port", sh.serverConfig.SSH.Port,
		"http_port", sh.serverConfig.HTTP.Port,
		"grpc_port", sh.serverConfig.GRPC.Port)
}

// StartServers starts all pool-based servers
func (sh *SessionHandler) StartServers(ctx context.Context) error {
	sh.logger.Info("Starting pool-based servers")
	
	for name, server := range sh.servers {
		sh.serverWg.Add(1)
		go func(serverName string, srv PoolBasedServer) {
			defer sh.serverWg.Done()
			if err := srv.Start(ctx); err != nil {
				sh.logger.Error("Pool-based server error", "server", serverName, "error", err)
			}
		}(name, server)
		
		sh.logger.Info("Started pool-based server", "server", name)
	}
	
	return nil
}

// StopServers stops all pool-based servers gracefully
func (sh *SessionHandler) StopServers(ctx context.Context) error {
	sh.logger.Info("Stopping pool-based servers")
	
	for name, server := range sh.servers {
		if err := server.Stop(ctx); err != nil {
			sh.logger.Error("Error stopping pool-based server", "server", name, "error", err)
		} else {
			sh.logger.Info("Stopped pool-based server", "server", name)
		}
	}
	
	// Wait for all servers to stop
	done := make(chan struct{})
	go func() {
		sh.serverWg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		sh.logger.Info("All pool-based servers stopped")
	case <-ctx.Done():
		sh.logger.Warn("Timeout waiting for pool-based servers to stop")
	}
	
	return nil
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
	// Get the SSH new channel from the context value that was set during work submission
	workData := ctx.Value("work_data")
	if workData == nil {
		return fmt.Errorf("no work data in context")
	}
	
	newChannel, ok := workData.(ssh.NewChannel)
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

	// Check resource limits for this user (use username if UserID is not set)
	userForLimiting := conn.UserID
	if userForLimiting == "" {
		userForLimiting = conn.Username
	}
	if !sh.resourceLimiter.CanExecute(userForLimiting, "session") {
		sh.logger.Warn("Session blocked by resource limiter",
			"user_id", userForLimiting,
			"connection_id", conn.ID)
		channel.Write([]byte("Resource limit exceeded. Please try again later.\r\n"))
		return fmt.Errorf("resource limit exceeded")
	}

	// Track this session
	sh.resourceTracker.TrackConnection(conn.ID, userForLimiting, "unknown")
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