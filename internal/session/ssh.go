package session

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	rand2 "math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dungeongate/internal/user"
	"github.com/dungeongate/pkg/config"
	"github.com/dungeongate/pkg/metrics"
	"golang.org/x/crypto/ssh"
)

// WindowSize represents terminal window dimensions
type WindowSize struct {
	Width  uint16
	Height uint16
	X      uint16
	Y      uint16
}

// SSHServer represents the SSH server
type SSHServer struct {
	config         *config.SessionServiceConfig
	sshConfig      *ssh.ServerConfig
	sessionService *Service
	ptyManager     *PTYManager

	// Service clients
	authClient AuthServiceClient
	userClient UserServiceClient
	gameClient GameServiceClient

	// Connection tracking
	connections    map[string]*SSHConnection
	connectionsMux sync.RWMutex

	// Metrics
	metrics     *SSHMetrics
	promMetrics *metrics.SSHMetrics
}

// SSHConnection represents an active SSH connection
type SSHConnection struct {
	ID           string
	Username     string
	RemoteAddr   string
	StartTime    time.Time
	LastActivity time.Time
	Sessions     map[string]*SSHSessionContext
	sessionsMux  sync.RWMutex
}

// SSHSessionContext represents an SSH session context
type SSHSessionContext struct {
	SessionID    string
	ConnectionID string
	Username     string
	Channel      ssh.Channel
	Requests     <-chan *ssh.Request

	// Terminal state
	TerminalType string
	WindowSize   *WindowSize
	HasPTY       bool

	// Authentication state
	IsAuthenticated   bool
	AuthenticatedUser *User

	// Session state
	Command     string
	Environment map[string]string
	ExitStatus  int

	// Control channels
	done       chan struct{}
	ptySession *PTYSession

	// Metrics
	BytesRead    int64
	BytesWritten int64
	StartTime    time.Time
	LastActivity time.Time
}

// SSHMetrics tracks SSH server metrics
type SSHMetrics struct {
	mutex             sync.RWMutex
	TotalConnections  int64
	ActiveConnections int64
	FailedConnections int64
	TotalSessions     int64
	ActiveSessions    int64
	BytesTransferred  int64
}

// NewSSHServer creates a new SSH server instance
func NewSSHServer(sessionService *Service, cfg *config.SessionServiceConfig) (*SSHServer, error) {
	server := &SSHServer{
		config:         cfg,
		sessionService: sessionService,
		connections:    make(map[string]*SSHConnection),
		metrics:        &SSHMetrics{},
	}

	// Only initialize Prometheus metrics if not in test mode
	// In test mode, the metrics will be nil to avoid registration conflicts
	if cfg != nil && cfg.Logging != nil && cfg.Logging.Level != "test" {
		server.promMetrics = metrics.NewSSHMetrics("dungeongate", "ssh")
	}

	// Initialize PTY manager
	ptyManager, err := NewPTYManager()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize PTY manager: %w", err)
	}
	server.ptyManager = ptyManager

	// Configure SSH server
	if err := server.configureSSH(); err != nil {
		return nil, fmt.Errorf("failed to configure SSH server: %w", err)
	}

	// Initialize service clients
	server.initializeClients()

	return server, nil
}

// updatePromMetrics safely updates Prometheus metrics if they're initialized
func (s *SSHServer) updatePromMetrics(fn func(*metrics.SSHMetrics)) {
	if s.promMetrics != nil {
		fn(s.promMetrics)
	}
}

// configureSSH sets up SSH server configuration
func (s *SSHServer) configureSSH() error {
	sshConfig := &ssh.ServerConfig{
		BannerCallback: s.handleBanner,
		ServerVersion:  "SSH-2.0-dungeongate",
		MaxAuthTries:   3,
	}

	// Configure authentication methods based on settings
	sshAuthConfig := s.config.GetSSH().Auth

	if sshAuthConfig.AllowAnonymous {
		// For anonymous access, use NoClientAuth to skip all authentication
		sshConfig.NoClientAuth = true
	} else {
		// Only set authentication callbacks when not allowing anonymous access
		if sshAuthConfig.PasswordAuth {
			sshConfig.PasswordCallback = s.handlePasswordAuth
		}
		if sshAuthConfig.PublicKeyAuth {
			sshConfig.PublicKeyCallback = s.handlePublicKeyAuth
		}
	}

	// Load or generate host key
	hostKey, err := s.loadOrGenerateHostKey()
	if err != nil {
		return fmt.Errorf("failed to load host key: %w", err)
	}

	sshConfig.AddHostKey(hostKey)
	s.sshConfig = sshConfig

	return nil
}

// initializeClients initializes service clients
func (s *SSHServer) initializeClients() {
	// Set default services addresses if not configured
	authServiceAddr := "localhost:9090"
	userServiceAddr := "localhost:9091"
	gameServiceAddr := "localhost:9092"

	// Get configured addresses if available
	if s.config != nil && s.config.Services != nil {
		if s.config.Services.AuthService != "" {
			authServiceAddr = s.config.Services.AuthService
		}
		if s.config.Services.UserService != "" {
			userServiceAddr = s.config.Services.UserService
		}
		if s.config.Services.GameService != "" {
			gameServiceAddr = s.config.Services.GameService
		}
	}

	s.authClient = NewAuthServiceClient(authServiceAddr)
	s.userClient = NewUserServiceClient(userServiceAddr)

	// Use games from config if available
	if s.config.Games != nil && len(s.config.Games) > 0 {
		s.gameClient = NewGameServiceClientWithConfig(gameServiceAddr, s.config.Games)
	} else {
		s.gameClient = NewGameServiceClient(gameServiceAddr)
	}
}

// Start starts the SSH server
func (s *SSHServer) Start(ctx context.Context, port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", port, err)
	}

	log.Printf("SSH server listening on port %d", port)

	// Start background services
	go s.startMetricsCollection(ctx)
	go s.startConnectionCleanup(ctx)

	// Handle shutdown
	go func() {
		<-ctx.Done()
		log.Println("SSH server shutting down...")
		listener.Close()
	}()

	// Accept connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil // Context cancelled
			}
			log.Printf("Failed to accept connection: %v", err)
			s.metrics.mutex.Lock()
			s.metrics.FailedConnections++
			s.metrics.mutex.Unlock()
			continue
		}

		go s.handleConnection(ctx, conn)
	}
}

// handleConnection handles a new SSH connection
func (s *SSHServer) handleConnection(ctx context.Context, netConn net.Conn) {
	defer netConn.Close()

	// Update metrics
	s.metrics.mutex.Lock()
	s.metrics.TotalConnections++
	s.metrics.ActiveConnections++
	s.metrics.mutex.Unlock()

	// Update Prometheus metrics
	s.updatePromMetrics(func(m *metrics.SSHMetrics) {
		m.ConnectionsTotal.Inc()
		m.ConnectionsActive.Inc()
	})
	connectionStart := time.Now()

	defer func() {
		s.metrics.mutex.Lock()
		s.metrics.ActiveConnections--
		s.metrics.mutex.Unlock()

		// Update Prometheus metrics
		s.updatePromMetrics(func(m *metrics.SSHMetrics) {
			m.ConnectionsActive.Dec()
			m.ConnectionDuration.Observe(time.Since(connectionStart).Seconds())
		})
	}()

	// Perform SSH handshake
	sshConn, chans, reqs, err := ssh.NewServerConn(netConn, s.sshConfig)
	if err != nil {
		log.Printf("Failed to handshake: %v", err)
		s.metrics.mutex.Lock()
		s.metrics.FailedConnections++
		s.metrics.mutex.Unlock()

		// Update Prometheus metrics
		s.updatePromMetrics(func(m *metrics.SSHMetrics) {
			m.ConnectionsFailed.Inc()
		})
		return
	}
	defer sshConn.Close()

	// Create connection context
	connectionID := generateConnectionID()
	connection := &SSHConnection{
		ID:           connectionID,
		Username:     sshConn.User(),
		RemoteAddr:   sshConn.RemoteAddr().String(),
		StartTime:    time.Now(),
		LastActivity: time.Now(),
		Sessions:     make(map[string]*SSHSessionContext),
	}

	// Track connection
	s.connectionsMux.Lock()
	s.connections[connectionID] = connection
	s.connectionsMux.Unlock()

	defer func() {
		s.connectionsMux.Lock()
		delete(s.connections, connectionID)
		s.connectionsMux.Unlock()
	}()

	log.Printf("User %s connected from %s (connection: %s)",
		connection.Username, connection.RemoteAddr, connectionID)

	// Handle global requests
	go ssh.DiscardRequests(reqs)

	// Handle channels
	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		go s.handleChannel(ctx, newChannel, connection)
	}
}

// handleChannel handles a new SSH channel (session)
func (s *SSHServer) handleChannel(ctx context.Context, newChannel ssh.NewChannel, connection *SSHConnection) {
	channel, requests, err := newChannel.Accept()
	if err != nil {
		log.Printf("Could not accept channel: %v", err)
		return
	}
	defer channel.Close()

	// Create session context
	sessionID := generateSSHSessionID()
	sessionCtx := &SSHSessionContext{
		SessionID:    sessionID,
		ConnectionID: connection.ID,
		Username:     connection.Username,
		Channel:      channel,
		Requests:     requests,
		WindowSize:   &WindowSize{Width: 80, Height: 24},
		Environment:  make(map[string]string),
		done:         make(chan struct{}),
		StartTime:    time.Now(),
		LastActivity: time.Now(),
	}

	// Track session
	connection.sessionsMux.Lock()
	connection.Sessions[sessionID] = sessionCtx
	connection.sessionsMux.Unlock()

	defer func() {
		connection.sessionsMux.Lock()
		delete(connection.Sessions, sessionID)
		connection.sessionsMux.Unlock()

		// Clean up PTY if allocated
		if sessionCtx.ptySession != nil {
			s.ptyManager.ReleasePTY(sessionID)
		}
	}()

	// Update metrics
	s.metrics.mutex.Lock()
	s.metrics.TotalSessions++
	s.metrics.ActiveSessions++
	s.metrics.mutex.Unlock()

	// Update Prometheus metrics
	s.updatePromMetrics(func(m *metrics.SSHMetrics) {
		m.SessionsTotal.Inc()
		m.SessionsActive.Inc()
	})
	sessionStart := time.Now()

	defer func() {
		s.metrics.mutex.Lock()
		s.metrics.ActiveSessions--
		s.metrics.mutex.Unlock()

		// Update Prometheus metrics
		s.updatePromMetrics(func(m *metrics.SSHMetrics) {
			m.SessionsActive.Dec()
			m.SessionDuration.Observe(time.Since(sessionStart).Seconds())
		})
	}()

	log.Printf("New session %s for user %s", sessionID, connection.Username)

	// Handle session requests
	go s.handleSessionRequests(ctx, sessionCtx)

	// Start the main menu
	s.startMainMenu(ctx, sessionCtx)
}

