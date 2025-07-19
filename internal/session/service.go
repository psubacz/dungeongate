package session

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dungeongate/internal/session/client"
	"github.com/dungeongate/internal/session/connection"
	"github.com/dungeongate/internal/session/server"
	"github.com/dungeongate/internal/session/streaming"
	"github.com/dungeongate/pkg/metrics"
)

// Service represents the stateless Session Service
type Service struct {
	config *Config
	logger *slog.Logger

	// Metrics
	metricsRegistry *metrics.Registry

	// Clients for external services
	gameClient *client.GameClient
	authClient *client.AuthClient

	// Core components
	connectionManager *connection.Manager
	streamingManager  *streaming.Manager

	// Servers
	sshServer  *server.SSHServer
	httpServer *server.HTTPServer
	grpcServer *server.GRPCServer

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a new stateless Session Service instance
func New(cfg *Config, logger *slog.Logger, metricsRegistry *metrics.Registry) (*Service, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize service clients
	gameClient, err := client.NewGameClient(cfg.GameService.Address, logger)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create game client: %w", err)
	}

	authClient, err := client.NewAuthClient(cfg.AuthService.Address, logger)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create auth client: %w", err)
	}

	// Initialize core components
	connectionManager := connection.NewManager(cfg.MaxConnections, logger)
	streamingManager := streaming.NewManager(logger, gameClient)

	// Initialize servers
	sshConfig := &server.SSHConfig{
		Address:                  cfg.SSH.Address,
		Port:                     cfg.SSH.Port,
		MaxConns:                 cfg.MaxConnections,
		IdleTimeout:              cfg.SSH.IdleTimeout,
		HostKey:                  cfg.SSH.HostKey,
		PasswordAuth:             cfg.SSH.PasswordAuth,
		PublicKeyAuth:            cfg.SSH.PublicKeyAuth,
		AllowAnonymous:           cfg.SSH.AllowAnonymous,
		AllowedUsername:          cfg.SSH.AllowedUsername,
		SSHPassword:              cfg.SSH.SSHPassword,
		BannerMainAnon:           cfg.Menu.Banners.MainAnon,
		BannerMainUser:           cfg.Menu.Banners.MainUser,
		BannerMainAdmin:          cfg.Menu.Banners.MainAdmin,
		BannerWatchMenu:          cfg.Menu.Banners.WatchMenu,
		BannerServiceUnavailable: cfg.Menu.Banners.ServiceUnavailable,
		IdleRetryInterval:        cfg.IdleRetryInterval,
		Version:                  cfg.Version,
	}
	sshServer, err := server.NewSSHServer(sshConfig, gameClient, authClient, logger)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create SSH server: %w", err)
	}

	httpConfig := &server.HTTPConfig{
		Address: cfg.HTTP.Address,
		Port:    cfg.HTTP.Port,
	}
	httpServer := server.NewHTTPServer(httpConfig, connectionManager, logger)

	grpcConfig := &server.GRPCConfig{
		Address: cfg.GRPC.Address,
		Port:    cfg.GRPC.Port,
	}
	grpcServer := server.NewGRPCServer(grpcConfig, logger)

	return &Service{
		config:            cfg,
		logger:            logger,
		metricsRegistry:   metricsRegistry,
		gameClient:        gameClient,
		authClient:        authClient,
		connectionManager: connectionManager,
		streamingManager:  streamingManager,
		sshServer:         sshServer,
		httpServer:        httpServer,
		grpcServer:        grpcServer,
		ctx:               ctx,
		cancel:            cancel,
	}, nil
}

// Start starts all service components
func (s *Service) Start() error {
	s.logger.Info("Starting Session Service")

	// Start connection manager
	if err := s.connectionManager.Start(s.ctx); err != nil {
		return fmt.Errorf("failed to start connection manager: %w", err)
	}

	// Start streaming manager
	if err := s.streamingManager.Start(s.ctx); err != nil {
		return fmt.Errorf("failed to start streaming manager: %w", err)
	}

	// Start HTTP server
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.httpServer.Start(s.ctx); err != nil {
			s.logger.Error("HTTP server error", "error", err)
		}
	}()

	// Start gRPC server
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.grpcServer.Start(s.ctx); err != nil {
			s.logger.Error("gRPC server error", "error", err)
		}
	}()

	// Start SSH server
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.sshServer.Start(s.ctx); err != nil {
			s.logger.Error("SSH server error", "error", err)
		}
	}()

	s.logger.Info("Session Service started successfully")
	return nil
}

// Stop gracefully shuts down the service
func (s *Service) Stop() error {
	s.logger.Info("Stopping Session Service")

	// Cancel context to signal shutdown
	s.cancel()

	// Stop servers
	if err := s.sshServer.Stop(s.ctx); err != nil {
		s.logger.Error("Error stopping SSH server", "error", err)
	}
	if err := s.httpServer.Stop(s.ctx); err != nil {
		s.logger.Error("Error stopping HTTP server", "error", err)
	}
	if err := s.grpcServer.Stop(s.ctx); err != nil {
		s.logger.Error("Error stopping gRPC server", "error", err)
	}

	// Stop managers
	if err := s.connectionManager.Stop(s.ctx); err != nil {
		s.logger.Error("Error stopping connection manager", "error", err)
	}
	if err := s.streamingManager.Stop(s.ctx); err != nil {
		s.logger.Error("Error stopping streaming manager", "error", err)
	}

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	// Wait for graceful shutdown or timeout
	select {
	case <-done:
		s.logger.Info("Session Service stopped gracefully")
	case <-time.After(30 * time.Second):
		s.logger.Warn("Session Service shutdown timeout")
	}

	// Close service clients
	if err := s.gameClient.Close(); err != nil {
		s.logger.Error("Error closing game client", "error", err)
	}
	if err := s.authClient.Close(); err != nil {
		s.logger.Error("Error closing auth client", "error", err)
	}

	return nil
}

// Health returns the service health status
func (s *Service) Health() map[string]interface{} {
	return map[string]interface{}{
		"status":      "healthy",
		"connections": s.connectionManager.GetStats(),
		"streaming":   s.streamingManager.GetStats(s.ctx),
	}
}
