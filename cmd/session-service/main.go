package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dungeongate/internal/session"
	"github.com/dungeongate/internal/user"
	"github.com/dungeongate/pkg/config"
	"github.com/dungeongate/pkg/database"
	"github.com/dungeongate/pkg/encryption"
	"github.com/dungeongate/pkg/ttyrec"
)

var (
	version   string = "dev"
	buildTime string = "unknown"
	gitCommit string = "unknown"
)

func main() {
	var (
		configFile  = flag.String("config", "configs/development/session-service.yaml", "Path to configuration file")
		showVersion = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("DungeonGate Session Service\n")
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Build Time: %s\n", buildTime)
		fmt.Printf("Git Commit: %s\n", gitCommit)
		return
	}

	// Load configuration
	cfg, err := config.LoadSessionServiceConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Setup database
	db, err := database.NewConnection(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Setup encryption
	encryptor, err := encryption.New(cfg.Encryption)
	if err != nil {
		log.Fatalf("Failed to initialize encryption: %v", err)
	}

	// Setup TTY recorder
	recorder, err := ttyrec.NewRecorder(cfg.SessionManagement.TTYRec)
	if err != nil {
		log.Fatalf("Failed to initialize TTY recorder: %v", err)
	}

	// Setup user service
	userConfig := &config.UserServiceConfig{
		Database: cfg.Database,
	}
	userService, err := user.NewService(db, userConfig, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize user service: %v", err)
	}

	// Setup auth middleware if enabled
	var authMiddleware *session.AuthMiddleware
	if cfg.Auth != nil && cfg.Auth.Enabled {
		authMiddleware, err = session.NewAuthMiddleware(cfg.Auth.ServiceAddress, cfg.Auth.Enabled)
		if err != nil {
			log.Printf("Warning: Failed to initialize auth middleware: %v", err)
			log.Printf("Falling back to direct user service authentication")
		}
	}

	// Setup session service with or without auth middleware
	var sessionService *session.Service
	if authMiddleware != nil {
		sessionService = session.NewServiceWithAuth(db, encryptor, recorder, cfg, userService, authMiddleware)
	} else {
		sessionService = session.NewService(db, encryptor, recorder, cfg, userService)
	}

	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start SSH server
	sshServer, err := session.NewSSHServer(sessionService, cfg)
	if err != nil {
		log.Fatalf("Failed to create SSH server: %v", err)
	}

	go func() {
		log.Printf("Starting SSH server on %s:%d", cfg.SSH.Host, cfg.SSH.Port)
		if err := sshServer.Start(ctx, cfg.SSH.Port); err != nil {
			log.Printf("SSH server error: %v", err)
		}
	}()

	// Start HTTP server
	httpHandler := session.NewHTTPHandler(sessionService)
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: httpHandler,
	}

	go func() {
		log.Printf("Starting HTTP server on port %d", cfg.Server.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutting down gracefully...")

	// Shutdown HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	// Cancel context to stop SSH server and other services
	cancel()

	log.Println("Server stopped")
}
