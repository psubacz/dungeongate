package handlers

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"

	"golang.org/x/crypto/ssh"
)

// PoolBasedSSHServer implements SSH server with pool-based architecture
type PoolBasedSSHServer struct {
	config  SSHServerConfig
	handler *SessionHandler
	logger  *slog.Logger

	listener  net.Listener
	sshConfig *ssh.ServerConfig

	stopChan chan struct{}
	wg       sync.WaitGroup

	// Connection tracking
	activeConnections map[string]net.Conn
	connMutex         sync.RWMutex
}

// NewPoolBasedSSHServer creates a new pool-based SSH server
func NewPoolBasedSSHServer(config SSHServerConfig, handler *SessionHandler, logger *slog.Logger) *PoolBasedSSHServer {
	return &PoolBasedSSHServer{
		config:            config,
		handler:           handler,
		logger:            logger,
		stopChan:          make(chan struct{}),
		activeConnections: make(map[string]net.Conn),
	}
}

// Name returns the server name
func (s *PoolBasedSSHServer) Name() string {
	return "ssh"
}

// Start starts the pool-based SSH server
func (s *PoolBasedSSHServer) Start(ctx context.Context) error {
	// Create SSH server configuration
	if err := s.createSSHConfig(); err != nil {
		return fmt.Errorf("failed to create SSH config: %w", err)
	}

	// Create listener
	address := fmt.Sprintf("%s:%d", s.config.Address, s.config.Port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", address, err)
	}
	s.listener = listener

	s.logger.Info("Pool-based SSH server starting", "address", address)

	// Start accepting connections
	s.wg.Add(1)
	go s.acceptConnections(ctx)

	return nil
}

// Stop stops the pool-based SSH server
func (s *PoolBasedSSHServer) Stop(ctx context.Context) error {
	s.logger.Info("Stopping pool-based SSH server")

	// Signal stop
	close(s.stopChan)

	// Close listener
	if s.listener != nil {
		s.listener.Close()
	}

	// Close all active connections
	s.connMutex.Lock()
	for id, conn := range s.activeConnections {
		s.logger.Debug("Closing SSH connection", "connection_id", id)
		conn.Close()
	}
	s.connMutex.Unlock()

	// Wait for accept loop to finish
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("Pool-based SSH server stopped")
	case <-ctx.Done():
		s.logger.Warn("Timeout stopping pool-based SSH server")
	}

	return nil
}

// acceptConnections handles incoming SSH connections
func (s *PoolBasedSSHServer) acceptConnections(ctx context.Context) {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ctx.Done():
			return
		default:
			// Accept connection with timeout
			conn, err := s.listener.Accept()
			if err != nil {
				select {
				case <-s.stopChan:
					return // Server is stopping
				default:
					s.logger.Error("Failed to accept SSH connection", "error", err)
					continue
				}
			}

			// Track connection
			connID := fmt.Sprintf("ssh-%p", conn)
			s.connMutex.Lock()
			s.activeConnections[connID] = conn
			s.connMutex.Unlock()

			// Handle connection in pool
			s.wg.Add(1)
			go func(c net.Conn, id string) {
				defer s.wg.Done()
				defer func() {
					// Clean up connection tracking
					s.connMutex.Lock()
					delete(s.activeConnections, id)
					s.connMutex.Unlock()
					c.Close()
				}()

				// Use pool-based connection handler
				if err := s.handler.HandleNewConnection(ctx, c, s.sshConfig); err != nil {
					s.logger.Error("Pool-based SSH connection handling failed",
						"connection_id", id, "error", err)
				}
			}(conn, connID)
		}
	}
}

// createSSHConfig creates the SSH server configuration
func (s *PoolBasedSSHServer) createSSHConfig() error {
	config := &ssh.ServerConfig{
		NoClientAuth: true, // For now, allow anonymous access
		// Note: Banner is set via config.BannerCallback, not directly
	}

	// Set banner if provided
	if s.config.Banner != "" {
		config.BannerCallback = func(conn ssh.ConnMetadata) string {
			return s.config.Banner
		}
	}

	// Load or generate host key
	hostKey, err := s.loadOrGenerateHostKey(s.config.HostKeyPath)
	if err != nil {
		return fmt.Errorf("failed to load host key: %w", err)
	}
	config.AddHostKey(hostKey)

	s.sshConfig = config
	return nil
}

// generateHostKey generates a new RSA host key
func (s *PoolBasedSSHServer) generateHostKey() (ssh.Signer, error) {
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
func (s *PoolBasedSSHServer) loadOrGenerateHostKey(hostKeyPath string) (ssh.Signer, error) {
	// If no path is specified, generate a new key
	if hostKeyPath == "" {
		s.logger.Info("No host key path specified, generating temporary key")
		return s.generateHostKey()
	}

	// Try to load existing key
	if _, err := os.Stat(hostKeyPath); err == nil {
		// Key file exists, try to load it
		keyBytes, err := os.ReadFile(hostKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read host key file: %w", err)
		}

		// Parse the private key
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse host key: %w", err)
		}

		s.logger.Info("Loaded host key from file", "path", hostKeyPath)
		return signer, nil
	}

	// Key file doesn't exist, generate a new key
	s.logger.Info("Host key file not found, generating temporary key", "path", hostKeyPath)
	return s.generateHostKey()
}
