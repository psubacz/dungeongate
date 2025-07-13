package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/dungeongate/internal/session"
	"github.com/dungeongate/pkg/config"
	"github.com/dungeongate/pkg/log"
	"github.com/op/go-logging"
)

var (
	version   string = "dev"
	buildTime string = "unknown"
	gitCommit string = "unknown"
)

// setupLogger creates a configured logger using the standard logging package
func setupLogger(cfg *config.SessionServiceConfig) *logging.Logger {
	// Create the go-logging logger (matches other services)
	var logConfig log.Config
	if cfg.Logging != nil {
		// Set defaults if values are empty
		level := cfg.Logging.Level
		if level == "" {
			level = "info"
		}
		format := cfg.Logging.Format
		if format == "" {
			format = "text"
		}
		output := cfg.Logging.Output
		if output == "" {
			output = "stdout"
		}
		
		logConfig = log.Config{
			Level:  level,
			Format: format,
			Output: output,
			File: &log.FileConfig{
				Directory: "./logs",
				Filename:  "session-service.log",
				MaxSize:   "100MB",
				MaxFiles:  10,
				MaxAge:    "30d",
				Compress:  true,
			},
		}
	} else {
		logConfig = log.Config{
			Level:  "info",
			Format: "text",
			Output: "stdout",
		}
	}
	
	return log.SetupLogger("session-service", logConfig)
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
		// Use basic logger for config load errors
		basicLogger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		basicLogger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Setup structured logging based on configuration  
	goLogger := setupLogger(cfg)
	
	// Create silent slog logger for internal components (we'll use go-logging for main messages)
	// Set to a high level to suppress most internal logging
	slogLogger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelError, // Only show errors from internal components
	}))

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
			Address        string `yaml:"address" default:"0.0.0.0"`
			Port           int    `yaml:"port" default:"2222"`
			IdleTimeout    string `yaml:"idle_timeout" default:"1h"`
			HostKey        string `yaml:"host_key" default:""`
			PasswordAuth   bool   `yaml:"password_auth" default:"true"`
			PublicKeyAuth  bool   `yaml:"public_key_auth" default:"false"`
			AllowAnonymous bool   `yaml:"allow_anonymous" default:"true"`
		}{
			Address:        cfg.SSH.Host,
			Port:           cfg.SSH.Port,
			IdleTimeout:    "1h",
			HostKey:        cfg.SSH.HostKeyPath,
			PasswordAuth:   cfg.SSH.Auth.PasswordAuth,
			PublicKeyAuth:  cfg.SSH.Auth.PublicKeyAuth,
			AllowAnonymous: cfg.SSH.Auth.AllowAnonymous,
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
	}

	// Set banner configuration if available
	if cfg.Menu != nil && cfg.Menu.Banners != nil {
		sessionConfig.Menu.Banners.MainAnon = cfg.Menu.Banners.MainAnon
		sessionConfig.Menu.Banners.MainUser = cfg.Menu.Banners.MainUser
		sessionConfig.Menu.Banners.WatchMenu = cfg.Menu.Banners.WatchMenu
	}

	// Create stateless session service
	sessionService, err := session.New(sessionConfig, slogLogger)
	if err != nil {
		goLogger.Errorf("Failed to create session service: %v", err)
		os.Exit(1)
	}

	// Start the service
	if err := sessionService.Start(); err != nil {
		goLogger.Errorf("Failed to start session service: %v", err)
		os.Exit(1)
	}

	goLogger.Infof("Session Service started - SSH: %d, HTTP: %d, gRPC: %d, Max Connections: %d",
		sessionConfig.SSH.Port,
		sessionConfig.HTTP.Port,
		sessionConfig.GRPC.Port,
		sessionConfig.MaxConnections,
	)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	goLogger.Info("Shutting down gracefully...")

	// Shutdown the service
	if err := sessionService.Stop(); err != nil {
		goLogger.Errorf("Error during shutdown: %v", err)
	}

	goLogger.Info("Session Service stopped")
}