// handleSessionRequests handles SSH session requests
func (s *SSHServer) handleSessionRequests(ctx context.Context, sessionCtx *SSHSessionContext) {
	for {
		select {
		case req, ok := <-sessionCtx.Requests:
			if !ok {
				return
			}

			switch req.Type {
			case "pty-req":
				s.handlePTYRequest(sessionCtx, req)
			case "shell":
				s.handleShellRequest(sessionCtx, req)
			case "exec":
				s.handleExecRequest(sessionCtx, req)
			case "window-change":
				s.handleWindowChangeRequest(sessionCtx, req)
			case "env":
				s.handleEnvRequest(sessionCtx, req)
			default:
				req.Reply(false, nil)
			}

		case <-ctx.Done():
			return
		case <-sessionCtx.done:
			return
		}
	}
}

// handlePTYRequest handles PTY allocation request
func (s *SSHServer) handlePTYRequest(sessionCtx *SSHSessionContext, req *ssh.Request) {
	if len(req.Payload) < 4 {
		req.Reply(false, nil)
		return
	}

	// Parse PTY request
	termLen := req.Payload[3]
	if len(req.Payload) < 4+int(termLen)+16 {
		req.Reply(false, nil)
		return
	}

	sessionCtx.TerminalType = string(req.Payload[4 : 4+termLen])

	// Track terminal type
	s.updatePromMetrics(func(m *metrics.SSHMetrics) {
		m.TerminalTypes.WithLabelValues(sessionCtx.TerminalType).Inc()
	})

	// Extract window dimensions
	offset := 4 + int(termLen)
	sessionCtx.WindowSize.Width = uint16(req.Payload[offset])<<8 | uint16(req.Payload[offset+1])
	sessionCtx.WindowSize.Height = uint16(req.Payload[offset+2])<<8 | uint16(req.Payload[offset+3])
	sessionCtx.WindowSize.X = sessionCtx.WindowSize.Width
	sessionCtx.WindowSize.Y = sessionCtx.WindowSize.Height

	sessionCtx.HasPTY = true

	log.Printf("PTY allocated for session %s: %s %dx%d",
		sessionCtx.SessionID, sessionCtx.TerminalType,
		sessionCtx.WindowSize.Width, sessionCtx.WindowSize.Height)

	req.Reply(true, nil)
}

// handleShellRequest handles shell request
func (s *SSHServer) handleShellRequest(sessionCtx *SSHSessionContext, req *ssh.Request) {
	req.Reply(true, nil)
}

// handleExecRequest handles command execution request
func (s *SSHServer) handleExecRequest(sessionCtx *SSHSessionContext, req *ssh.Request) {
	if len(req.Payload) < 4 {
		req.Reply(false, nil)
		return
	}

	cmdLen := uint32(req.Payload[0])<<24 | uint32(req.Payload[1])<<16 |
		uint32(req.Payload[2])<<8 | uint32(req.Payload[3])

	if len(req.Payload) < 4+int(cmdLen) {
		req.Reply(false, nil)
		return
	}

	sessionCtx.Command = string(req.Payload[4 : 4+cmdLen])

	log.Printf("Command execution request for session %s: %s",
		sessionCtx.SessionID, sessionCtx.Command)

	req.Reply(true, nil)
}

// handleWindowChangeRequest handles window size change
func (s *SSHServer) handleWindowChangeRequest(sessionCtx *SSHSessionContext, req *ssh.Request) {
	if len(req.Payload) >= 16 {
		sessionCtx.WindowSize.Width = uint16(req.Payload[0])<<8 | uint16(req.Payload[1])
		sessionCtx.WindowSize.Height = uint16(req.Payload[2])<<8 | uint16(req.Payload[3])
		sessionCtx.WindowSize.X = sessionCtx.WindowSize.Width
		sessionCtx.WindowSize.Y = sessionCtx.WindowSize.Height

		// Track terminal size change
		s.updatePromMetrics(func(m *metrics.SSHMetrics) {
			m.TerminalSizeChanges.Inc()
		})

		// Update PTY window size if active
		if sessionCtx.ptySession != nil {
			sessionCtx.ptySession.ResizeWindow(sessionCtx.WindowSize.Height, sessionCtx.WindowSize.Width)
		}

		log.Printf("Window size changed for session %s: %dx%d",
			sessionCtx.SessionID, sessionCtx.WindowSize.Width, sessionCtx.WindowSize.Height)
	}
	req.Reply(true, nil)
}

// handleEnvRequest handles environment variable request
func (s *SSHServer) handleEnvRequest(sessionCtx *SSHSessionContext, req *ssh.Request) {
	if len(req.Payload) < 8 {
		req.Reply(false, nil)
		return
	}

	nameLen := uint32(req.Payload[0])<<24 | uint32(req.Payload[1])<<16 |
		uint32(req.Payload[2])<<8 | uint32(req.Payload[3])
	valueLen := uint32(req.Payload[4])<<24 | uint32(req.Payload[5])<<16 |
		uint32(req.Payload[6])<<8 | uint32(req.Payload[7])

	if len(req.Payload) < 8+int(nameLen)+int(valueLen) {
		req.Reply(false, nil)
		return
	}

	name := string(req.Payload[8 : 8+nameLen])
	value := string(req.Payload[8+nameLen : 8+nameLen+valueLen])

	sessionCtx.Environment[name] = value

	log.Printf("Environment variable set for session %s: %s=%s",
		sessionCtx.SessionID, name, value)

	req.Reply(true, nil)
}

// startMainMenu starts the main menu
func (s *SSHServer) startMainMenu(ctx context.Context, sessionCtx *SSHSessionContext) {
	defer close(sessionCtx.done)

	// Send welcome banner
	s.sendBanner(sessionCtx)

	// Main menu loop
	for {
		select {
		case <-ctx.Done():
			return
		case <-sessionCtx.done:
			return
		default:
			// Show menu
			s.showMenu(sessionCtx)

			// Read user input
			choice, err := s.readUserInput(sessionCtx)
			if err != nil {
				log.Printf("Error reading user input: %v", err)
				return
			}

			// Handle menu choice
			if !s.handleMenuChoice(ctx, sessionCtx, choice) {
				return
			}
		}
	}
}

// sendBanner sends the appropriate banner based on user authentication status
func (s *SSHServer) sendBanner(sessionCtx *SSHSessionContext) {
	var bannerPath string
	var username string

	// Determine which banner to show based on authentication status
	if sessionCtx.IsAuthenticated {
		bannerPath = s.getBannerPath("main_user")
		username = sessionCtx.Username
	} else {
		bannerPath = s.getBannerPath("main_anon")
		username = "Anonymous"
	}

	// Get terminal width, with fallback to default
	terminalWidth := 80
	if sessionCtx.WindowSize != nil {
		terminalWidth = int(sessionCtx.WindowSize.Width)
	}

	// Load and display the dynamic banner
	banner := s.loadDynamicBanner(bannerPath, terminalWidth, username)
	if banner != "" {
		s.writeToSession(sessionCtx, banner)
	} else {
		// Fallback to simple banner if file loading fails
		fallbackBanner := fmt.Sprintf("Welcome to DungeonGate, %s!\r\n\r\n", username)
		s.writeToSession(sessionCtx, fallbackBanner)
	}
}

// getBannerPath returns the configured path for a specific banner type
func (s *SSHServer) getBannerPath(bannerType string) string {
	if s.config == nil || s.config.Menu == nil || s.config.Menu.Banners == nil {
		return ""
	}

	switch bannerType {
	case "main_anon":
		return s.config.Menu.Banners.MainAnon
	case "main_user":
		return s.config.Menu.Banners.MainUser
	case "watch_menu":
		return s.config.Menu.Banners.WatchMenu
	default:
		return ""
	}
}

// loadDynamicBanner loads a banner file and dynamically resizes it for the terminal
func (s *SSHServer) loadDynamicBanner(bannerPath string, terminalWidth int, username string) string {
	if bannerPath == "" {
		return ""
	}

	// Read banner file
	content, err := os.ReadFile(bannerPath)
	if err != nil {
		log.Printf("Failed to read banner file %s: %v", bannerPath, err)
		return ""
	}

	// Process the banner content
	bannerText := string(content)

	// Replace placeholders
	bannerText = s.replaceBannerPlaceholders(bannerText, username)

	// Add hardcoded footer
	bannerText = s.addBannerFooter(bannerText)

	// Resize banner to fit terminal width
	resizedBanner := s.resizeBanner(bannerText, terminalWidth)

	return resizedBanner + "\r\n"
}

// replaceBannerPlaceholders replaces template variables in banner text
func (s *SSHServer) replaceBannerPlaceholders(bannerText, username string) string {
	// Replace common placeholders - order matters! Replace longer placeholders first
	bannerText = strings.ReplaceAll(bannerText, "$SERVERID", "DungeonGate")
	bannerText = strings.ReplaceAll(bannerText, "$USERNAME", username) // Replace this before $USER
	bannerText = strings.ReplaceAll(bannerText, "{user}", username)
	bannerText = strings.ReplaceAll(bannerText, "$USER", username)

	// Add timestamp if placeholder exists
	bannerText = strings.ReplaceAll(bannerText, "$DATE", time.Now().Format("2006-01-02"))
	bannerText = strings.ReplaceAll(bannerText, "$TIME", time.Now().Format("15:04:05"))

	return bannerText
}

// addBannerFooter adds a hardcoded footer to the banner content
func (s *SSHServer) addBannerFooter(bannerText string) string {
	version := "0.0.2" // default fallback
	if s.config != nil && s.config.Version != "" {
		version = s.config.Version
	}

	footer := fmt.Sprintf(`

	
  ## Powered by ᚠ ᚢ ᚦ ᚨ ᚱ ᚷ ᚹ ᛞ ᛉ ᛏ   DungeonGate   ᛃ ᛇ ᛒ ᛗ ᛚ ᛝ %s
  ## See https://github.com/psubacz/dungeongate`, version)

	return bannerText + footer
}

// resizeBanner dynamically adjusts banner content to fit terminal width
func (s *SSHServer) resizeBanner(bannerText string, terminalWidth int) string {
	if terminalWidth <= 0 {
		terminalWidth = 80 // Default fallback
	}

	lines := strings.Split(bannerText, "\n")
	var resizedLines []string

	for _, line := range lines {
		// Remove any existing \r characters
		line = strings.TrimRight(line, "\r")

		// Calculate padding needed to center the line
		lineLength := len(line)

		if lineLength == 0 {
			// Empty line
			resizedLines = append(resizedLines, "")
			continue
		}

		if lineLength > terminalWidth && terminalWidth > 10 {
			// Line is too long, truncate it with ellipsis if there's enough space
			truncated := line[:terminalWidth-3] + "..."
			resizedLines = append(resizedLines, truncated)
		} else if lineLength > terminalWidth {
			// Just truncate without ellipsis for very narrow terminals
			truncated := line[:terminalWidth]
			resizedLines = append(resizedLines, truncated)
		} else {
			// Left-align the line (no padding needed)
			resizedLines = append(resizedLines, line)
		}
	}

	// Join lines with \r\n for proper terminal display
	return strings.Join(resizedLines, "\r\n")
}

