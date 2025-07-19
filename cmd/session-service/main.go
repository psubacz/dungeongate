package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dungeongate/internal/session"
	"github.com/dungeongate/pkg/config"
	"github.com/dungeongate/pkg/logging"
	"github.com/dungeongate/pkg/metrics"
)

var (
	version   string = "dev"
	buildTime string = "unknown"
	gitCommit string = "unknown"
)

// setupLogger creates a configured slog logger
func setupLogger(cfg *config.SessionServiceConfig) *slog.Logger {
	// Set defaults if values are empty
	level := "info"
	format := "text"
	output := "stdout"

	if cfg.Logging != nil {
		if cfg.Logging.Level != "" {
			level = cfg.Logging.Level
		}
		if cfg.Logging.Format != "" {
			format = cfg.Logging.Format
		}
		if cfg.Logging.Output != "" {
			output = cfg.Logging.Output
		}
	}

	return logging.NewLoggerBasic("session-service", level, format, output)
}

func main() {
	var (
		configFile  = flag.String("config", "configs/session-service.yaml", "Path to configuration file")
		showVersion = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("DungeonGate Session Service (Stateless)\n")
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Build Time: %s\n", buildTime)
		fmt.Printf("Git Commit: %s\n", gitCommit)
		return
	}

	// Load configuration first to setup logging properly
	cfg, err := config.LoadSessionServiceConfig(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Setup structured logging based on configuration
	logger := setupLogger(cfg)

	// Initialize metrics registry
	metricsRegistry := metrics.NewRegistry("session-service", version, buildTime, gitCommit, logger)

	// Start metrics server if enabled
	if cfg.Metrics != nil && cfg.Metrics.Enabled {
		go func() {
			if err := metricsRegistry.StartMetricsServer(cfg.Metrics.Port); err != nil {
				logger.Error("Failed to start metrics server", "error", err)
			}
		}()
		logger.Info("Metrics server starting", "port", cfg.Metrics.Port)
	}

	// Convert to session config format
	sessionConfig := &session.Config{
		GameService: struct {
			Address string `yaml:"address" default:"localhost:50051"`
		}{
			Address: cfg.Services.GameService,
		},
		AuthService: struct {
			Address string `yaml:"address" default:"localhost:8082"`
		}{
			Address: cfg.Services.AuthService,
		},
		SSH: struct {
			Address         string `yaml:"address" default:"0.0.0.0"`
			Port            int    `yaml:"port" default:"2222"`
			IdleTimeout     string `yaml:"idle_timeout" default:"1h"`
			HostKey         string `yaml:"host_key" default:""`
			PasswordAuth    bool   `yaml:"password_auth" default:"true"`
			PublicKeyAuth   bool   `yaml:"public_key_auth" default:"false"`
			AllowAnonymous  bool   `yaml:"allow_anonymous" default:"true"`
			AllowedUsername string `yaml:"allowed_username" default:"dungeongate"`
			SSHPassword     string `yaml:"ssh_password" default:""`
		}{
			Address:         cfg.SSH.Host,
			Port:            cfg.SSH.Port,
			IdleTimeout:     "1h",
			HostKey:         cfg.SSH.HostKeyPath,
			PasswordAuth:    cfg.SSH.Auth.PasswordAuth,
			PublicKeyAuth:   cfg.SSH.Auth.PublicKeyAuth,
			AllowAnonymous:  cfg.SSH.Auth.AllowAnonymous,
			AllowedUsername: cfg.SSH.Auth.AllowedUsername,
			SSHPassword:     cfg.SSH.Auth.SSHPassword,
		},
		HTTP: struct {
			Address string `yaml:"address" default:"0.0.0.0"`
			Port    int    `yaml:"port" default:"8083"`
		}{
			Address: "0.0.0.0",
			Port:    cfg.Server.Port,
		},
		GRPC: struct {
			Address string `yaml:"address" default:"0.0.0.0"`
			Port    int    `yaml:"port" default:"9093"`
		}{
			Address: "0.0.0.0",
			Port:    cfg.Server.Port + 1000, // Use different port for gRPC
		},
		MaxConnections: 1000,
		MaxPTYs:        500,
		Version:        version,
	}

	// Set idle retry interval if available
	if cfg.SessionManagement != nil && cfg.SessionManagement.Heartbeat != nil {
		if interval, err := time.ParseDuration(cfg.SessionManagement.Heartbeat.IdleRetryInterval); err == nil {
			sessionConfig.IdleRetryInterval = interval
		} else {
			sessionConfig.IdleRetryInterval = 5 * time.Second
		}
	} else {
		sessionConfig.IdleRetryInterval = 5 * time.Second
	}

	// Set banner configuration if available
	if cfg.Menu != nil && cfg.Menu.Banners != nil {
		sessionConfig.Menu.Banners.MainAnon = cfg.Menu.Banners.MainAnon
		sessionConfig.Menu.Banners.MainUser = cfg.Menu.Banners.MainUser
		sessionConfig.Menu.Banners.MainAdmin = cfg.Menu.Banners.MainAdmin
		sessionConfig.Menu.Banners.WatchMenu = cfg.Menu.Banners.WatchMenu
		sessionConfig.Menu.Banners.ServiceUnavailable = cfg.Menu.Banners.ServiceUnavailable
	}

	// Create stateless session service
	sessionService, err := session.New(sessionConfig, logger, metricsRegistry)
	if err != nil {
		logger.Error("Failed to create session service", "error", err)
		os.Exit(1)
	}

	// Start the service
	if err := sessionService.Start(); err != nil {
		logger.Error("Failed to start session service", "error", err)
		os.Exit(1)
	}

	logger.Info("Session Service started",
		"ssh_port", sessionConfig.SSH.Port,
		"http_port", sessionConfig.HTTP.Port,
		"grpc_port", sessionConfig.GRPC.Port,
		"max_connections", sessionConfig.MaxConnections,
	)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	logger.Info("Shutting down gracefully...")

	// Shutdown the service
	if err := sessionService.Stop(); err != nil {
		logger.Error("Error during shutdown", "error", err)
	}

	// Stop metrics server
	if cfg.Metrics != nil && cfg.Metrics.Enabled {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := metricsRegistry.StopMetricsServer(shutdownCtx); err != nil {
			logger.Error("Error stopping metrics server", "error", err)
		}
	}

	logger.Info("Session Service stopped")
}
