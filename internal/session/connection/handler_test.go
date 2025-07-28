package connection

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/dungeongate/internal/session/banner"
	"github.com/dungeongate/internal/session/client"
	"github.com/dungeongate/internal/session/menu"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestNewHandler(t *testing.T) {
	logger := slog.Default()

	// Create manager
	manager := NewManager(100, logger)

	// Note: These will fail if services aren't running, but that's expected in unit tests
	gameClient, err := client.NewGameClient("localhost:50051", logger)
	if err != nil {
		t.Skip("Game service not available for testing")
	}
	defer gameClient.Close()

	authClient, err := client.NewAuthClient("localhost:8082", logger)
	if err != nil {
		t.Skip("Auth service not available for testing")
	}
	defer authClient.Close()

	// Create a mock menu handler for testing
	bannerConfig := &banner.BannerConfig{}
	bannerManager := banner.NewBannerManager(bannerConfig, "test-version")
	menuHandler := menu.NewMenuHandler(bannerManager, gameClient, authClient, logger)

	handler := NewHandler(manager, gameClient, authClient, menuHandler, logger, 5*time.Second)

	assert.NotNil(t, handler)
	assert.Equal(t, manager, handler.manager)
	assert.Equal(t, gameClient, handler.gameClient)
	assert.Equal(t, authClient, handler.authClient)
	assert.Equal(t, logger, handler.logger)
}

func TestHandlerHandleConnectionRegistration(t *testing.T) {
	logger := slog.Default()

	// Create manager
	manager := NewManager(100, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	// Create minimal clients (will fail gracefully if services not available)
	gameClient, _ := client.NewGameClient("localhost:50051", logger)
	authClient, _ := client.NewAuthClient("localhost:8082", logger)

	// Close clients immediately to avoid hanging connections
	if gameClient != nil {
		defer gameClient.Close()
	}
	if authClient != nil {
		defer authClient.Close()
	}

	// Create a mock menu handler for testing
	bannerConfig := &banner.BannerConfig{}
	bannerManager := banner.NewBannerManager(bannerConfig, "test-version")
	menuHandler := menu.NewMenuHandler(bannerManager, gameClient, authClient, logger)

	handler := NewHandler(manager, gameClient, authClient, menuHandler, logger, 5*time.Second)

	// Create mock connection
	conn := &mockNetConn{
		remoteAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345},
	}

	// Create SSH server config
	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			return &ssh.Permissions{}, nil
		},
	}

	// Set up a context with timeout
	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()

	// Test connection handling (will register and unregister)
	handler.HandleConnection(ctx2, conn, config)

	// Verify connection was closed
	assert.True(t, conn.closed)
}

func TestHandlerHandleConnectionMaxConnections(t *testing.T) {
	logger := slog.Default()

	// Create manager with small limit
	manager := NewManager(1, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop(ctx)

	// Create minimal clients
	gameClient, _ := client.NewGameClient("localhost:50051", logger)
	authClient, _ := client.NewAuthClient("localhost:8082", logger)

	if gameClient != nil {
		defer gameClient.Close()
	}
	if authClient != nil {
		defer authClient.Close()
	}

	// Create a mock menu handler for testing
	bannerConfig := &banner.BannerConfig{}
	bannerManager := banner.NewBannerManager(bannerConfig, "test-version")
	menuHandler := menu.NewMenuHandler(bannerManager, gameClient, authClient, logger)

	handler := NewHandler(manager, gameClient, authClient, menuHandler, logger, 5*time.Second)

	// Register first connection
	conn1 := &mockNetConn{
		remoteAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345},
	}

	// This should succeed
	connID1 := manager.RegisterConnection(conn1)
	assert.NotEmpty(t, connID1)

	// Create second connection
	conn2 := &mockNetConn{
		remoteAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12346},
	}

	// Create SSH server config
	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			return &ssh.Permissions{}, nil
		},
	}

	// Set up a context with timeout
	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()

	// This should fail due to max connections
	handler.HandleConnection(ctx2, conn2, config)

	// Verify second connection was closed due to limit
	assert.True(t, conn2.closed)

	// Clean up first connection
	manager.UnregisterConnection(connID1, conn1.RemoteAddr())
}

func TestAuthHandlerPasswordCallback(t *testing.T) {
	logger := slog.Default()

	// Skip if auth service not available
	authClient, err := client.NewAuthClient("localhost:8082", logger)
	if err != nil {
		t.Skip("Auth service not available for testing")
	}
	defer authClient.Close()

	authHandler := NewAuthHandler(authClient, logger)

	// Create mock connection metadata
	connMeta := &mockConnMetadata{
		user: "testuser",
	}

	// Test password callback (will likely fail without running auth service)
	permissions, err := authHandler.PasswordCallback(connMeta, []byte("password"))

	// We expect this to fail in unit tests without running services
	if err != nil {
		assert.Error(t, err)
		assert.Nil(t, permissions)
	} else {
		// If it succeeds (services are running), verify structure
		assert.NotNil(t, permissions)
		assert.NotNil(t, permissions.Extensions)
	}
}