// showDynamicMenu generates a menu with the specified width
func (s *SSHServer) showDynamicMenu(sessionCtx *SSHSessionContext, width int) {
	var bannerPath string
	var username string

	// Determine which banner to show based on authentication status
	if sessionCtx.IsAuthenticated && sessionCtx.AuthenticatedUser != nil {
		bannerPath = s.getBannerPath("main_user")
		username = sessionCtx.AuthenticatedUser.Username
	} else {
		bannerPath = s.getBannerPath("main_anon")
		username = "Anonymous"
	}

	// Try to load and display the dynamic banner
	banner := s.loadDynamicBanner(bannerPath, width, username)
	if banner != "" {
		s.writeToSession(sessionCtx, banner)
		s.writeToSession(sessionCtx, "\r\nChoice: ")
		return
	}

	// Fallback to boxed menu if banner loading fails

	// Generate top border
	topBorder := "╔" + strings.Repeat("═", width-2) + "╗\r\n"

	// Generate title line
	title := "DungeonGate - SSH Edition"
	titleLine := s.generateMenuLine(title, width, true)

	// Generate separator
	separator := "╠" + strings.Repeat("═", width-2) + "╣\r\n"

	// Generate content lines
	var menu strings.Builder
	menu.WriteString(topBorder)
	menu.WriteString(titleLine)
	menu.WriteString(separator)
	menu.WriteString(s.generateMenuLine("", width, false)) // Empty line

	// Welcome message
	if sessionCtx.IsAuthenticated && sessionCtx.AuthenticatedUser != nil {
		welcomeMsg := fmt.Sprintf("Welcome, %s!", sessionCtx.AuthenticatedUser.Username)
		menu.WriteString(s.generateMenuLine(welcomeMsg, width, false))
	} else {
		menu.WriteString(s.generateMenuLine("Welcome, anonymous user!", width, false))
	}

	menu.WriteString(s.generateMenuLine("", width, false)) // Empty line

	// Menu options
	if sessionCtx.IsAuthenticated && sessionCtx.AuthenticatedUser != nil {
		menu.WriteString(s.generateMenuLine("[p] Play a game", width, false))
		menu.WriteString(s.generateMenuLine("[w] Watch games", width, false))
		menu.WriteString(s.generateMenuLine("[e] Edit profile", width, false))
		menu.WriteString(s.generateMenuLine("[l] List games", width, false))
		menu.WriteString(s.generateMenuLine("[r] View recordings", width, false))
		menu.WriteString(s.generateMenuLine("[s] Statistics", width, false))
		menu.WriteString(s.generateMenuLine("[q] Quit", width, false))
	} else {
		menu.WriteString(s.generateMenuLine("[l] Login", width, false))
		menu.WriteString(s.generateMenuLine("[r] Register", width, false))
		menu.WriteString(s.generateMenuLine("[w] Watch games", width, false))
		menu.WriteString(s.generateMenuLine("[g] List games", width, false))
		menu.WriteString(s.generateMenuLine("[q] Quit", width, false))
	}

	menu.WriteString(s.generateMenuLine("", width, false)) // Empty line

	// Generate bottom border
	bottomBorder := "╚" + strings.Repeat("═", width-2) + "╝\r\n"
	menu.WriteString(bottomBorder)

	menu.WriteString("\r\nChoice: ")

	s.writeToSession(sessionCtx, menu.String())
}

// generateMenuLine creates a properly formatted menu line
func (s *SSHServer) generateMenuLine(content string, width int, centered bool) string {
	if centered {
		// Center the content
		padding := (width - 2 - len(content)) / 2
		leftPad := strings.Repeat(" ", padding)
		rightPad := strings.Repeat(" ", width-2-len(content)-padding)
		return "║" + leftPad + content + rightPad + "║\r\n"
	} else {
		// Left-align with padding
		if content == "" {
			// Empty line
			return "║" + strings.Repeat(" ", width-2) + "║\r\n"
		} else {
			// Content line with left padding
			leftPad := "  " // 2 spaces for indentation
			contentWithPad := leftPad + content
			rightPad := strings.Repeat(" ", width-2-len(contentWithPad))
			return "║" + contentWithPad + rightPad + "║\r\n"
		}
	}
}

// showCompactMenu shows a simple menu for narrow terminals
func (s *SSHServer) showCompactMenu(sessionCtx *SSHSessionContext) {
	var bannerPath string
	var username string

	// Determine which banner to show based on authentication status
	if sessionCtx.IsAuthenticated && sessionCtx.AuthenticatedUser != nil {
		bannerPath = s.getBannerPath("main_user")
		username = sessionCtx.AuthenticatedUser.Username
	} else {
		bannerPath = s.getBannerPath("main_anon")
		username = "Anonymous"
	}

	// Get terminal width for narrow terminals
	terminalWidth := 50
	if sessionCtx.WindowSize != nil && sessionCtx.WindowSize.Width > 0 {
		terminalWidth = int(sessionCtx.WindowSize.Width)
	}

	// Try to load and display the dynamic banner
	banner := s.loadDynamicBanner(bannerPath, terminalWidth, username)
	if banner != "" {
		s.writeToSession(sessionCtx, banner)
		s.writeToSession(sessionCtx, "\r\nChoice: ")
	} else {
		// Fallback to simple menu if banner loading fails
		var menu strings.Builder
		menu.WriteString("=== DungeonGate ===\r\n\r\n")

		if sessionCtx.IsAuthenticated && sessionCtx.AuthenticatedUser != nil {
			menu.WriteString(fmt.Sprintf("Welcome, %s!\r\n\r\n", sessionCtx.AuthenticatedUser.Username))
			menu.WriteString("[p] Play a game\r\n")
			menu.WriteString("[w] Watch games\r\n")
			menu.WriteString("[e] Edit profile\r\n")
			menu.WriteString("[l] List games\r\n")
			menu.WriteString("[r] View recordings\r\n")
			menu.WriteString("[s] Statistics\r\n")
			menu.WriteString("[q] Quit\r\n")
		} else {
			menu.WriteString("Welcome, anonymous user!\r\n\r\n")
			menu.WriteString("[l] Login\r\n")
			menu.WriteString("[r] Register\r\n")
			menu.WriteString("[w] Watch games\r\n")
			menu.WriteString("[g] List games\r\n")
			menu.WriteString("[q] Quit\r\n")
		}

		menu.WriteString("\r\nChoice: ")
		s.writeToSession(sessionCtx, menu.String())
	}
}

func (s *SSHServer) showMenu(sessionCtx *SSHSessionContext) {
	s.clearScreen(sessionCtx)

	termWidth := int(sessionCtx.WindowSize.Width)

	// Minimum width check
	if termWidth < 50 {
		s.showCompactMenu(sessionCtx)
		return
	}

	// Calculate optimal menu width
	menuWidth := termWidth - 4 // Leave 2 chars margin on each side
	if menuWidth > 78 {
		menuWidth = 78 // Cap at reasonable maximum
	}

	s.showDynamicMenu(sessionCtx, menuWidth)
}

// // showMenu displays the main menu
// func (s *SSHServer) showMenu(sessionCtx *SSHSessionContext) {
// 	s.clearScreen(sessionCtx)

// 	menu := `
// ╔══════════════════════════════════════════════════════════════════════════════╗
// ║                            DungeonGate - SSH Edition                         ║
// ╠══════════════════════════════════════════════════════════════════════════════╣
// ║                                                                              ║
// `

// 	if sessionCtx.IsAuthenticated && sessionCtx.AuthenticatedUser != nil {
// 		menu += fmt.Sprintf("║  Welcome, %s!%-60s║\r\n",
// 			sessionCtx.AuthenticatedUser.Username,
// 			strings.Repeat(" ", 60-len(sessionCtx.AuthenticatedUser.Username)-10))
// 		menu += `║                                                                              ║
// ║  [p] Play a game                                                             ║
// ║  [w] Watch games                                                             ║
// ║  [e] Edit profile                                                            ║
// ║  [l] List games                                                              ║
// ║  [r] View recordings                                                         ║
// ║  [s] Statistics                                                              ║
// ║  [q] Quit                                                                    ║
// `
// 	} else {
// 		menu += `║  Welcome, anonymous user!                                                    ║
// ║                                                                              ║
// ║  [l] Login                                                                   ║
// ║  [r] Register                                                                ║
// ║  [w] Watch games                                                             ║
// ║  [g] List games                                                              ║
// ║  [q] Quit                                                                    ║
// `
// 	}

// 	menu += `║                                                                              ║
// ╚══════════════════════════════════════════════════════════════════════════════╝

// Choice: `

// 	s.writeToSession(sessionCtx, menu)
// }

// readUserInput reads user input from the SSH session
func (s *SSHServer) readUserInput(sessionCtx *SSHSessionContext) (string, error) {
	buffer := make([]byte, 1)

	for {
		n, err := sessionCtx.Channel.Read(buffer)
		if err != nil {
			return "", err
		}

		if n == 0 {
			continue
		}

		// Update metrics
		sessionCtx.BytesRead += int64(n)
		sessionCtx.LastActivity = time.Now()

		char := buffer[0]

		// Handle special characters
		switch char {
		case 3: // Ctrl+C
			return "", fmt.Errorf("interrupted")
		case '\r', '\n':
			// Ignore line endings for single char input
			continue
		case ' ', '\t':
			// Ignore whitespace for single char input
			continue
		default:
			// Return the character as a string
			if char >= 32 && char <= 126 { // Printable ASCII
				return string(char), nil
			}
			// Ignore other control characters
			continue
		}
	}
}

// readPasswordInput reads password input (without echo)
func (s *SSHServer) readPasswordInput(sessionCtx *SSHSessionContext) (string, error) {
	var password strings.Builder
	buffer := make([]byte, 1)

	for {
		n, err := sessionCtx.Channel.Read(buffer)
		if err != nil {
			return "", err
		}

		if n == 0 {
			continue
		}

		sessionCtx.BytesRead += int64(n)
		sessionCtx.LastActivity = time.Now()

		char := buffer[0]

		switch char {
		case '\r', '\n':
			// End of input
			s.writeToSession(sessionCtx, "\r\n")
			return password.String(), nil
		case '\b', 127: // Backspace
			if password.Len() > 0 {
				passwordStr := password.String()
				password.Reset()
				password.WriteString(passwordStr[:len(passwordStr)-1])
				s.writeToSession(sessionCtx, "\b \b")
			}
		case 3: // Ctrl+C
			return "", fmt.Errorf("interrupted")
		default:
			password.WriteByte(char)
			s.writeToSession(sessionCtx, "*")
		}
	}
}

