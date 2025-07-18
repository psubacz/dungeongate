package server

import (
	"context"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dungeongate/internal/session/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestNewSSHServer(t *testing.T) {
	logger := slog.Default()

	config := &SSHConfig{
		Address:       "localhost",
		Port:          2222,
		HostKey:       "",
		MaxConns:      100,
		IdleTimeout:   "30m",
		PasswordAuth:  true,
		PublicKeyAuth: true,
	}

	// Create clients (may fail if services aren't running)
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

	server, err := NewSSHServer(config, gameClient, authClient, logger)

	assert.NoError(t, err)
	assert.NotNil(t, server)
	assert.Equal(t, config, server.config)
	assert.NotNil(t, server.sshConfig)
	assert.NotNil(t, server.handler)
	assert.NotNil(t, server.connManager)
	assert.Equal(t, logger, server.logger)
	assert.NotNil(t, server.sshConfig.PasswordCallback)
	assert.NotNil(t, server.sshConfig.PublicKeyCallback)
	assert.False(t, server.sshConfig.NoClientAuth)
}

func TestSSHServerStartStop(t *testing.T) {
	logger := slog.Default()

	config := &SSHConfig{
		Address:     "localhost",
		Port:        0, // Use port 0 to get a random available port
		HostKey:     "",
		MaxConns:    100,
		IdleTimeout: "30m",
	}

	// Create minimal clients
	gameClient, _ := client.NewGameClient("localhost:50051", logger)
	authClient, _ := client.NewAuthClient("localhost:8082", logger)

	if gameClient != nil {
		defer gameClient.Close()
	}
	if authClient != nil {
		defer authClient.Close()
	}

	server, err := NewSSHServer(config, gameClient, authClient, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test start
	err = server.Start(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, server.listener)

	// Test stop
	err = server.Stop(ctx)
	assert.NoError(t, err)

	// Test stop on already stopped server
	err = server.Stop(ctx)
	assert.NoError(t, err)
}

func TestSSHServerStartPortInUse(t *testing.T) {
	logger := slog.Default()

	config := &SSHConfig{
		Address:     "localhost",
		Port:        0, // Use port 0 to get a random available port
		HostKey:     "",
		MaxConns:    100,
		IdleTimeout: "30m",
	}

	// Create minimal clients
	gameClient, _ := client.NewGameClient("localhost:50051", logger)
	authClient, _ := client.NewAuthClient("localhost:8082", logger)

	if gameClient != nil {
		defer gameClient.Close()
	}
	if authClient != nil {
		defer authClient.Close()
	}

	server1, err := NewSSHServer(config, gameClient, authClient, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start first server
	err = server1.Start(ctx)
	require.NoError(t, err)
	defer server1.Stop(ctx)

	// Get the actual port used
	actualPort := server1.listener.Addr().(*net.TCPAddr).Port

	// Create second server with same port
	config2 := &SSHConfig{
		Address:     "localhost",
		Port:        actualPort,
		HostKey:     "",
		MaxConns:    100,
		IdleTimeout: "30m",
	}

	server2, err := NewSSHServer(config2, gameClient, authClient, logger)
	require.NoError(t, err)

	// This should fail due to port already in use
	err = server2.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to listen")
}

func TestSSHServerWithInvalidConfig(t *testing.T) {
	logger := slog.Default()

	// Create minimal clients
	gameClient, _ := client.NewGameClient("localhost:50051", logger)
	authClient, _ := client.NewAuthClient("localhost:8082", logger)

	if gameClient != nil {
		defer gameClient.Close()
	}
	if authClient != nil {
		defer authClient.Close()
	}

	// Test with invalid port
	config := &SSHConfig{
		Address:     "localhost",
		Port:        99999, // Invalid port
		HostKey:     "",
		MaxConns:    100,
		IdleTimeout: "30m",
	}

	server, err := NewSSHServer(config, gameClient, authClient, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = server.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to listen")
}

func TestSSHServerConnectionAcceptance(t *testing.T) {
	logger := slog.Default()

	config := &SSHConfig{
		Address:     "localhost",
		Port:        0,
		HostKey:     "",
		MaxConns:    100,
		IdleTimeout: "30m",
	}

	// Create minimal clients
	gameClient, _ := client.NewGameClient("localhost:50051", logger)
	authClient, _ := client.NewAuthClient("localhost:8082", logger)

	if gameClient != nil {
		defer gameClient.Close()
	}
	if authClient != nil {
		defer authClient.Close()
	}

	server, err := NewSSHServer(config, gameClient, authClient, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start server
	err = server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop(ctx)

	// Get the actual address
	addr := server.listener.Addr().String()

	// Try to connect (will fail due to SSH handshake without proper client)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Logf("Failed to connect (expected): %v", err)
		return
	}

	// Close connection immediately
	conn.Close()
}

func TestGenerateHostKey(t *testing.T) {
	hostKey, err := generateHostKey()

	assert.NoError(t, err)
	assert.NotNil(t, hostKey)

	// Test that we can use the key
	publicKey := hostKey.PublicKey()
	assert.NotNil(t, publicKey)

	// Test signing
	data := []byte("test data")
	signature, err := hostKey.Sign(nil, data)
	assert.NoError(t, err)
	assert.NotNil(t, signature)
}

func TestSSHServerContextCancellation(t *testing.T) {
	logger := slog.Default()

	config := &SSHConfig{
		Address:     "localhost",
		Port:        0,
		HostKey:     "",
		MaxConns:    100,
		IdleTimeout: "30m",
	}

	// Create minimal clients
	gameClient, _ := client.NewGameClient("localhost:50051", logger)
	authClient, _ := client.NewAuthClient("localhost:8082", logger)

	if gameClient != nil {
		defer gameClient.Close()
	}
	if authClient != nil {
		defer authClient.Close()
	}

	server, err := NewSSHServer(config, gameClient, authClient, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	// Start server
	err = server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop(ctx)

	// Cancel context
	cancel()

	// Give some time for context cancellation to take effect
	time.Sleep(100 * time.Millisecond)

	// Server should still be running until explicitly stopped
	assert.NotNil(t, server.listener)
}

func TestSSHServerMultipleStartStop(t *testing.T) {
	logger := slog.Default()

	config := &SSHConfig{
		Address:     "localhost",
		Port:        0,
		HostKey:     "",
		MaxConns:    100,
		IdleTimeout: "30m",
	}

	// Create minimal clients
	gameClient, _ := client.NewGameClient("localhost:50051", logger)
	authClient, _ := client.NewAuthClient("localhost:8082", logger)

	if gameClient != nil {
		defer gameClient.Close()
	}
	if authClient != nil {
		defer authClient.Close()
	}

	server, err := NewSSHServer(config, gameClient, authClient, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start and stop multiple times
	for i := 0; i < 3; i++ {
		err = server.Start(ctx)
		assert.NoError(t, err)

		err = server.Stop(ctx)
		assert.NoError(t, err)
	}
}

// Integration test with real SSH client
func TestSSHServerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := slog.Default()

	config := &SSHConfig{
		Address:     "localhost",
		Port:        0,
		HostKey:     "",
		MaxConns:    100,
		IdleTimeout: "30m",
	}

	// Create clients
	gameClient, err := client.NewGameClient("localhost:50051", logger)
	if err != nil {
		t.Skip("Game service not available for integration testing")
	}
	defer gameClient.Close()

	authClient, err := client.NewAuthClient("localhost:8082", logger)
	if err != nil {
		t.Skip("Auth service not available for integration testing")
	}
	defer authClient.Close()

	server, err := NewSSHServer(config, gameClient, authClient, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Start server
	err = server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop(ctx)

	// Get the actual address
	addr := server.listener.Addr().String()

	// Try to create an SSH client connection
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Create SSH client config
	clientConfig := &ssh.ClientConfig{
		User: "testuser",
		Auth: []ssh.AuthMethod{
			ssh.Password("testpass"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	// Try SSH handshake (will likely fail without proper auth service)
	_, _, _, err = ssh.NewClientConn(conn, addr, clientConfig)

	// We expect this to fail in unit tests
	assert.Error(t, err)
	t.Logf("SSH handshake failed as expected: %v", err)
}

// TestSSHConfigurationRespected tests that all SSH configuration options are properly applied
func TestSSHConfigurationRespected(t *testing.T) {
	logger := slog.Default()

	// Create minimal clients
	gameClient, _ := client.NewGameClient("localhost:50051", logger)
	authClient, _ := client.NewAuthClient("localhost:8082", logger)

	if gameClient != nil {
		defer gameClient.Close()
	}
	if authClient != nil {
		defer authClient.Close()
	}

	t.Run("PasswordAuth enabled", func(t *testing.T) {
		config := &SSHConfig{
			Address:        "localhost",
			Port:           0,
			HostKey:        "",
			MaxConns:       100,
			IdleTimeout:    "30m",
			PasswordAuth:   true,
			PublicKeyAuth:  false,
			AllowAnonymous: false,
		}

		server, err := NewSSHServer(config, gameClient, authClient, logger)
		require.NoError(t, err)

		// Test that password callback is set
		assert.NotNil(t, server.sshConfig.PasswordCallback)
		assert.Nil(t, server.sshConfig.PublicKeyCallback)
		assert.False(t, server.sshConfig.NoClientAuth)
	})

	t.Run("PublicKeyAuth enabled", func(t *testing.T) {
		config := &SSHConfig{
			Address:        "localhost",
			Port:           0,
			HostKey:        "",
			MaxConns:       100,
			IdleTimeout:    "30m",
			PasswordAuth:   false,
			PublicKeyAuth:  true,
			AllowAnonymous: false,
		}

		server, err := NewSSHServer(config, gameClient, authClient, logger)
		require.NoError(t, err)

		// Test that public key callback is set
		assert.Nil(t, server.sshConfig.PasswordCallback)
		assert.NotNil(t, server.sshConfig.PublicKeyCallback)
		assert.False(t, server.sshConfig.NoClientAuth)
	})

	t.Run("Both auth methods enabled", func(t *testing.T) {
		config := &SSHConfig{
			Address:        "localhost",
			Port:           0,
			HostKey:        "",
			MaxConns:       100,
			IdleTimeout:    "30m",
			PasswordAuth:   true,
			PublicKeyAuth:  true,
			AllowAnonymous: false,
		}

		server, err := NewSSHServer(config, gameClient, authClient, logger)
		require.NoError(t, err)

		// Test that both callbacks are set
		assert.NotNil(t, server.sshConfig.PasswordCallback)
		assert.NotNil(t, server.sshConfig.PublicKeyCallback)
		assert.False(t, server.sshConfig.NoClientAuth)
	})

	t.Run("Anonymous auth enabled", func(t *testing.T) {
		config := &SSHConfig{
			Address:        "localhost",
			Port:           0,
			HostKey:        "",
			MaxConns:       100,
			IdleTimeout:    "30m",
			PasswordAuth:   false,
			PublicKeyAuth:  false,
			AllowAnonymous: true,
		}

		server, err := NewSSHServer(config, gameClient, authClient, logger)
		require.NoError(t, err)

		// Test that NoClientAuth is set
		assert.Nil(t, server.sshConfig.PasswordCallback)
		assert.Nil(t, server.sshConfig.PublicKeyCallback)
		assert.True(t, server.sshConfig.NoClientAuth)
	})

	t.Run("No auth methods enabled", func(t *testing.T) {
		config := &SSHConfig{
			Address:        "localhost",
			Port:           0,
			HostKey:        "",
			MaxConns:       100,
			IdleTimeout:    "30m",
			PasswordAuth:   false,
			PublicKeyAuth:  false,
			AllowAnonymous: false,
		}

		server, err := NewSSHServer(config, gameClient, authClient, logger)
		require.NoError(t, err)

		// Test that no callbacks are set and NoClientAuth is false
		assert.Nil(t, server.sshConfig.PasswordCallback)
		assert.Nil(t, server.sshConfig.PublicKeyCallback)
		assert.False(t, server.sshConfig.NoClientAuth)
	})
}

// TestHostKeyPathHandling tests that host key path configuration is properly handled
func TestHostKeyPathHandling(t *testing.T) {
	logger := slog.Default()

	// Create minimal clients
	gameClient, _ := client.NewGameClient("localhost:50051", logger)
	authClient, _ := client.NewAuthClient("localhost:8082", logger)

	if gameClient != nil {
		defer gameClient.Close()
	}
	if authClient != nil {
		defer authClient.Close()
	}

	t.Run("Empty host key path generates key", func(t *testing.T) {
		config := &SSHConfig{
			Address:        "localhost",
			Port:           0,
			HostKey:        "", // Empty path
			MaxConns:       100,
			IdleTimeout:    "30m",
			PasswordAuth:   true,
			PublicKeyAuth:  false,
			AllowAnonymous: false,
		}

		server, err := NewSSHServer(config, gameClient, authClient, logger)
		require.NoError(t, err)

		// Should succeed without errors
		assert.NotNil(t, server.sshConfig)
		assert.NotNil(t, server.sshConfig.PasswordCallback) // Should have auth callback based on config
	})

	t.Run("Host key path creates and loads key", func(t *testing.T) {
		// Create temporary directory for test
		tempDir, err := os.MkdirTemp("", "ssh_test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		hostKeyPath := filepath.Join(tempDir, "test_host_key")

		config := &SSHConfig{
			Address:        "localhost",
			Port:           0,
			HostKey:        hostKeyPath,
			MaxConns:       100,
			IdleTimeout:    "30m",
			PasswordAuth:   true,
			PublicKeyAuth:  false,
			AllowAnonymous: false,
		}

		// First call should create the key
		server1, err := NewSSHServer(config, gameClient, authClient, logger)
		require.NoError(t, err)

		// Verify key file was created
		assert.FileExists(t, hostKeyPath)

		// Check file permissions
		info, err := os.Stat(hostKeyPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

		// Get key content
		keyContent1, err := os.ReadFile(hostKeyPath)
		require.NoError(t, err)

		// Second call should load the existing key
		server2, err := NewSSHServer(config, gameClient, authClient, logger)
		require.NoError(t, err)

		// Verify key file content hasn't changed
		keyContent2, err := os.ReadFile(hostKeyPath)
		require.NoError(t, err)
		assert.Equal(t, keyContent1, keyContent2)

		// Both servers should have valid SSH configs
		assert.NotNil(t, server1.sshConfig)
		assert.NotNil(t, server2.sshConfig)
	})

	t.Run("Host key path with non-existent directory", func(t *testing.T) {
		// Create temporary directory for test
		tempDir, err := os.MkdirTemp("", "ssh_test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		// Use a subdirectory that doesn't exist
		hostKeyPath := filepath.Join(tempDir, "nonexistent", "test_host_key")

		config := &SSHConfig{
			Address:        "localhost",
			Port:           0,
			HostKey:        hostKeyPath,
			MaxConns:       100,
			IdleTimeout:    "30m",
			PasswordAuth:   true,
			PublicKeyAuth:  false,
			AllowAnonymous: false,
		}

		// Should create directory and key
		server, err := NewSSHServer(config, gameClient, authClient, logger)
		require.NoError(t, err)

		// Verify key file was created
		assert.FileExists(t, hostKeyPath)

		// Verify directory was created
		assert.DirExists(t, filepath.Dir(hostKeyPath))

		assert.NotNil(t, server.sshConfig)
	})
}

// TestLoadOrGenerateHostKey tests the host key loading function directly
func TestLoadOrGenerateHostKey(t *testing.T) {
	t.Run("Empty path generates key", func(t *testing.T) {
		key, err := loadOrGenerateHostKey("")
		assert.NoError(t, err)
		assert.NotNil(t, key)

		// Test that the key is usable
		publicKey := key.PublicKey()
		assert.NotNil(t, publicKey)
	})

	t.Run("Non-existent file generates and saves key", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "ssh_key_test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		keyPath := filepath.Join(tempDir, "test_key")

		// First call should generate and save
		key1, err := loadOrGenerateHostKey(keyPath)
		assert.NoError(t, err)
		assert.NotNil(t, key1)

		// Verify file was created
		assert.FileExists(t, keyPath)

		// Check file permissions
		info, err := os.Stat(keyPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

		// Second call should load existing
		key2, err := loadOrGenerateHostKey(keyPath)
		assert.NoError(t, err)
		assert.NotNil(t, key2)

		// Keys should be the same
		assert.Equal(t, key1.PublicKey().Marshal(), key2.PublicKey().Marshal())
	})

	t.Run("Invalid key file returns error", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "ssh_key_test")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		keyPath := filepath.Join(tempDir, "invalid_key")

		// Create invalid key file
		err = os.WriteFile(keyPath, []byte("invalid key content"), 0600)
		require.NoError(t, err)

		// Should return error
		_, err = loadOrGenerateHostKey(keyPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse host key")
	})
}

// TestSSHServerConfigurationFields tests that all configuration fields are properly applied
func TestSSHServerConfigurationFields(t *testing.T) {
	logger := slog.Default()

	// Create minimal clients
	gameClient, _ := client.NewGameClient("localhost:50051", logger)
	authClient, _ := client.NewAuthClient("localhost:8082", logger)

	if gameClient != nil {
		defer gameClient.Close()
	}
	if authClient != nil {
		defer authClient.Close()
	}

	t.Run("Configuration values are preserved", func(t *testing.T) {
		config := &SSHConfig{
			Address:        "127.0.0.1",
			Port:           2345,
			HostKey:        "/tmp/test_key",
			MaxConns:       200,
			IdleTimeout:    "45m",
			PasswordAuth:   true,
			PublicKeyAuth:  true,
			AllowAnonymous: false,
		}

		server, err := NewSSHServer(config, gameClient, authClient, logger)
		require.NoError(t, err)

		// Verify configuration is preserved
		assert.Equal(t, config.Address, server.config.Address)
		assert.Equal(t, config.Port, server.config.Port)
		assert.Equal(t, config.HostKey, server.config.HostKey)
		assert.Equal(t, config.MaxConns, server.config.MaxConns)
		assert.Equal(t, config.IdleTimeout, server.config.IdleTimeout)
		assert.Equal(t, config.PasswordAuth, server.config.PasswordAuth)
		assert.Equal(t, config.PublicKeyAuth, server.config.PublicKeyAuth)
		assert.Equal(t, config.AllowAnonymous, server.config.AllowAnonymous)
	})

	t.Run("SSH config reflects auth settings", func(t *testing.T) {
		testCases := []struct {
			name           string
			passwordAuth   bool
			publicKeyAuth  bool
			allowAnonymous bool
			expectedNoAuth bool
		}{
			{"Password only", true, false, false, false},
			{"Public key only", false, true, false, false},
			{"Both methods", true, true, false, false},
			{"Anonymous only", false, false, true, true},
			{"Anonymous with password", true, false, true, true},
			{"Anonymous with public key", false, true, true, true},
			{"Anonymous with both", true, true, true, true},
			{"No auth methods", false, false, false, false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				config := &SSHConfig{
					Address:        "localhost",
					Port:           0,
					HostKey:        "",
					MaxConns:       100,
					IdleTimeout:    "30m",
					PasswordAuth:   tc.passwordAuth,
					PublicKeyAuth:  tc.publicKeyAuth,
					AllowAnonymous: tc.allowAnonymous,
				}

				server, err := NewSSHServer(config, gameClient, authClient, logger)
				require.NoError(t, err)

				// Check NoClientAuth setting
				assert.Equal(t, tc.expectedNoAuth, server.sshConfig.NoClientAuth,
					"NoClientAuth mismatch for %s", tc.name)

				// Check callback presence
				if tc.passwordAuth {
					assert.NotNil(t, server.sshConfig.PasswordCallback,
						"PasswordCallback should be set for %s", tc.name)
				} else {
					assert.Nil(t, server.sshConfig.PasswordCallback,
						"PasswordCallback should be nil for %s", tc.name)
				}

				if tc.publicKeyAuth {
					assert.NotNil(t, server.sshConfig.PublicKeyCallback,
						"PublicKeyCallback should be set for %s", tc.name)
				} else {
					assert.Nil(t, server.sshConfig.PublicKeyCallback,
						"PublicKeyCallback should be nil for %s", tc.name)
				}
			})
		}
	})
}