func TestAuthHandlerPublicKeyCallback(t *testing.T) {
	logger := slog.Default()

	authClient, err := client.NewAuthClient("localhost:8082", logger)
	if err != nil {
		t.Skip("Auth service not available for testing")
	}
	defer authClient.Close()

	authHandler := NewAuthHandler(authClient, logger)

	// Create mock connection metadata
	connMeta := &mockConnMetadata{
		user: "testuser",
	}

	// Test public key callback (should always fail as not implemented)
	permissions, err := authHandler.PublicKeyCallback(connMeta, nil)

	assert.Error(t, err)
	assert.Nil(t, permissions)
	assert.Contains(t, err.Error(), "not supported")
}

func TestParsePTYRequest(t *testing.T) {
	logger := slog.Default()
	manager := NewManager(100, logger)
	gameClient, _ := client.NewGameClient("localhost:50051", logger)
	authClient, _ := client.NewAuthClient("localhost:8082", logger)

	if gameClient != nil {
		defer gameClient.Close()
	}
	if authClient != nil {
		defer authClient.Close()
	}

	// Create a mock menu handler for testing
	bannerConfig := &banner.BannerConfig{}
	bannerManager := banner.NewBannerManager(bannerConfig, "test-version")
	menuHandler := menu.NewMenuHandler(bannerManager, gameClient, authClient, logger)

	handler := NewHandler(manager, gameClient, authClient, menuHandler, logger, 5*time.Second)

	tests := []struct {
		name     string
		payload  []byte
		wantCols int
		wantRows int
	}{
		{
			name:     "empty payload",
			payload:  []byte{},
			wantCols: 80,
			wantRows: 24,
		},
		{
			name:     "short payload",
			payload:  []byte{0, 0, 0, 1},
			wantCols: 80,
			wantRows: 24,
		},
		{
			name:     "valid payload",
			payload:  []byte{0, 0, 0, 4, 'x', 't', 'e', 'r', 0, 0, 0, 100, 0, 0, 0, 30}, // term="xter", cols=100, rows=30
			wantCols: 100,
			wantRows: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cols, rows := handler.parsePTYRequest(tt.payload)
			assert.Equal(t, tt.wantCols, cols)
			assert.Equal(t, tt.wantRows, rows)
		})
	}
}

func TestParseWindowChange(t *testing.T) {
	logger := slog.Default()
	manager := NewManager(100, logger)
	gameClient, _ := client.NewGameClient("localhost:50051", logger)
	authClient, _ := client.NewAuthClient("localhost:8082", logger)

	if gameClient != nil {
		defer gameClient.Close()
	}
	if authClient != nil {
		defer authClient.Close()
	}

	// Create a mock menu handler for testing
	bannerConfig := &banner.BannerConfig{}
	bannerManager := banner.NewBannerManager(bannerConfig, "test-version")
	menuHandler := menu.NewMenuHandler(bannerManager, gameClient, authClient, logger)

	handler := NewHandler(manager, gameClient, authClient, menuHandler, logger, 5*time.Second)

	tests := []struct {
		name     string
		payload  []byte
		wantCols int
		wantRows int
	}{
		{
			name:     "empty payload",
			payload:  []byte{},
			wantCols: 80,
			wantRows: 24,
		},
		{
			name:     "short payload",
			payload:  []byte{0, 0, 0, 1},
			wantCols: 80,
			wantRows: 24,
		},
		{
			name:     "valid payload",
			payload:  []byte{0, 0, 0, 120, 0, 0, 0, 40}, // cols=120, rows=40
			wantCols: 120,
			wantRows: 40,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cols, rows := handler.parseWindowChange(tt.payload)
			assert.Equal(t, tt.wantCols, cols)
			assert.Equal(t, tt.wantRows, rows)
		})
	}
}

// Mock connection metadata for testing
type mockConnMetadata struct {
	user string
}

func (m *mockConnMetadata) User() string {
	return m.user
}

func (m *mockConnMetadata) SessionID() []byte {
	return []byte("test-session")
}

func (m *mockConnMetadata) ClientVersion() []byte {
	return []byte("SSH-2.0-Test")
}

func (m *mockConnMetadata) ServerVersion() []byte {
	return []byte("SSH-2.0-DungeonGate")
}

func (m *mockConnMetadata) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
}

func (m *mockConnMetadata) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 2222}
}

// Mock net.Conn for testing
type mockNetConn struct {
	remoteAddr net.Addr
	closed     bool
	reader     io.Reader
	writer     io.Writer
}

func (c *mockNetConn) Read(b []byte) (n int, err error) {
	if c.reader != nil {
		return c.reader.Read(b)
	}
	return 0, io.EOF
}

func (c *mockNetConn) Write(b []byte) (n int, err error) {
	if c.writer != nil {
		return c.writer.Write(b)
	}
	return len(b), nil
}

func (c *mockNetConn) Close() error {
	c.closed = true
	return nil
}

func (c *mockNetConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
}

func (c *mockNetConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *mockNetConn) SetDeadline(t time.Time) error {
	return nil
}

func (c *mockNetConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *mockNetConn) SetWriteDeadline(t time.Time) error {
	return nil
}
