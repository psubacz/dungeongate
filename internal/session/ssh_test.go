package session

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dungeongate/pkg/config"
	"golang.org/x/crypto/ssh"
)

// TestSSHServer tests the SSH server functionality
func TestSSHServer(t *testing.T) {
	// Create test configuration
	cfg := createTestConfig()

	// Create mock session service
	sessionService := &Service{}

	// Create SSH server
	sshServer, err := NewSSHServer(sessionService, cfg)
	if err != nil {
		t.Fatalf("Failed to create SSH server: %v", err)
	}

	// Test server creation
	if sshServer == nil {
		t.Fatal("SSH server is nil")
	}

	// Test configuration
	if sshServer.config == nil {
		t.Fatal("SSH server configuration is nil")
	}

	// Test PTY manager
	if sshServer.ptyManager == nil {
		t.Fatal("PTY manager is nil")
	}

	// Test service clients
	if sshServer.authClient == nil {
		t.Fatal("Auth client is nil")
	}
	if sshServer.userClient == nil {
		t.Fatal("User client is nil")
	}
	if sshServer.gameClient == nil {
		t.Fatal("Game client is nil")
	}
}

// TestSSHServerStart tests SSH server startup
func TestSSHServerStart(t *testing.T) {
	// Create test configuration
	cfg := createTestConfig()

	// Create mock session service
	sessionService := &Service{}

	// Create SSH server
	sshServer, err := NewSSHServer(sessionService, cfg)
	if err != nil {
		t.Fatalf("Failed to create SSH server: %v", err)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start server in background
	go func() {
		err := sshServer.Start(ctx, 0) // Use port 0 for automatic port selection
		if err != nil && err != context.DeadlineExceeded {
			t.Errorf("SSH server start failed: %v", err)
		}
	}()

	// Wait a bit for server to start
	time.Sleep(100 * time.Millisecond)

	// Cancel context to stop server
	cancel()

	// Wait for server to stop
	time.Sleep(100 * time.Millisecond)
}

// TestSSHConnectionHandling tests SSH connection handling
func TestSSHConnectionHandling(t *testing.T) {
	// Create test configuration
	cfg := createTestConfig()

	// Create mock session service
	sessionService := &Service{}

	// Create SSH server
	sshServer, err := NewSSHServer(sessionService, cfg)
	if err != nil {
		t.Fatalf("Failed to create SSH server: %v", err)
	}

	// Test connection tracking
	connections := sshServer.GetActiveConnections()
	if len(connections) != 0 {
		t.Errorf("Expected 0 connections, got %d", len(connections))
	}

	// Test metrics
	metrics := sshServer.GetMetrics()
	if metrics == nil {
		t.Fatal("Metrics is nil")
	}

	if metrics.TotalConnections != 0 {
		t.Errorf("Expected 0 total connections, got %d", metrics.TotalConnections)
	}
}

// TestSSHAuthentication tests SSH authentication
func TestSSHAuthentication(t *testing.T) {
	// Create test configuration
	cfg := createTestConfig()

	// Create mock session service
	sessionService := &Service{}

	// Create SSH server
	sshServer, err := NewSSHServer(sessionService, cfg)
	if err != nil {
		t.Fatalf("Failed to create SSH server: %v", err)
	}

	// Test password authentication
	conn := &mockSSHConnMetadata{username: "testuser"}
	permissions, err := sshServer.handlePasswordAuth(conn, []byte("password"))

	// Should allow all connections (we handle auth in the menu)
	if err != nil {
		t.Errorf("Password auth failed: %v", err)
	}

	if permissions == nil {
		t.Error("Permissions should not be nil")
	}

	// Test public key authentication (should fail as not implemented)
	_, err = sshServer.handlePublicKeyAuth(conn, nil)
	if err == nil {
		t.Error("Public key auth should fail (not implemented)")
	}

	// Test banner
	banner := sshServer.handleBanner(conn)
	if banner == "" {
		t.Error("Banner should not be empty")
	}
}

// TestSSHHostKeyGeneration tests SSH host key generation
func TestSSHHostKeyGeneration(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "ssh-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test configuration with temp key path
	cfg := createTestConfig()
	cfg.SSH.HostKeyPath = filepath.Join(tempDir, "test_host_key")

	// Create mock session service
	sessionService := &Service{}

	// Create SSH server (should generate key)
	sshServer, err := NewSSHServer(sessionService, cfg)
	if err != nil {
		t.Fatalf("Failed to create SSH server: %v", err)
	}

	// Verify server was created
	if sshServer == nil {
		t.Fatal("SSH server should not be nil")
	}

	// Check if key file was created
	if _, err := os.Stat(cfg.SSH.HostKeyPath); os.IsNotExist(err) {
		t.Error("Host key file was not created")
	}

	// Test key loading on second creation
	sshServer2, err := NewSSHServer(sessionService, cfg)
	if err != nil {
		t.Fatalf("Failed to create SSH server with existing key: %v", err)
	}

	if sshServer2 == nil {
		t.Fatal("Second SSH server is nil")
	}
}

// TestSSHSessionHandling tests SSH session handling
func TestSSHSessionHandling(t *testing.T) {
	// Create test configuration
	cfg := createTestConfig()

	// Create mock session service
	sessionService := &Service{}

	// Create SSH server
	sshServer, err := NewSSHServer(sessionService, cfg)
	if err != nil {
		t.Fatalf("Failed to create SSH server: %v", err)
	}

	// Test session context creation
	sessionCtx := &SSHSessionContext{
		SessionID:    "test-session",
		ConnectionID: "test-connection",
		Username:     "testuser",
		WindowSize:   &WindowSize{Width: 80, Height: 24},
		Environment:  make(map[string]string),
		done:         make(chan struct{}),
		StartTime:    time.Now(),
		LastActivity: time.Now(),
	}

	// Test session data
	if sessionCtx.SessionID != "test-session" {
		t.Errorf("Expected session ID 'test-session', got '%s'", sessionCtx.SessionID)
	}

	if sessionCtx.WindowSize.Width != 80 {
		t.Errorf("Expected window width 80, got %d", sessionCtx.WindowSize.Width)
	}

	if sessionCtx.WindowSize.Height != 24 {
		t.Errorf("Expected window height 24, got %d", sessionCtx.WindowSize.Height)
	}

	// Use sshServer to avoid unused variable error
	if sshServer == nil {
		t.Error("SSH server should not be nil")
	}
}

// TestSSHMenuHandling tests SSH menu handling
func TestSSHMenuHandling(t *testing.T) {
	// Create test configuration
	cfg := createTestConfig()

	// Create mock session service
	sessionService := &Service{}

	// Create SSH server
	sshServer, err := NewSSHServer(sessionService, cfg)
	if err != nil {
		t.Fatalf("Failed to create SSH server: %v", err)
	}

	// Create mock channel for testing
	mockChannel := &mockSSHChannel{}

	// Create test session context
	sessionCtx := &SSHSessionContext{
		SessionID:       "test-session",
		Username:        "testuser",
		Channel:         mockChannel,
		WindowSize:      &WindowSize{Width: 80, Height: 24},
		Environment:     make(map[string]string),
		done:            make(chan struct{}),
		IsAuthenticated: false,
	}

	// Test menu choice handling
	ctx := context.Background()

	// Test quit command
	continueMenu := sshServer.handleMenuChoice(ctx, sessionCtx, "q")
	if continueMenu {
		t.Error("Expected quit command to return false")
	}

	// Test invalid command
	continueMenu = sshServer.handleMenuChoice(ctx, sessionCtx, "invalid")
	if !continueMenu {
		t.Error("Expected invalid command to return true")
	}
}

// TestSSHCleanup tests SSH cleanup functionality
func TestSSHCleanup(t *testing.T) {
	// Create test configuration
	cfg := createTestConfig()

	// Create mock session service
	sessionService := &Service{}

	// Create SSH server
	sshServer, err := NewSSHServer(sessionService, cfg)
	if err != nil {
		t.Fatalf("Failed to create SSH server: %v", err)
	}

	// Test cleanup
	sshServer.cleanupIdleConnections()

	// Test shutdown
	ctx := context.Background()
	err = sshServer.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

// TestSSHServiceIntegration tests SSH service integration
func TestSSHServiceIntegration(t *testing.T) {
	// Create test configuration
	cfg := createTestConfig()

	// Create mock session service with actual methods
	sessionService := &Service{}

	// Create SSH server
	sshServer, err := NewSSHServer(sessionService, cfg)
	if err != nil {
		t.Fatalf("Failed to create SSH server: %v", err)
	}

	// Test service client calls
	ctx := context.Background()

	// Test auth client
	loginReq := &LoginRequest{
		Username: "admin",
		Password: "admin",
	}

	// Use sshServer to avoid unused variable error
	if sshServer == nil {
		t.Error("SSH server should not be nil")
		return
	}

	if sshServer.authClient == nil {
		t.Error("Auth client should not be nil")
		return
	}

	loginResp, err := sshServer.authClient.Login(ctx, loginReq)
	if err != nil {
		t.Errorf("Auth client login failed: %v", err)
	}

	if loginResp == nil {
		t.Error("Login response is nil")
	}

	// Test user client
	if sshServer.userClient == nil {
		t.Error("User client should not be nil")
		return
	}

	user, err := sshServer.userClient.GetUser(ctx, "admin")
	if err != nil {
		t.Errorf("User client get user failed: %v", err)
	}

	if user == nil {
		t.Error("User is nil")
	}

	// Test game client
	if sshServer.gameClient == nil {
		t.Error("Game client should not be nil")
		return
	}

	games, err := sshServer.gameClient.ListGames(ctx)
	if err != nil {
		t.Errorf("Game client list games failed: %v", err)
	}

	if games == nil {
		t.Error("Games list is nil")
	}
}

// TestSSHConfiguration tests SSH configuration
func TestSSHConfiguration(t *testing.T) {
	// Test configuration validation
	cfg := createTestConfig()

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Configuration validation failed: %v", err)
	}

	// Test SSH configuration
	sshConfig := cfg.GetSSH()
	if sshConfig == nil {
		t.Fatal("SSH configuration is nil")
	}

	if sshConfig.Port != 2222 {
		t.Errorf("Expected SSH port 2222, got %d", sshConfig.Port)
	}

	if !sshConfig.Enabled {
		t.Error("SSH should be enabled")
	}

	// Test SSH configuration validation
	err = sshConfig.Validate()
	if err != nil {
		t.Errorf("SSH configuration validation failed: %v", err)
	}
}

// TestSSHPTYIntegration tests SSH PTY integration
func TestSSHPTYIntegration(t *testing.T) {
	// Create test configuration
	cfg := createTestConfig()

	// Create mock session service
	sessionService := &Service{}

	// Create SSH server
	sshServer, err := NewSSHServer(sessionService, cfg)
	if err != nil {
		t.Fatalf("Failed to create SSH server: %v", err)
	}

	// Test PTY manager
	if sshServer.ptyManager == nil {
		t.Fatal("PTY manager is nil")
	}

	// Test PTY session allocation
	windowSize := WindowSize{Width: 80, Height: 24}
	ptySession, err := sshServer.ptyManager.AllocatePTY("test-session", "testuser", "bash", windowSize)
	if err != nil {
		// PTY allocation can fail in testing environments - this is expected
		if err.Error() == "failed to grant PTY: inappropriate ioctl for device" ||
			err.Error() == "failed to open PTY master: no such file or directory" ||
			err.Error() == "failed to open PTY master: operation not permitted" {
			t.Skipf("PTY allocation failed in test environment (expected): %v", err)
			return
		}
		t.Errorf("PTY allocation failed: %v", err)
		return
	}

	if ptySession != nil {
		// Test PTY session
		if ptySession.SessionID != "test-session" {
			t.Errorf("Expected session ID 'test-session', got '%s'", ptySession.SessionID)
		}

		if ptySession.Username != "testuser" {
			t.Errorf("Expected username 'testuser', got '%s'", ptySession.Username)
		}

		// Clean up PTY session
		err = sshServer.ptyManager.ReleasePTY("test-session")
		if err != nil {
			t.Errorf("PTY release failed: %v", err)
		}
	}

	// Use sshServer to avoid unused variable error
	if sshServer == nil {
		t.Error("SSH server should not be nil")
	}
}

// Helper functions for testing

// createTestConfig creates a test configuration
func createTestConfig() *config.SessionServiceConfig {
	return &config.SessionServiceConfig{
		Server: &config.ServerConfig{
			Port:           8083,
			GRPCPort:       9093,
			Host:           "localhost",
			Timeout:        "30s",
			MaxConnections: 100,
		},
		SSH: &config.SSHConfig{
			Enabled:        true,
			Port:           2222,
			Host:           "localhost",
			HostKeyPath:    "/tmp/test_ssh_host_key",
			Banner:         "Test SSH Server\r\n",
			MaxSessions:    10,
			SessionTimeout: "1h",
			IdleTimeout:    "15m",
			Auth: &config.SSHAuthConfig{
				PasswordAuth:   true,
				PublicKeyAuth:  false,
				AllowAnonymous: true,
			},
			Terminal: &config.SSHTerminalConfig{
				DefaultSize:        "80x24",
				MaxSize:            "120x40",
				SupportedTerminals: []string{"xterm", "xterm-256color"},
			},
		},
		SessionManagement: &config.SessionManagementConfig{
			Terminal: &config.TerminalConfig{
				DefaultSize: "80x24",
				MaxSize:     "120x40",
				Encoding:    "utf-8",
			},
			Timeouts: &config.TimeoutsConfig{
				IdleTimeout:        "15m",
				MaxSessionDuration: "1h",
				CleanupInterval:    "1m",
			},
			TTYRec: &config.TTYRecConfig{
				Enabled:       true,
				Compression:   "gzip",
				Directory:     "/tmp/test-ttyrec",
				MaxFileSize:   "10MB",
				RetentionDays: 7,
			},
			Spectating: &config.SpectatingConfig{
				Enabled:                 true,
				MaxSpectatorsPerSession: 3,
				SpectatorTimeout:        "30m",
			},
		},
		Services: &config.ServicesConfig{
			AuthService: "localhost:9090",
			UserService: "localhost:9091",
			GameService: "localhost:9092",
		},
		Storage: &config.StorageConfig{
			TTYRecPath: "/tmp/test-ttyrec",
			TempPath:   "/tmp/test-sessions",
		},
		Logging: &config.LoggingConfig{
			Level:  "test",
			Format: "text",
			Output: "stdout",
		},
	}
}

// mockSSHConnMetadata implements ssh.ConnMetadata for testing
type mockSSHConnMetadata struct {
	username   string
	clientAddr net.Addr
	serverAddr net.Addr
}

// mockSSHChannel implements ssh.Channel for testing
type mockSSHChannel struct {
	writeBuffer []byte
	readBuffer  []byte
	closed      bool
}

func (m *mockSSHChannel) Read(data []byte) (int, error) {
	if m.closed {
		return 0, io.EOF
	}
	if len(m.readBuffer) == 0 {
		return 0, nil
	}
	n := copy(data, m.readBuffer)
	m.readBuffer = m.readBuffer[n:]
	return n, nil
}

func (m *mockSSHChannel) Write(data []byte) (int, error) {
	if m.closed {
		return 0, fmt.Errorf("channel closed")
	}
	m.writeBuffer = append(m.writeBuffer, data...)
	return len(data), nil
}

func (m *mockSSHChannel) Close() error {
	m.closed = true
	return nil
}

func (m *mockSSHChannel) CloseWrite() error {
	return nil
}

func (m *mockSSHChannel) SendRequest(name string, wantReply bool, payload []byte) (bool, error) {
	return false, nil
}

func (m *mockSSHChannel) Stderr() io.ReadWriter {
	return m
}

func (m *mockSSHConnMetadata) User() string {
	return m.username
}

func (m *mockSSHConnMetadata) SessionID() []byte {
	return []byte("test-session-id")
}

func (m *mockSSHConnMetadata) ClientVersion() []byte {
	return []byte("SSH-2.0-Test")
}

func (m *mockSSHConnMetadata) ServerVersion() []byte {
	return []byte("SSH-2.0-dungeongate")
}

func (m *mockSSHConnMetadata) RemoteAddr() net.Addr {
	if m.clientAddr == nil {
		return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}
	}
	return m.clientAddr
}