// handleMenuChoice handles menu selection
func (s *SSHServer) handleMenuChoice(ctx context.Context, sessionCtx *SSHSessionContext, choice string) bool {
	switch strings.ToLower(choice) {
	case "l":
		if sessionCtx.IsAuthenticated {
			return s.handleListGames(ctx, sessionCtx)
		}
		return s.handleLogin(ctx, sessionCtx)
	case "r":
		if sessionCtx.IsAuthenticated {
			return s.handleViewRecordings(ctx, sessionCtx)
		}
		return s.handleRegisterEnhanced(ctx, sessionCtx)
	case "p":
		if !sessionCtx.IsAuthenticated {
			s.writeToSession(sessionCtx, "Please login first.\r\n")
			s.waitForKeypress(sessionCtx)
			return true
		}
		return s.handlePlayGame(ctx, sessionCtx)
	case "w":
		return s.handleWatchGames(ctx, sessionCtx)
	case "e":
		if !sessionCtx.IsAuthenticated {
			s.writeToSession(sessionCtx, "Please login first.\r\n")
			s.waitForKeypress(sessionCtx)
			return true
		}
		return s.handleEditProfile(ctx, sessionCtx)
	case "g":
		return s.handleListGames(ctx, sessionCtx)
	case "s":
		return s.handleStatistics(ctx, sessionCtx)
	case "x":
		if !sessionCtx.IsAuthenticated {
			s.writeToSession(sessionCtx, "Please login first.\r\n")
			s.waitForKeypress(sessionCtx)
			return true
		}
		return s.handleResetSave(ctx, sessionCtx)
	case "q":
		s.writeToSession(sessionCtx, "Goodbye!\r\n")
		return false
	default:
		s.writeToSession(sessionCtx, "Invalid choice. Please try again.\r\n")
		s.waitForKeypress(sessionCtx)
		return true
	}
}

// handleLogin handles user login with retry logic
func (s *SSHServer) handleLogin(ctx context.Context, sessionCtx *SSHSessionContext) bool {
	maxLoginAttempts := s.getMaxLoginAttemptsWithDefault()
	attempts := 0

	for attempts < maxLoginAttempts {
		s.clearScreen(sessionCtx)
		s.writeToSession(sessionCtx, "=== Login ===\r\n\r\n")

		if attempts > 0 {
			s.writeToSession(sessionCtx, fmt.Sprintf("Login attempt %d of %d\r\n\r\n", attempts+1, maxLoginAttempts))
		}

		s.writeToSession(sessionCtx, "Username: ")
		username, err := s.readLineInput(sessionCtx)
		if err != nil {
			return false
		}

		s.writeToSession(sessionCtx, "Password: ")
		password, err := s.readPasswordInput(sessionCtx)
		if err != nil {
			s.writeToSession(sessionCtx, "Login cancelled.\r\n")
			s.waitForKeypress(sessionCtx)
			return true
		}

		// Track authentication attempt
		authStart := time.Now()
		s.promMetrics.AuthAttemptsTotal.WithLabelValues("password", username).Inc()

		// Wait for auth service to be available - spin until it comes back
		for s.sessionService.authMiddleware == nil {
			s.writeToSession(sessionCtx, "Authentication service is starting up, please wait...\r\n")
			time.Sleep(2 * time.Second)
			
			// Check if connection is still alive
			if sessionCtx.Channel == nil {
				return true // Connection was closed
			}
		}

		// Auth service is now available
		// Get client IP from connection
		clientIP := "unknown"
		if sessionCtx.ConnectionID != "" {
			s.connectionsMux.RLock()
			if conn, exists := s.connections[sessionCtx.ConnectionID]; exists {
				clientIP = conn.RemoteAddr
			}
			s.connectionsMux.RUnlock()
		}
		authenticatedUser, err := s.sessionService.authMiddleware.AuthenticateUser(ctx, username, password, clientIP)
		if err != nil {
			// Handle specific error types for better user feedback
			var errorMessage string
			var metricLabel string
			var shouldRetry = true

			switch {
			case strings.Contains(err.Error(), "user_not_found") || strings.Contains(err.Error(), "username_not_found"):
				errorMessage = "Username not found. Please check your username and try again.\r\n"
				metricLabel = "username_not_found"
			case strings.Contains(err.Error(), "invalid_credentials") || strings.Contains(err.Error(), "invalid_password"):
				errorMessage = "Incorrect password. Please try again.\r\n"
				metricLabel = "invalid_password"
			case strings.Contains(err.Error(), "account_locked"):
				errorMessage = "Account is temporarily locked due to too many failed login attempts. Please try again later.\r\n"
				metricLabel = "account_locked"
				shouldRetry = false // Don't allow retry for locked accounts
			default:
				errorMessage = "Login failed. Please try again.\r\n"
				metricLabel = "other_error"
			}

			s.promMetrics.AuthFailuresTotal.WithLabelValues("password", metricLabel).Inc()
			s.promMetrics.AuthDuration.Observe(time.Since(authStart).Seconds())
			s.writeToSession(sessionCtx, errorMessage)

			if !shouldRetry {
				s.waitForKeypress(sessionCtx)
				return true
			}

			attempts++
			if attempts < maxLoginAttempts {
				s.writeToSession(sessionCtx, "\r\nPress any key to try again...")
				s.waitForKeypress(sessionCtx)
				continue
			} else {
				s.writeToSession(sessionCtx, "\r\nMaximum login attempts exceeded.\r\n")
				s.waitForKeypress(sessionCtx)
				return true
			}
		}

		// Login successful - User is already in the correct format from auth middleware
		sessionCtx.IsAuthenticated = true
		sessionCtx.AuthenticatedUser = authenticatedUser
		sessionCtx.Username = authenticatedUser.Username

		// Track successful authentication
		s.promMetrics.AuthDuration.Observe(time.Since(authStart).Seconds())

		s.writeToSession(sessionCtx, "Login successful - Press any key to continue!\r\n")
		s.waitForKeypress(sessionCtx)
		return true
	}

	// This should never be reached
	return true
}

// handleRegisterEnhanced handles enhanced user registration with validation
func (s *SSHServer) handleRegisterEnhanced(ctx context.Context, sessionCtx *SSHSessionContext) bool {
	s.clearScreen(sessionCtx)
	s.writeToSession(sessionCtx, "╔═══════════════════════════════════════════════════════════╗\r\n")
	s.writeToSession(sessionCtx, "║                    User Registration                      ║\r\n")
	s.writeToSession(sessionCtx, "╚═══════════════════════════════════════════════════════════╝\r\n\r\n")

	// Get connection info for logging
	remoteAddr := "unknown"
	if sessionCtx.ConnectionID != "" {
		s.connectionsMux.RLock()
		if conn, exists := s.connections[sessionCtx.ConnectionID]; exists {
			remoteAddr = conn.RemoteAddr
		}
		s.connectionsMux.RUnlock()
	}

	// Step 1: Username
	s.writeToSession(sessionCtx, "Step 1/4: Choose your username\r\n")
	s.writeToSession(sessionCtx, "(3-30 characters, letters, numbers, and underscores only)\r\n\r\n")
	s.writeToSession(sessionCtx, "Username: ")
	username, err := s.readLineInput(sessionCtx)
	if err != nil {
		log.Printf("DEBUG: Username input error: %v", err)
		return false
	}

	log.Printf("DEBUG: Registration attempt - Username: %s, IP: %s", username, remoteAddr)

	// Step 2: Email (optional)
	s.writeToSession(sessionCtx, "\r\nStep 2/4: Email address (optional, press Enter to skip)\r\n")
	s.writeToSession(sessionCtx, "Email: ")
	email, err := s.readLineInput(sessionCtx)
	if err != nil {
		log.Printf("DEBUG: Email input error: %v", err)
		return false
	}

	// Step 3: Password
	s.writeToSession(sessionCtx, "\r\nStep 3/4: Choose a password\r\n")
	s.writeToSession(sessionCtx, "(minimum 6 characters)\r\n\r\n")
	s.writeToSession(sessionCtx, "Password: ")
	password, err := s.readPasswordInput(sessionCtx)
	if err != nil {
		log.Printf("DEBUG: Password input error: %v", err)
		s.writeToSession(sessionCtx, "Registration cancelled.\r\n")
		s.waitForKeypress(sessionCtx)
		return true
	}

	// Step 4: Confirm password
	s.writeToSession(sessionCtx, "\r\nStep 4/4: Confirm your password\r\n")
	s.writeToSession(sessionCtx, "Confirm password: ")
	confirmPassword, err := s.readPasswordInput(sessionCtx)
	if err != nil {
		log.Printf("DEBUG: Password confirmation error: %v", err)
		s.writeToSession(sessionCtx, "Registration cancelled.\r\n")
		s.waitForKeypress(sessionCtx)
		return true
	}

	// Basic validation
	if password != confirmPassword {
		s.writeToSession(sessionCtx, "\r\n❌ Passwords don't match. Please try again.\r\n")
		s.waitForKeypress(sessionCtx)
		return true
	}

	s.writeToSession(sessionCtx, "\r\n⏳ Creating your account...\r\n")

	// Create registration request - convert from user to session types
	req := &RegistrationRequest{
		Username:        username,
		Password:        password,
		PasswordConfirm: confirmPassword,
		Email:           email,
		RealName:        "",   // Not collected in enhanced version yet
		AcceptTerms:     true, // Implied by registration
		Source:          "ssh",
		IPAddress:       remoteAddr,
		UserAgent:       "dungeongate-ssh",
	}

	log.Printf("DEBUG: Calling user service registration for: %s", username)

	// Use the real user service for registration
	if s.sessionService.userService != nil {
		// Convert to user service registration request
		userReq := &user.RegistrationRequest{
			Username:        username,
			Password:        password,
			PasswordConfirm: confirmPassword,
			Email:           email,
			RealName:        "",   // Not collected in enhanced version yet
			AcceptTerms:     true, // Implied by registration
			Source:          "ssh",
			IPAddress:       remoteAddr,
			UserAgent:       "dungeongate-ssh",
		}

		// Direct call to user service
		response, err := s.sessionService.userService.RegisterUser(ctx, userReq)
		if err != nil {
			log.Printf("DEBUG: Registration failed for %s: %v", username, err)
			s.writeToSession(sessionCtx, fmt.Sprintf("❌ Registration failed: %v\r\n", err))
			s.waitForKeypress(sessionCtx)
			return true
		}

		if !response.Success {
			log.Printf("DEBUG: Registration validation failed for %s", username)
			s.writeToSession(sessionCtx, "❌ Registration failed:\r\n")
			for _, validationErr := range response.Errors {
				s.writeToSession(sessionCtx, fmt.Sprintf("   • %s: %s\r\n", validationErr.Field, validationErr.Message))
			}
			s.waitForKeypress(sessionCtx)
			return true
		}

		// Success!
		log.Printf("DEBUG: User registered successfully - ID: %d, Username: %s", response.User.ID, response.User.Username)

		// Verify in database
		if registeredUser, err := s.sessionService.userService.GetUserByUsername(ctx, username); err == nil {
			log.Printf("DEBUG: User verification - Found in DB: %s (ID: %d)", registeredUser.Username, registeredUser.ID)
		} else {
			log.Printf("DEBUG: User verification failed: %v", err)
		}

		s.writeToSession(sessionCtx, "\r\n✅ Registration successful!\r\n\r\n")
		s.writeToSession(sessionCtx, fmt.Sprintf("Welcome to DungeonGate, %s!\r\n", response.User.Username))
		s.writeToSession(sessionCtx, "You can now login with your credentials.\r\n\r\n")
		s.writeToSession(sessionCtx, "Press any key to continue...")
		s.waitForKeypress(sessionCtx)

		return true
	} else {
		// Fallback to mock user client (this should not happen in production)
		log.Printf("DEBUG: No user service available, using fallback mock client")
		response, err := s.userClient.RegisterUser(ctx, req)
		if err != nil {
			log.Printf("DEBUG: Mock registration failed for %s: %v", username, err)
			s.writeToSession(sessionCtx, fmt.Sprintf("❌ Registration failed: %v\r\n", err))
			s.waitForKeypress(sessionCtx)
			return true
		}

		log.Printf("DEBUG: Mock user registered: %s", response.User.Username)
		s.writeToSession(sessionCtx, "\r\n✅ Registration successful! (Mock)\r\n\r\n")
		s.writeToSession(sessionCtx, fmt.Sprintf("Welcome to DungeonGate, %s!\r\n", response.User.Username))
		s.writeToSession(sessionCtx, "You can now login with your credentials.\r\n\r\n")
		s.writeToSession(sessionCtx, "Press any key to continue...")
		s.waitForKeypress(sessionCtx)

		return true
	}
}

