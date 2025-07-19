package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/dungeongate/internal/session/banner"
	"github.com/dungeongate/internal/session/client"
	"github.com/dungeongate/internal/session/connection"
	"github.com/dungeongate/internal/session/menu"
	"golang.org/x/crypto/ssh"
)

// SSHServer provides stateless SSH server functionality
type SSHServer struct {
	config      *SSHConfig
	listener    net.Listener
	sshConfig   *ssh.ServerConfig
	handler     *connection.Handler
	connManager *connection.Manager
	gameClient  *client.GameClient
	authClient  *client.AuthClient
	logger      *slog.Logger
}

// SSHConfig holds SSH server configuration
type SSHConfig struct {
	Address                  string
	Port                     int
	HostKey                  string
	MaxConns                 int
	IdleTimeout              string
	PasswordAuth             bool
	PublicKeyAuth            bool
	AllowAnonymous           bool
	AllowedUsername          string // Only allow connections from this username
	SSHPassword              string // SSH password for the allowed username
	BannerMainAnon           string
	BannerMainUser           string
	BannerMainAdmin          string
	BannerWatchMenu          string
	BannerServiceUnavailable string
	IdleRetryInterval        time.Duration
	Version                  string
}

// NewSSHServer creates a new SSH server
func NewSSHServer(config *SSHConfig, gameClient *client.GameClient, authClient *client.AuthClient, logger *slog.Logger) (*SSHServer, error) {
	// Create connection manager
	connManager := connection.NewManager(config.MaxConns, logger)

	// Create banner manager
	bannerConfig := &banner.BannerConfig{
		MainAnon:           config.BannerMainAnon,
		MainUser:           config.BannerMainUser,
		MainAdmin:          config.BannerMainAdmin,
		WatchMenu:          config.BannerWatchMenu,
		ServiceUnavailable: config.BannerServiceUnavailable,
	}
	bannerManager := banner.NewBannerManager(bannerConfig, config.Version)

	// Create menu handler
	menuHandler := menu.NewMenuHandler(bannerManager, gameClient, authClient, logger)

	// Create auth handler (needed for environment variable handling)
	authHandler := connection.NewSSHAuthHandler(authClient, logger, config.AllowedUsername, config.SSHPassword)

	// Create connection handler
	handler := connection.NewHandler(connManager, gameClient, authClient, menuHandler, logger, config.IdleRetryInterval, authHandler)

	// Create SSH server config
	sshConfig := &ssh.ServerConfig{
		NoClientAuth: config.AllowAnonymous,
	}

	// Set authentication callbacks based on configuration
	if config.PasswordAuth {
		sshConfig.PasswordCallback = authHandler.PasswordCallback
	}
	if config.PublicKeyAuth {
		sshConfig.PublicKeyCallback = authHandler.PublicKeyCallback
	}

	// Load or generate host key
	hostKey, err := loadOrGenerateHostKey(config.HostKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load host key: %w", err)
	}
	sshConfig.AddHostKey(hostKey)

	server := &SSHServer{
		config:      config,
		sshConfig:   sshConfig,
		handler:     handler,
		connManager: connManager,
		gameClient:  gameClient,
		authClient:  authClient,
		logger:      logger,
	}

	return server, nil
}

// Start starts the SSH server
func (s *SSHServer) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.config.Address, s.config.Port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	s.listener = listener

	s.logger.Info("SSH server starting", "address", addr)

	// Start connection manager
	if err := s.connManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start connection manager: %w", err)
	}

	// Start service health monitoring
	go s.monitorServiceHealth(ctx)

	// Accept connections
	go s.acceptConnections(ctx)

	return nil
}

// Stop stops the SSH server
func (s *SSHServer) Stop(ctx context.Context) error {
	if s.listener != nil {
		s.logger.Info("SSH server stopping")
		return s.listener.Close()
	}
	return nil
}

// acceptConnections accepts incoming SSH connections
func (s *SSHServer) acceptConnections(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				s.logger.Error("Failed to accept connection", "error", err)
				continue
			}

			// Handle connection in goroutine
			go s.handler.HandleConnection(ctx, conn, s.sshConfig)
		}
	}
}

// generateHostKey generates a new RSA host key
func generateHostKey() (ssh.Signer, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		return nil, err
	}

	return signer, nil
}

// loadOrGenerateHostKey loads an existing host key or generates a new one
func loadOrGenerateHostKey(hostKeyPath string) (ssh.Signer, error) {
	// If no path is specified, generate a new key
	if hostKeyPath == "" {
		return generateHostKey()
	}

	// Try to load existing key
	if _, err := os.Stat(hostKeyPath); err == nil {
		// Key file exists, try to load it
		keyBytes, err := os.ReadFile(hostKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read host key file: %w", err)
		}

		// Parse the private key (supports both PEM and OpenSSH formats)
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse host key: %w", err)
		}

		return signer, nil
	}

	// Key file doesn't exist, generate a new one and save it
	slog.Info("Generating new host key and saving to file", "path", hostKeyPath)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate host key: %w", err)
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(hostKeyPath), 0700); err != nil {
		return nil, fmt.Errorf("failed to create host key directory: %w", err)
	}

	// Convert to PEM format
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	// Write to file
	if err := os.WriteFile(hostKeyPath, keyPEM, 0600); err != nil {
		return nil, fmt.Errorf("failed to write host key file: %w", err)
	}

	// Create signer from the key
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create signer from key: %w", err)
	}

	return signer, nil
}

// monitorServiceHealth periodically checks service health and logs status
func (s *SSHServer) monitorServiceHealth(ctx context.Context) {
	// Check interval matches idle retry interval for consistency
	ticker := time.NewTicker(s.config.IdleRetryInterval)
	defer ticker.Stop()

	// Initial health check
	s.logServiceHealth(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.logServiceHealth(ctx)
		}
	}
}

// logServiceHealth checks and logs the current health status of all services
func (s *SSHServer) logServiceHealth(ctx context.Context) {
	authHealthy := s.authClient.IsHealthy(ctx)
	gameHealthy := s.gameClient.IsHealthy(ctx)

	if authHealthy && gameHealthy {
		s.logger.Info("Service health check", "auth_service", "available", "game_service", "available", "status", "all_healthy")
	} else {
		authStatus := "available"
		if !authHealthy {
			authStatus = "unavailable"
		}
		gameStatus := "available"
		if !gameHealthy {
			gameStatus = "unavailable"
		}
		s.logger.Warn("Service health check", "auth_service", authStatus, "game_service", gameStatus, "status", "degraded")
	}
}