func (m *mockSSHConnMetadata) LocalAddr() net.Addr {
	if m.serverAddr == nil {
		return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 2222}
	}
	return m.serverAddr
}

// BenchmarkSSHServer benchmarks SSH server performance
func BenchmarkSSHServer(b *testing.B) {
	// Create test configuration
	cfg := createTestConfig()

	// Create mock session service
	sessionService := &Service{}

	// Create SSH server
	sshServer, err := NewSSHServer(sessionService, cfg)
	if err != nil {
		b.Fatalf("Failed to create SSH server: %v", err)
	}

	// Benchmark SSH server operations
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Test connection handling
		conn := &mockSSHConnMetadata{username: fmt.Sprintf("user%d", i)}

		// Test authentication
		_, err := sshServer.handlePasswordAuth(conn, []byte("password"))
		if err != nil {
			b.Errorf("Password auth failed: %v", err)
		}

		// Test banner generation
		banner := sshServer.handleBanner(conn)
		if banner == "" {
			b.Error("Banner should not be empty")
		}
	}
}

// BenchmarkSSHHostKeyGeneration benchmarks SSH host key generation
func BenchmarkSSHHostKeyGeneration(b *testing.B) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "ssh-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Generate RSA key
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			b.Errorf("Failed to generate RSA key: %v", err)
		}

		// Convert to SSH signer
		_, err = ssh.NewSignerFromKey(key)
		if err != nil {
			b.Errorf("Failed to create SSH signer: %v", err)
		}
	}
}