// handleRegister handles user registration
func (s *SSHServer) handleRegister(ctx context.Context, sessionCtx *SSHSessionContext) bool {
	s.clearScreen(sessionCtx)
	s.writeToSession(sessionCtx, "=== Register ===\r\n\r\n")

	s.writeToSession(sessionCtx, "Choose a username: ")
	username, err := s.readLineInput(sessionCtx)
	if err != nil {
		return false
	}

	s.writeToSession(sessionCtx, "Choose a password: ")
	password, err := s.readPasswordInput(sessionCtx)
	if err != nil {
		s.writeToSession(sessionCtx, "Registration cancelled.\r\n")
		s.waitForKeypress(sessionCtx)
		return true
	}

	s.writeToSession(sessionCtx, "Confirm password: ")
	confirmPassword, err := s.readPasswordInput(sessionCtx)
	if err != nil {
		s.writeToSession(sessionCtx, "Registration cancelled.\r\n")
		s.waitForKeypress(sessionCtx)
		return true
	}

	if password != confirmPassword {
		s.writeToSession(sessionCtx, "Passwords don't match. Please try again.\r\n")
		s.waitForKeypress(sessionCtx)
		return true
	}

	// Register with user service
	_, err = s.userClient.CreateUser(ctx, &CreateUserRequest{
		Username: username,
		Password: password,
	})
	if err != nil {
		s.writeToSession(sessionCtx, fmt.Sprintf("Registration failed: %v\r\n", err))
		s.waitForKeypress(sessionCtx)
		return true
	}

	s.writeToSession(sessionCtx, "Registration successful! You can now login.\r\n")
	s.waitForKeypress(sessionCtx)

	return true
}

// handlePlayGame handles game selection and launching
func (s *SSHServer) handlePlayGame(ctx context.Context, sessionCtx *SSHSessionContext) bool {
	games, err := s.gameClient.ListGames(ctx)
	if err != nil {
		s.writeToSession(sessionCtx, "Error retrieving games.\r\n")
		s.waitForKeypress(sessionCtx)
		return true
	}

	enabledGames := make([]*Game, 0)
	for _, game := range games {
		if game.Enabled {
			enabledGames = append(enabledGames, game)
		}
	}

	if len(enabledGames) == 0 {
		s.writeToSession(sessionCtx, "No games available.\r\n")
		s.waitForKeypress(sessionCtx)
		return true
	}

	s.clearScreen(sessionCtx)
	s.writeToSession(sessionCtx, "=== Available Games ===\r\n\r\n")

	for i, game := range enabledGames {
		gameDisplay := fmt.Sprintf("[%d] %s - %s", i+1, game.Name, game.Description)

		// Add save information for NetHack
		if game.ID == "nethack" && sessionCtx.IsAuthenticated {
			saveManager := NewSaveManager("/tmp/nethack-saves")
			if userSave, err := saveManager.GetUserSave(sessionCtx.Username, "nethack"); err == nil && userSave.HasSave {
				if userSave.SaveHash != "" {
					gameDisplay += fmt.Sprintf(" (Save Hash: %s)", userSave.SaveHash)
				} else {
					gameDisplay += " (Save Available)"
				}
			}
		}

		s.writeToSession(sessionCtx, gameDisplay+"\r\n")
	}

	s.writeToSession(sessionCtx, "\r\nChoice (0 to cancel): ")
	choice, err := s.readLineInput(sessionCtx)
	if err != nil {
		return false
	}

	gameID, err := strconv.Atoi(choice)
	if err != nil || gameID == 0 {
		return true
	}

	if gameID < 1 || gameID > len(enabledGames) {
		s.writeToSession(sessionCtx, "Invalid choice.\r\n")
		s.waitForKeypress(sessionCtx)
		return true
	}

	selectedGame := enabledGames[gameID-1]

	// Handle NetHack with save slot selection
	if selectedGame.ID == "nethack" {
		return s.handleNetHackGame(ctx, sessionCtx, selectedGame)
	}

	// Start the game
	s.writeToSession(sessionCtx, fmt.Sprintf("Starting %s...\r\n", selectedGame.Name))

	// Create game session
	var userID int
	var username string
	if sessionCtx.IsAuthenticated && sessionCtx.AuthenticatedUser != nil {
		userID = sessionCtx.AuthenticatedUser.ID
		username = sessionCtx.AuthenticatedUser.Username
	} else {
		userID = 0                     // Anonymous user
		username = sessionCtx.Username // Use the session username
	}

	gameSession, err := s.sessionService.CreateSession(ctx, &CreateSessionRequest{
		UserID:       userID,
		Username:     username,
		GameID:       selectedGame.ID,
		TerminalSize: fmt.Sprintf("%dx%d", sessionCtx.WindowSize.Width, sessionCtx.WindowSize.Height),
		Encoding:     "utf-8",
	})
	if err != nil {
		s.writeToSession(sessionCtx, fmt.Sprintf("Failed to create game session: %v\r\n", err))
		s.waitForKeypress(sessionCtx)
		return true
	}

	// Start game in PTY
	err = s.startGameInPTY(ctx, sessionCtx, selectedGame, gameSession)
	if err != nil {
		// Check if this is just a normal game exit
		if err == io.EOF || err.Error() == "PTY session has ended" || strings.Contains(err.Error(), "has ended") {
			// Game ended normally, this is not an error
			log.Printf("Game ended normally for user %s", sessionCtx.Username)
		} else {
			s.writeToSession(sessionCtx, fmt.Sprintf("Failed to start game: %v\r\n", err))
			s.waitForKeypress(sessionCtx)
		}
		return true
	}

	return true
}

// startGameInPTY starts a game in a PTY session
func (s *SSHServer) startGameInPTY(ctx context.Context, sessionCtx *SSHSessionContext, game *Game, gameSession *Session) error {
	// Track game start
	gameStart := time.Now()
	s.promMetrics.GamesStartedTotal.WithLabelValues(game.ID, game.Name).Inc()
	s.promMetrics.GamesActive.WithLabelValues(game.ID, game.Name).Inc()

	// Allocate PTY
	ptySession, err := s.ptyManager.AllocatePTY(sessionCtx.SessionID, sessionCtx.Username, game.ID, *sessionCtx.WindowSize)
	if err != nil {
		s.promMetrics.GameSessionErrors.WithLabelValues(game.ID, "pty_allocation").Inc()
		return fmt.Errorf("failed to allocate PTY: %w", err)
	}

	sessionCtx.ptySession = ptySession

	// Build game command
	command, args := s.buildGameCommand(game, sessionCtx)

	// Start game process
	if err := ptySession.StartCommand(command, args); err != nil {
		s.ptyManager.ReleasePTY(sessionCtx.SessionID)
		s.promMetrics.GameSessionErrors.WithLabelValues(game.ID, "start_failed").Inc()
		s.promMetrics.GamesActive.WithLabelValues(game.ID, game.Name).Dec()
		return fmt.Errorf("failed to start game: %w", err)
	}

	// Set up cleanup for when game ends
	defer func() {
		s.promMetrics.GamesActive.WithLabelValues(game.ID, game.Name).Dec()
		s.promMetrics.GameDuration.WithLabelValues(game.ID, game.Name).Observe(time.Since(gameStart).Seconds())
	}()

	s.writeToSession(sessionCtx, "Game started! Press Ctrl+C to exit.\r\n")
	time.Sleep(1 * time.Second)

	// Start I/O bridging with spectator support
	return s.bridgeGameIO(ctx, sessionCtx, ptySession, gameSession)
}

// buildGameCommand builds the command to start a game
func (s *SSHServer) buildGameCommand(game *Game, sessionCtx *SSHSessionContext) (string, []string) {
	// Use configured binary and args if available, but fix NetHack args
	if game.Binary != "" {
		args := make([]string, len(game.Args))
		copy(args, game.Args)

		// Special handling for NetHack to add username after -u
		if game.ID == "nethack" {
			for i, arg := range args {
				if arg == "-u" && i+1 < len(args) {
					// Username should already be there, but if it's not, add it
					if args[i+1] == "" || args[i+1] == "-u" {
						args[i+1] = sessionCtx.Username
					}
				} else if arg == "-u" && i+1 >= len(args) {
					// Add username after -u
					args = append(args, sessionCtx.Username)
				}
			}
			// If no -u flag found, add it with username
			hasUserFlag := false
			for _, arg := range args {
				if arg == "-u" {
					hasUserFlag = true
					break
				}
			}
			if !hasUserFlag {
				args = append(args, "-u", sessionCtx.Username)
			}
		}

		return game.Binary, args
	}

	// Fallback to predefined game commands
	switch game.ID {
	case "nethack":
		return "/usr/games/nethack", []string{"-u", sessionCtx.Username}
	case "dcss", "crawl":
		return "/usr/games/crawl", []string{}
	case "bash":
		return "/bin/bash", []string{"-l"}
	case "nano":
		return "/usr/bin/nano", []string{}
	default:
		// Default to a simple shell
		return "/bin/bash", []string{"-c", fmt.Sprintf("echo 'Welcome to %s!'; /bin/bash", game.Name)}
	}
}