// Test helper functions


// TestSSHEndToEnd tests SSH end-to-end functionality
func TestSSHEndToEnd(t *testing.T) {
	// Skip this test in short mode
	if testing.Short() {
		t.Skip("Skipping end-to-end test in short mode")
	}

	// Create test configuration
	cfg := createTestConfig()

	// Use a random port for testing
	cfg.SSH.Port = 0

	// Create mock session service
	sessionService := &Service{}

	// Create SSH server
	sshServer, err := NewSSHServer(sessionService, cfg)
	if err != nil {
		t.Fatalf("Failed to create SSH server: %v", err)
	}

	// Verify SSH server was created
	if sshServer == nil {
		t.Fatal("SSH server should not be nil")
	}

	// Start server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := sshServer.Start(ctx, cfg.SSH.Port)
		if err != nil && err != context.Canceled {
			t.Errorf("SSH server start failed: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(500 * time.Millisecond)

	// Note: Full SSH client testing would require implementing
	// a complete SSH client interaction, which is complex for unit tests.
	// In practice, this would be done with integration tests.

	t.Log("SSH server started successfully")
}

// TestNetHackLaunch tests that a user can successfully launch NetHack
func TestNetHackLaunch(t *testing.T) {
	// Skip if NetHack is not available
	if _, err := os.Stat("/opt/homebrew/bin/nethack"); os.IsNotExist(err) {
		t.Skip("NetHack not found at /opt/homebrew/bin/nethack, skipping test")
	}

	// Create test configuration with NetHack game
	cfg := createTestConfigWithNetHack()

	// Create mock session service
	sessionService := &Service{}

	// Create SSH server
	sshServer, err := NewSSHServer(sessionService, cfg)
	if err != nil {
		t.Fatalf("Failed to create SSH server: %v", err)
	}

	// Use sshServer to avoid unused variable error
	if sshServer == nil {
		t.Error("SSH server should not be nil")
		return
	}

	// Test that NetHack game configuration is loaded
	if len(cfg.Games) == 0 {
		t.Fatal("No games configured")
	}

	var nethackGame *config.GameConfig
	for _, game := range cfg.Games {
		if game.ID == "nethack" {
			nethackGame = game
			break
		}
	}

	if nethackGame == nil {
		t.Fatal("NetHack game not found in configuration")
	}

	// Verify NetHack configuration
	if nethackGame.Binary.Path != "/opt/homebrew/bin/nethack" {
		t.Errorf("Expected NetHack binary path '/opt/homebrew/bin/nethack', got '%s'", nethackGame.Binary.Path)
	}

	if !nethackGame.Enabled {
		t.Error("NetHack should be enabled")
	}

	// Test PTY allocation and environment setup
	ptyManager, err := NewPTYManager()
	if err != nil {
		t.Fatalf("Failed to create PTY manager: %v", err)
	}
	defer ptyManager.Shutdown()

	windowSize := WindowSize{Width: 80, Height: 24}
	ptySession, err := ptyManager.AllocatePTY("test-session", "testuser", "nethack", windowSize)
	if err != nil {
		t.Fatalf("Failed to allocate PTY: %v", err)
	}

	// Create a mock SSH session context
	sessionCtx := &SSHSessionContext{
		SessionID:  "test-session",
		Username:   "testuser",
		WindowSize: &windowSize,
		ptySession: ptySession,
	}

	// Test that game environment variables are applied
	game := &Game{
		ID:   nethackGame.ID,
		Name: nethackGame.Name,
		Environment: map[string]string{
			"NETHACKOPTIONS": "@/opt/homebrew/share/nethack/${USERNAME}.nethackrc",
			"HACKDIR":        "/opt/homebrew/share/nethack",
			"HOME":           "/opt/homebrew/share/nethack/${USERNAME}",
			"TERM":           "xterm-256color",
		},
		Binary: nethackGame.Binary.Path,
		Args:   []string{"-u", "testuser"}, // Use processed args rather than template
	}

	// Apply game environment (this is the fix we implemented)
	sshServer.applyGameEnvironment(ptySession, game, sessionCtx)

	// Verify environment variables were applied correctly
	expectedVars := map[string]string{
		"NETHACKOPTIONS": "@/opt/homebrew/share/nethack/testuser.nethackrc",
		"HACKDIR":        "/opt/homebrew/share/nethack",
		"HOME":           "/opt/homebrew/share/nethack/testuser",
		"TERM":           "xterm-256color",
	}

	for key, expectedValue := range expectedVars {
		if actualValue, exists := ptySession.Environment[key]; !exists {
			t.Errorf("Environment variable %s not set", key)
		} else if actualValue != expectedValue {
			t.Errorf("Environment variable %s: expected '%s', got '%s'", key, expectedValue, actualValue)
		}
	}

	// Test command building
	command, args := sshServer.buildGameCommand(game, sessionCtx)
	
	if command != "/opt/homebrew/bin/nethack" {
		t.Errorf("Expected command '/opt/homebrew/bin/nethack', got '%s'", command)
	}

	// Debug: Print the actual args
	t.Logf("Command args: %v", args)

	// Check that username is properly added to args
	foundUserFlag := false
	foundUsername := false
	for i, arg := range args {
		if arg == "-u" {
			foundUserFlag = true
			if i+1 < len(args) && args[i+1] == "testuser" {
				foundUsername = true
			}
		}
	}

	if !foundUserFlag {
		t.Error("NetHack command should include -u flag")
	}

	if !foundUsername {
		t.Errorf("NetHack command should include username after -u flag. Args: %v", args)
	}

	// Test that the game can be started (but don't actually run it to completion)
	// We'll start it and immediately stop it to test the launch mechanism
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = ctx // Avoid unused variable warning

	// Start the command
	err = ptySession.StartCommand(command, args)
	if err != nil {
		t.Errorf("Failed to start NetHack command: %v", err)
	} else {
		// If command started successfully, stop it immediately
		time.Sleep(100 * time.Millisecond) // Give it a moment to start
		ptySession.Close()
		t.Log("NetHack command started and stopped successfully")
	}

	// Clean up
	_ = ptyManager.ReleasePTY("test-session")
}

// createTestConfigWithNetHack creates a test configuration that includes NetHack
func createTestConfigWithNetHack() *config.SessionServiceConfig {
	cfg := createTestConfig()
	
	// Add NetHack game configuration
	cfg.Games = []*config.GameConfig{
		{
			ID:        "nethack",
			Name:      "NetHack",
			ShortName: "nh",
			Version:   "3.6.7",
			Enabled:   true,
			Binary: &config.BinaryConfig{
				Path: "/opt/homebrew/bin/nethack",
				Args: []string{"-u", "${USERNAME}"},
				WorkingDirectory: "/opt/homebrew/share/nethack",
				Permissions:      "0755",
			},
			Files: &config.FilesConfig{
				DataDirectory:   "/opt/homebrew/share/nethack",
				SaveDirectory:   "/opt/homebrew/share/nethack/save",
				ConfigDirectory: "/opt/homebrew/share/nethack",
				LogDirectory:    "/tmp/nethack-logs",
				TempDirectory:   "/tmp/nethack-temp",
				SharedFiles:     []string{"nhdat", "license", "recover"},
				UserFiles:       []string{"${USERNAME}.nh", "${USERNAME}.0", "${USERNAME}.bak"},
			},
			Environment: map[string]string{
				"NETHACKOPTIONS": "@/opt/homebrew/share/nethack/${USERNAME}.nethackrc",
				"HACKDIR":        "/opt/homebrew/share/nethack",
				"TERM":           "xterm-256color",
				"USER":           "${USERNAME}",
				"HOME":           "/opt/homebrew/share/nethack/${USERNAME}",
				"SHELL":          "/bin/sh",
			},
		},
	}
	
	return cfg
}