// bridgeGameIO bridges I/O between SSH session and game PTY, with spectator broadcasting
func (s *SSHServer) bridgeGameIO(ctx context.Context, sessionCtx *SSHSessionContext, ptySession *PTYSession, gameSession *Session) error {
	// Create channels for coordination
	done := make(chan error, 2)

	// Start input forwarding (SSH -> PTY)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic in input forwarding: %v", r)
			}
		}()

		buffer := make([]byte, 1024)
		for {
			select {
			case <-ctx.Done():
				done <- ctx.Err()
				return
			case <-sessionCtx.done:
				done <- nil
				return
			default:
				n, err := sessionCtx.Channel.Read(buffer)
				if err != nil {
					if err == io.EOF {
						done <- nil
						return
					}
					done <- err
					return
				}

				// Update metrics
				sessionCtx.BytesRead += int64(n)
				sessionCtx.LastActivity = time.Now()

				// Check for exit signals
				if n == 1 && buffer[0] == 3 { // Ctrl+C
					s.writeToSession(sessionCtx, "\r\nExiting game...\r\n")
					ptySession.SendSignal(syscall.SIGTERM)
					done <- nil
					return
				}

				// Forward to PTY
				if err := ptySession.SendInput(buffer[:n]); err != nil {
					done <- err
					return
				}
			}
		}
	}()

	// Start output forwarding (PTY -> SSH)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic in output forwarding: %v", r)
			}
		}()

		for {
			select {
			case <-ctx.Done():
				done <- ctx.Err()
				return
			case <-sessionCtx.done:
				done <- nil
				return
			default:
				data, err := ptySession.ReadOutput()
				if err != nil {
					if err == io.EOF {
						done <- nil
						return
					}
					done <- err
					return
				}

				if len(data) == 0 {
					continue
				}

				// Forward to SSH (the player)
				if _, err := sessionCtx.Channel.Write(data); err != nil {
					done <- err
					return
				}

				// Broadcast to spectators using immutable data sharing
				if gameSession != nil {
					if err := s.sessionService.WriteToSession(ctx, gameSession.ID, data); err != nil {
						log.Printf("Failed to broadcast to spectators for session %s: %v", gameSession.ID, err)
						// Don't fail the whole game if spectator broadcasting fails
					}
				}

				// Update metrics
				sessionCtx.BytesWritten += int64(len(data))
				sessionCtx.LastActivity = time.Now()
			}
		}
	}()

	// Wait for completion
	err := <-done

	// Clean up
	if sessionCtx.ptySession != nil {
		sessionCtx.ptySession.Close()
		// Also release the PTY from the manager
		s.ptyManager.ReleasePTY(sessionCtx.SessionID)
		sessionCtx.ptySession = nil
	}

	s.writeToSession(sessionCtx, "\r\nGame session ended. Press any key to continue...\r\n")
	s.waitForKeypress(sessionCtx)

	return err
}

// handleWatchGames handles game spectating
func (s *SSHServer) handleWatchGames(ctx context.Context, sessionCtx *SSHSessionContext) bool {
	activeSessions, err := s.sessionService.GetActiveSessions(ctx)
	if err != nil {
		s.writeToSession(sessionCtx, "Error retrieving active games.\r\n")
		s.waitForKeypress(sessionCtx)
		return true
	}

	if len(activeSessions) == 0 {
		s.clearScreen(sessionCtx)
		s.writeToSession(sessionCtx, "╔══════════════════════════════════════════════════════════════════════════════╗\r\n")
		s.writeToSession(sessionCtx, "║                                  Watch Games                                 ║\r\n")
		s.writeToSession(sessionCtx, "╚══════════════════════════════════════════════════════════════════════════════╝\r\n\r\n")
		s.writeToSession(sessionCtx, "No active games to watch.\r\n\r\n")
		s.writeToSession(sessionCtx, "Press any key to continue...")
		s.waitForKeypress(sessionCtx)
		return true
	}

	s.clearScreen(sessionCtx)

	// Get terminal width for proper formatting
	terminalWidth := 80
	if sessionCtx.WindowSize != nil {
		terminalWidth = int(sessionCtx.WindowSize.Width)
	}

	// Load and display the watch menu banner
	bannerPath := s.getBannerPath("watch_menu")
	banner := s.loadWatchMenuBanner(bannerPath, terminalWidth, activeSessions)
	if banner != "" {
		s.writeToSession(sessionCtx, banner)
	} else {
		// Fallback banner
		s.writeToSession(sessionCtx, "╔══════════════════════════════════════════════════════════════════════════════╗\r\n")
		s.writeToSession(sessionCtx, "║                                  Watch Games                                 ║\r\n")
		s.writeToSession(sessionCtx, "╚══════════════════════════════════════════════════════════════════════════════╝\r\n\r\n")
		s.writeToSession(sessionCtx, "The following games are in progress:\r\n\r\n")
		s.writeToSession(sessionCtx, "     Username         Game    Size     Start date & time     Idle time  Watch\r\n")
	}

	// Display session list with proper formatting
	s.displayWatchGamesList(sessionCtx, activeSessions)

	s.writeToSession(sessionCtx, fmt.Sprintf("\r\n(%d-%d of %d)\r\n\r\n", 1, len(activeSessions), len(activeSessions)))
	s.writeToSession(sessionCtx, "Watch which game? ('?' for help) => ")

	choice, err := s.readUserInput(sessionCtx)
	if err != nil {
		return false
	}

	// Handle special commands
	if choice == "?" {
		s.showWatchHelp(sessionCtx)
		return true
	}

	if choice == "q" || choice == "" {
		return true
	}

	// Convert letter choice to index (a=0, b=1, etc.)
	if len(choice) == 1 && choice[0] >= 'a' && choice[0] <= 'z' {
		sessionIndex := int(choice[0] - 'a')
		if sessionIndex >= 0 && sessionIndex < len(activeSessions) {
			selectedSession := activeSessions[sessionIndex]
			return s.startSpectating(ctx, sessionCtx, selectedSession)
		}
	}

	s.writeToSession(sessionCtx, "Invalid choice. Please try again.\r\n")
	s.waitForKeypress(sessionCtx)
	return true
}

// loadWatchMenuBanner loads the watch menu banner and processes it
func (s *SSHServer) loadWatchMenuBanner(bannerPath string, terminalWidth int, sessions []*Session) string {
	if bannerPath == "" {
		return ""
	}

	// Read banner file
	content, err := os.ReadFile(bannerPath)
	if err != nil {
		log.Printf("Failed to read watch menu banner file %s: %v", bannerPath, err)
		return ""
	}

	bannerText := string(content)

	// Resize banner to fit terminal width
	resizedBanner := s.resizeBanner(bannerText, terminalWidth)

	return resizedBanner + "\r\n"
}

// displayWatchGamesList displays the formatted list of active games
func (s *SSHServer) displayWatchGamesList(sessionCtx *SSHSessionContext, sessions []*Session) {
	for i, session := range sessions {
		// Format: a) Username Game Size Start date & time Idle time Watchers
		letter := string(rune('a' + i))

		// Truncate username to fit in column
		username := session.Username
		if len(username) > 15 {
			username = username[:12] + "..."
		}

		// Map game IDs to short names
		gameDisplay := session.GameID
		switch session.GameID {
		case "nethack":
			gameDisplay = "NH370"
		case "dcss", "crawl":
			gameDisplay = "DCSS"
		case "bash":
			gameDisplay = "SHELL"
		}

		// Get terminal size
		termSize := session.TerminalSize
		if termSize == "" {
			termSize = "80x24"
		}

		// Calculate time since start and idle time
		startTime := session.StartTime.Format("2006-01-02 15:04:05")
		idleTime := ""
		if time.Since(session.LastActivity) > time.Minute {
			idle := time.Since(session.LastActivity)
			if idle > time.Hour {
				idleTime = fmt.Sprintf("%dh %dm", int(idle.Hours()), int(idle.Minutes())%60)
			} else if idle > time.Minute {
				idleTime = fmt.Sprintf("%dm %ds", int(idle.Minutes()), int(idle.Seconds())%60)
			} else {
				idleTime = fmt.Sprintf("%ds", int(idle.Seconds()))
			}
		}

		// Count spectators
		spectatorCount := len(session.Spectators)

		// Format the line with proper spacing
		line := fmt.Sprintf("%s) %-15s %-6s %-8s %s %-10s %d",
			letter, username, gameDisplay, termSize, startTime, idleTime, spectatorCount)

		s.writeToSession(sessionCtx, line+"\r\n")
	}
}

// showWatchHelp displays help for the watch command
func (s *SSHServer) showWatchHelp(sessionCtx *SSHSessionContext) {
	s.clearScreen(sessionCtx)
	s.writeToSession(sessionCtx, "╔══════════════════════════════════════════════════════════════════════════════╗\r\n")
	s.writeToSession(sessionCtx, "║                                Watch Games Help                              ║\r\n")
	s.writeToSession(sessionCtx, "╚══════════════════════════════════════════════════════════════════════════════╝\r\n\r\n")
	s.writeToSession(sessionCtx, "Commands:\r\n")
	s.writeToSession(sessionCtx, "  a-z    : Watch the corresponding game (e.g., 'a' for first game)\r\n")
	s.writeToSession(sessionCtx, "  q      : Return to main menu\r\n")
	s.writeToSession(sessionCtx, "  ?      : Show this help\r\n")
	s.writeToSession(sessionCtx, "  Enter  : Return to main menu\r\n\r\n")
	s.writeToSession(sessionCtx, "While spectating:\r\n")
	s.writeToSession(sessionCtx, "  Ctrl+C : Stop spectating and return to watch menu\r\n\r\n")
	s.writeToSession(sessionCtx, "Press any key to continue...")
	s.waitForKeypress(sessionCtx)
}

// startSpectating initiates spectating for a selected session
func (s *SSHServer) startSpectating(ctx context.Context, sessionCtx *SSHSessionContext, selectedSession *Session) bool {
	// Get user info for spectator
	userID := 0
	username := "anonymous"
	if sessionCtx.IsAuthenticated && sessionCtx.AuthenticatedUser != nil {
		userID = sessionCtx.AuthenticatedUser.ID
		username = sessionCtx.AuthenticatedUser.Username
	}

	// Create SSH spectator connection
	spectatorConnection := NewSSHSpectatorConnection(sessionCtx)

	// Add spectator with connection
	err := s.sessionService.AddSpectatorWithConnection(ctx, selectedSession.ID, userID, username, spectatorConnection)
	if err != nil {
		s.writeToSession(sessionCtx, fmt.Sprintf("Failed to start spectating: %v\r\n", err))
		s.waitForKeypress(sessionCtx)
		return true
	}

	// Run spectating loop
	return s.runSpectatorSession(ctx, sessionCtx, selectedSession.ID, userID)
}

// runSpectatorSession handles the spectating session loop
func (s *SSHServer) runSpectatorSession(ctx context.Context, sessionCtx *SSHSessionContext, gameSessionID string, userID int) bool {
	// Create a channel to handle user input for exiting
	exitChan := make(chan struct{})

	// Initialize terminal for spectating
	// Clear screen and set up terminal properly
	s.writeToSession(sessionCtx, "\033[2J\033[H") // Clear screen and home cursor
	s.writeToSession(sessionCtx, "\033[?1049h")   // Switch to alternate screen buffer (like vim/less)

	// Start goroutine to handle user input (Ctrl+C to exit)
	go func() {
		buffer := make([]byte, 1)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				n, err := sessionCtx.Channel.Read(buffer)
				if err != nil {
					log.Printf("Spectator input read error for user %d: %v", userID, err)
					select {
					case exitChan <- struct{}{}:
					default:
					}
					return
				}

				if n > 0 && n <= len(buffer) {
					char := buffer[0]
					if char == 3 { // Ctrl+C
						select {
						case exitChan <- struct{}{}:
						default:
						}
						return
					}
					// Ignore other input while spectating
				}
			}
		}
	}()

	// Note: The actual game data is being written directly to sessionCtx.Channel
	// by the SSHSpectatorConnection.Write() method, so we just need to wait
	// for the user to exit

	// Wait for exit signal or context cancellation
	var exitReason string
	select {
	case <-exitChan:
		exitReason = "user_exit"
		// Restore terminal state
		s.writeToSession(sessionCtx, "\033[?1049l")   // Switch back from alternate screen buffer
		s.writeToSession(sessionCtx, "\033[2J\033[H") // Clear screen
		s.writeToSession(sessionCtx, "Exiting spectator mode...\r\n")
	case <-ctx.Done():
		exitReason = "context_cancelled"
		// Restore terminal state
		s.writeToSession(sessionCtx, "\033[?1049l")   // Switch back from alternate screen buffer
		s.writeToSession(sessionCtx, "\033[2J\033[H") // Clear screen
		s.writeToSession(sessionCtx, "Connection terminated.\r\n")
	}

	// Always attempt to remove spectator with retry logic
	const maxCleanupRetries = 3
	var cleanupErr error
	for i := 0; i < maxCleanupRetries; i++ {
		cleanupErr = s.sessionService.RemoveSpectator(ctx, gameSessionID, userID)
		if cleanupErr == nil {
			log.Printf("Successfully removed spectator %d from session %s (exit reason: %s)", userID, gameSessionID, exitReason)
			break
		}
		log.Printf("Failed to remove spectator %d from session %s (attempt %d/%d): %v", userID, gameSessionID, i+1, maxCleanupRetries, cleanupErr)
		if i < maxCleanupRetries-1 {
			time.Sleep(time.Duration(100*(i+1)) * time.Millisecond)
		}
	}

	if cleanupErr != nil {
		log.Printf("CRITICAL: Failed to remove spectator %d from session %s after %d attempts: %v", userID, gameSessionID, maxCleanupRetries, cleanupErr)
	}

	s.writeToSession(sessionCtx, "Press any key to continue...")
	s.waitForKeypress(sessionCtx)

	return true
}

// handleListGames handles game listing
func (s *SSHServer) handleListGames(ctx context.Context, sessionCtx *SSHSessionContext) bool {
	games, err := s.gameClient.ListGames(ctx)
	if err != nil {
		s.writeToSession(sessionCtx, "Error retrieving games.\r\n")
		s.waitForKeypress(sessionCtx)
		return true
	}

	s.clearScreen(sessionCtx)
	s.writeToSession(sessionCtx, "=== Available Games ===\r\n\r\n")

	if len(games) == 0 {
		s.writeToSession(sessionCtx, "No games configured.\r\n")
	} else {
		for _, game := range games {
			status := "🔴 Disabled"
			if game.Enabled {
				status = "🟢 Available"
			}
			s.writeToSession(sessionCtx, fmt.Sprintf("%-20s %s\r\n", game.Name, status))
			if game.Description != "" {
				s.writeToSession(sessionCtx, fmt.Sprintf("    %s\r\n", game.Description))
			}
			s.writeToSession(sessionCtx, "\r\n")
		}
	}

	s.writeToSession(sessionCtx, "\r\nPress any key to continue...")
	s.waitForKeypress(sessionCtx)

	return true
}

// handleEditProfile handles profile editing
func (s *SSHServer) handleEditProfile(ctx context.Context, sessionCtx *SSHSessionContext) bool {
	s.clearScreen(sessionCtx)
	s.writeToSession(sessionCtx, "=== Edit Profile ===\r\n\r\n")
	s.writeToSession(sessionCtx, "Profile editing not yet implemented.\r\n")
	s.waitForKeypress(sessionCtx)
	return true
}

// handleViewRecordings handles recording viewing
func (s *SSHServer) handleViewRecordings(ctx context.Context, sessionCtx *SSHSessionContext) bool {
	s.clearScreen(sessionCtx)
	s.writeToSession(sessionCtx, "=== View Recordings ===\r\n\r\n")
	s.writeToSession(sessionCtx, "Recording viewing not yet implemented.\r\n")
	s.waitForKeypress(sessionCtx)
	return true
}

// handleStatistics handles statistics viewing
func (s *SSHServer) handleStatistics(ctx context.Context, sessionCtx *SSHSessionContext) bool {
	s.clearScreen(sessionCtx)
	s.writeToSession(sessionCtx, "=== Statistics ===\r\n\r\n")

	// Get system metrics
	metrics := s.sessionService.GetMetrics()
	sshMetrics := s.GetMetrics()

	s.writeToSession(sessionCtx, fmt.Sprintf("System Statistics:\r\n"))
	s.writeToSession(sessionCtx, fmt.Sprintf("  Active Sessions: %d\r\n", metrics.ActiveSessions))
	s.writeToSession(sessionCtx, fmt.Sprintf("  Total Sessions: %d\r\n", metrics.TotalSessions))
	s.writeToSession(sessionCtx, fmt.Sprintf("  Active Spectators: %d\r\n", metrics.ActiveSpectators))
	s.writeToSession(sessionCtx, fmt.Sprintf("  Total Spectators: %d\r\n", metrics.TotalSpectators))
	s.writeToSession(sessionCtx, fmt.Sprintf("  Bytes Transferred: %d\r\n", metrics.BytesTransferred))
	s.writeToSession(sessionCtx, fmt.Sprintf("  Uptime: %d seconds\r\n", metrics.UptimeSeconds))

	s.writeToSession(sessionCtx, fmt.Sprintf("\r\nSSH Statistics:\r\n"))
	s.writeToSession(sessionCtx, fmt.Sprintf("  Total Connections: %d\r\n", sshMetrics.TotalConnections))
	s.writeToSession(sessionCtx, fmt.Sprintf("  Active Connections: %d\r\n", sshMetrics.ActiveConnections))
	s.writeToSession(sessionCtx, fmt.Sprintf("  Failed Connections: %d\r\n", sshMetrics.FailedConnections))
	s.writeToSession(sessionCtx, fmt.Sprintf("  Total Sessions: %d\r\n", sshMetrics.TotalSessions))
	s.writeToSession(sessionCtx, fmt.Sprintf("  Active Sessions: %d\r\n", sshMetrics.ActiveSessions))

	s.writeToSession(sessionCtx, "\r\nPress any key to continue...")
	s.waitForKeypress(sessionCtx)
	return true
}

// Utility functions

// readLineInput reads a line of input from the SSH session
func (s *SSHServer) readLineInput(sessionCtx *SSHSessionContext) (string, error) {
	var line strings.Builder
	buffer := make([]byte, 1)

	for {
		n, err := sessionCtx.Channel.Read(buffer)
		if err != nil {
			return "", err
		}

		if n == 0 {
			continue
		}

		sessionCtx.BytesRead += int64(n)
		sessionCtx.LastActivity = time.Now()

		char := buffer[0]

		switch char {
		case '\r', '\n':
			// End of input
			s.writeToSession(sessionCtx, "\r\n")
			return line.String(), nil
		case '\b', 127: // Backspace
			if line.Len() > 0 {
				lineStr := line.String()
				line.Reset()
				line.WriteString(lineStr[:len(lineStr)-1])
				s.writeToSession(sessionCtx, "\b \b")
			}
		case 3: // Ctrl+C
			return "", fmt.Errorf("interrupted")
		default:
			line.WriteByte(char)
			s.writeToSession(sessionCtx, string(char))
		}
	}
}

// writeToSession writes data to the SSH session
func (s *SSHServer) writeToSession(sessionCtx *SSHSessionContext, data string) {
	if sessionCtx.Channel == nil {
		log.Printf("Warning: Attempting to write to session %s with nil channel", sessionCtx.SessionID)
		return
	}
	if _, err := sessionCtx.Channel.Write([]byte(data)); err != nil {
		log.Printf("Error writing to session %s: %v", sessionCtx.SessionID, err)
	}
	sessionCtx.BytesWritten += int64(len(data))
	sessionCtx.LastActivity = time.Now()
}

// clearScreen clears the terminal screen
func (s *SSHServer) clearScreen(sessionCtx *SSHSessionContext) {
	s.writeToSession(sessionCtx, "\033[2J\033[H")
}

// waitForKeypress waits for a keypress
func (s *SSHServer) waitForKeypress(sessionCtx *SSHSessionContext) {
	buffer := make([]byte, 1)
	sessionCtx.Channel.Read(buffer)
	sessionCtx.LastActivity = time.Now()
}

// Authentication handlers

// handlePasswordAuth handles password authentication
func (s *SSHServer) handlePasswordAuth(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
	// Allow all connections - we handle authentication in the menu
	return &ssh.Permissions{}, nil
}

// handlePublicKeyAuth handles public key authentication
func (s *SSHServer) handlePublicKeyAuth(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	// TODO: Implement public key authentication
	return nil, fmt.Errorf("public key authentication not implemented")
}

// handleBanner returns the SSH banner
func (s *SSHServer) handleBanner(conn ssh.ConnMetadata) string {
	if s.config != nil && s.config.SSH != nil {
		return s.config.SSH.Banner
	}
	return "Welcome to DungeonGate!\r\n"
}

// Host key management

// loadOrGenerateHostKey loads or generates SSH host key
func (s *SSHServer) loadOrGenerateHostKey() (ssh.Signer, error) {
	// Use configured path, which already has a default value from config
	keyPath := s.config.SSH.HostKeyPath
	if keyPath == "" {
		// This should never happen as config provides defaults, but just in case
		keyPath = "./ssh_host_rsa_key"
	}

	// Try to load existing key
	if _, err := os.Stat(keyPath); err == nil {
		keyData, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read host key: %w", err)
		}

		key, err := ssh.ParsePrivateKey(keyData)
		if err != nil {
			return nil, fmt.Errorf("failed to parse host key: %w", err)
		}

		log.Printf("Loaded SSH host key from %s", keyPath)
		return key, nil
	}

	// Generate new key
	log.Printf("Generating new SSH host key at %s", keyPath)
	return s.generateHostKey(keyPath)
}

// generateHostKey generates a new SSH host key
func (s *SSHServer) generateHostKey(keyPath string) (ssh.Signer, error) {
	// Generate RSA key
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// Convert to PEM format
	keyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(keyPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Write private key to file
	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to create key file: %w", err)
	}
	defer keyFile.Close()

	if err := pem.Encode(keyFile, keyPEM); err != nil {
		return nil, fmt.Errorf("failed to write key file: %w", err)
	}

	// Convert to SSH signer
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create signer: %w", err)
	}

	return signer, nil
}

// Background services

// startMetricsCollection starts metrics collection
func (s *SSHServer) startMetricsCollection(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.metrics.mutex.RLock()
			log.Printf("SSH Metrics: total_conn=%d active_conn=%d failed_conn=%d total_sess=%d active_sess=%d",
				s.metrics.TotalConnections, s.metrics.ActiveConnections, s.metrics.FailedConnections,
				s.metrics.TotalSessions, s.metrics.ActiveSessions)
			s.metrics.mutex.RUnlock()
		}
	}
}

// startConnectionCleanup starts connection cleanup
func (s *SSHServer) startConnectionCleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.cleanupIdleConnections()
		}
	}
}

// cleanupIdleConnections cleans up idle connections
func (s *SSHServer) cleanupIdleConnections() {
	s.connectionsMux.Lock()
	defer s.connectionsMux.Unlock()

	now := time.Now()
	idleTimeout := 30 * time.Minute

	for connID, conn := range s.connections {
		if now.Sub(conn.LastActivity) > idleTimeout {
			log.Printf("Cleaning up idle connection: %s", connID)

			// Close all sessions in this connection
			conn.sessionsMux.Lock()
			for sessionID, session := range conn.Sessions {
				log.Printf("Closing idle session: %s", sessionID)
				close(session.done)
			}
			conn.sessionsMux.Unlock()

			delete(s.connections, connID)
		}
	}
}

// Utility functions for ID generation

// generateConnectionID generates a unique connection ID
func generateConnectionID() string {
	return fmt.Sprintf("conn_%d_%d", time.Now().UnixNano(), rand2.Int63n(10000))
}

// generateSSHSessionID generates a unique SSH session ID
func generateSSHSessionID() string {
	return fmt.Sprintf("ssh_sess_%d_%d", time.Now().UnixNano(), rand2.Int63n(10000))
}

// GetMetrics returns SSH server metrics
func (s *SSHServer) GetMetrics() *SSHMetrics {
	s.metrics.mutex.RLock()
	defer s.metrics.mutex.RUnlock()

	return &SSHMetrics{
		TotalConnections:  s.metrics.TotalConnections,
		ActiveConnections: s.metrics.ActiveConnections,
		FailedConnections: s.metrics.FailedConnections,
		TotalSessions:     s.metrics.TotalSessions,
		ActiveSessions:    s.metrics.ActiveSessions,
		BytesTransferred:  s.metrics.BytesTransferred,
	}
}

// GetActiveConnections returns active connection information
func (s *SSHServer) GetActiveConnections() map[string]*SSHConnection {
	s.connectionsMux.RLock()
	defer s.connectionsMux.RUnlock()

	result := make(map[string]*SSHConnection)
	for id, conn := range s.connections {
		result[id] = conn
	}
	return result
}

// Shutdown gracefully shuts down the SSH server
func (s *SSHServer) Shutdown(ctx context.Context) error {
	log.Println("Shutting down SSH server...")

	// Close all active connections
	s.connectionsMux.Lock()
	for connID, conn := range s.connections {
		log.Printf("Closing connection: %s", connID)

		conn.sessionsMux.Lock()
		for sessionID, session := range conn.Sessions {
			log.Printf("Closing session: %s", sessionID)
			close(session.done)
		}
		conn.sessionsMux.Unlock()
	}
	s.connectionsMux.Unlock()

	// Clean up PTY manager
	if s.ptyManager != nil {
		s.ptyManager.Shutdown()
	}

	return nil
}

// handleNetHackGame handles NetHack with auto-load/new game logic
func (s *SSHServer) handleNetHackGame(ctx context.Context, sessionCtx *SSHSessionContext, game *Game) bool {
	// Initialize save manager
	saveManager := NewSaveManager("/tmp/nethack-saves")

	// Check if user has an existing save
	userSave, err := saveManager.GetUserSave(sessionCtx.Username, "nethack")
	if err != nil {
		s.writeToSession(sessionCtx, fmt.Sprintf("Error checking save data: %v\r\n", err))
		s.waitForKeypress(sessionCtx)
		return true
	}

	if userSave.HasSave {
		// Auto-load existing save
		size := fmt.Sprintf("%.1f KB", float64(userSave.FileSize)/1024)
		hashInfo := ""
		if userSave.SaveHash != "" {
			hashInfo = fmt.Sprintf(" [%s]", userSave.SaveHash)
		}
		s.writeToSession(sessionCtx, fmt.Sprintf("Loading your NetHack game... (%s%s, last played: %s)\r\n",
			size, hashInfo, userSave.UpdatedAt.Format("2006-01-02 15:04:05")))
	} else {
		// Start new game
		s.writeToSession(sessionCtx, "Starting new NetHack game...\r\n")
	}

	// Start NetHack
	return s.startNetHackWithSave(ctx, sessionCtx, game)
}

// startNetHackWithSave starts NetHack with proper save environment setup
func (s *SSHServer) startNetHackWithSave(ctx context.Context, sessionCtx *SSHSessionContext, game *Game) bool {
	saveManager := NewSaveManager("/tmp/nethack-saves")

	// Get save environment for this user
	saveEnv, err := saveManager.PrepareUserSaveEnvironment(sessionCtx.Username, "nethack")
	if err != nil {
		s.writeToSession(sessionCtx, fmt.Sprintf("Failed to prepare save environment: %v\r\n", err))
		s.waitForKeypress(sessionCtx)
		return true
	}

	// Update game environment with save-specific settings
	gameWithSave := *game
	if gameWithSave.Environment == nil {
		gameWithSave.Environment = make(map[string]string)
	}

	// Merge save environment
	for key, value := range saveEnv {
		gameWithSave.Environment[key] = value
	}

	// Create game session
	var userID int
	var username string
	if sessionCtx.IsAuthenticated && sessionCtx.AuthenticatedUser != nil {
		userID = sessionCtx.AuthenticatedUser.ID
		username = sessionCtx.AuthenticatedUser.Username
	} else {
		userID = 0                     // Anonymous user
		username = sessionCtx.Username // Use the session username
	}

	gameSession, err := s.sessionService.CreateSession(ctx, &CreateSessionRequest{
		UserID:       userID,
		Username:     username,
		GameID:       gameWithSave.ID,
		TerminalSize: fmt.Sprintf("%dx%d", sessionCtx.WindowSize.Width, sessionCtx.WindowSize.Height),
		Encoding:     "utf-8",
	})
	if err != nil {
		s.writeToSession(sessionCtx, fmt.Sprintf("Failed to create game session: %v\r\n", err))
		s.waitForKeypress(sessionCtx)
		return true
	}

	// Start game in PTY
	err = s.startGameInPTY(ctx, sessionCtx, &gameWithSave, gameSession)
	if err != nil {
		// Check if this is just a normal game exit
		if err == io.EOF || err.Error() == "PTY session has ended" || strings.Contains(err.Error(), "has ended") {
			// Game ended normally, this is not an error
			log.Printf("NetHack game ended normally for user %s", sessionCtx.Username)
		} else {
			s.writeToSession(sessionCtx, fmt.Sprintf("Failed to start game: %v\r\n", err))
			s.waitForKeypress(sessionCtx)
		}
		return true
	}

	return true
}

// handleResetSave handles resetting a user's NetHack save
func (s *SSHServer) handleResetSave(ctx context.Context, sessionCtx *SSHSessionContext) bool {
	saveManager := NewSaveManager("/tmp/nethack-saves")

	s.clearScreen(sessionCtx)
	s.writeToSession(sessionCtx, "=== Reset NetHack Save ===\r\n\r\n")

	// Check if user has an existing save
	userSave, err := saveManager.GetUserSave(sessionCtx.Username, "nethack")
	if err != nil {
		s.writeToSession(sessionCtx, fmt.Sprintf("Error checking save data: %v\r\n", err))
		s.waitForKeypress(sessionCtx)
		return true
	}

	if !userSave.HasSave {
		s.writeToSession(sessionCtx, "You don't have any NetHack save data to reset.\r\n")
		s.waitForKeypress(sessionCtx)
		return true
	}

	// Show save info and confirm deletion
	size := fmt.Sprintf("%.1f KB", float64(userSave.FileSize)/1024)
	hashInfo := ""
	if userSave.SaveHash != "" {
		hashInfo = fmt.Sprintf(" (Hash: %s)", userSave.SaveHash)
	}

	s.writeToSession(sessionCtx, fmt.Sprintf("Current save: %s%s\r\n", size, hashInfo))
	s.writeToSession(sessionCtx, fmt.Sprintf("Last played: %s\r\n", userSave.UpdatedAt.Format("2006-01-02 15:04:05")))
	s.writeToSession(sessionCtx, fmt.Sprintf("Save file: %s\r\n\r\n", userSave.SavePath))

	s.writeToSession(sessionCtx, "⚠️  WARNING: This will permanently delete your NetHack save game! ⚠️\r\n")
	s.writeToSession(sessionCtx, "This action cannot be undone. A backup will be created first.\r\n\r\n")
	s.writeToSession(sessionCtx, "Type 'DELETE' to confirm or anything else to cancel: ")

	choice, err := s.readLineInput(sessionCtx)
	if err != nil {
		return false
	}

	choice = strings.TrimSpace(choice)
	if choice != "DELETE" {
		s.writeToSession(sessionCtx, "Reset cancelled.\r\n")
		s.waitForKeypress(sessionCtx)
		return true
	}

	// Create backup before deletion
	s.writeToSession(sessionCtx, "Creating backup...\r\n")
	if err := saveManager.BackupUserSave(sessionCtx.Username, "nethack"); err != nil {
		s.writeToSession(sessionCtx, fmt.Sprintf("Warning: Failed to create backup: %v\r\n", err))
	}

	// Delete the save
	s.writeToSession(sessionCtx, "Deleting save data...\r\n")
	if err := saveManager.DeleteUserSave(sessionCtx.Username, "nethack"); err != nil {
		s.writeToSession(sessionCtx, fmt.Sprintf("Error deleting save: %v\r\n", err))
		s.waitForKeypress(sessionCtx)
		return true
	}

	s.writeToSession(sessionCtx, "NetHack save has been reset! You can now start a fresh game.\r\n")
	s.waitForKeypress(sessionCtx)
	return true
}

// getMaxLoginAttempts returns the maximum login attempts from config
// getMaxLoginAttempts returns the maximum login attempts from config
// getMaxLoginAttemptsWithDefault returns the maximum login attempts from config with default handling
func (s *SSHServer) getMaxLoginAttemptsWithDefault() int {
	if s.config != nil && s.config.User != nil && s.config.User.LoginAttempts != nil {
		if s.config.User.LoginAttempts.MaxAttempts > 0 {
			return s.config.User.LoginAttempts.MaxAttempts
		}
	}
	return 3 // Default to 3 attempts
}
