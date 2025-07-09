package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dungeongate/internal/auth"
	"github.com/dungeongate/internal/user"
	proto "github.com/dungeongate/pkg/api/auth/v1"
	"github.com/dungeongate/pkg/config"
	"github.com/dungeongate/pkg/database"
	"github.com/dungeongate/pkg/encryption"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	version   string = "dev"
	buildTime string = "unknown"
	gitCommit string = "unknown"
)

func main() {
	var (
		configFile  = flag.String("config", "configs/development/auth-service.yaml", "Path to configuration file")
		showVersion = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("DungeonGate Auth Service\n")
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Build Time: %s\n", buildTime)
		fmt.Printf("Git Commit: %s\n", gitCommit)
		return
	}

	// Load configuration
	cfg, err := config.LoadUserServiceConfig(*configFile)
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
	encryptor, err := encryption.New(&config.EncryptionConfig{
		Enabled:             true,
		Algorithm:           "AES-256-GCM",
		KeyRotationInterval: "24h",
	})
	if err != nil {
		log.Fatalf("Failed to initialize encryption: %v", err)
	}

	// Setup user service
	sessionCfg := config.GetDefaultDevelopmentConfig()
	userService, err := user.NewService(db, cfg, sessionCfg)
	if err != nil {
		log.Fatalf("Failed to create user service: %v", err)
	}

	// Generate JWT secret if not provided
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Println("JWT_SECRET not set, generating random secret (not recommended for production)")
		secretBytes := make([]byte, 32)
		if _, err := rand.Read(secretBytes); err != nil {
			log.Fatalf("Failed to generate JWT secret: %v", err)
		}
		jwtSecret = hex.EncodeToString(secretBytes)
	}

	// Setup auth service
	authConfig := &auth.Config{
		JWTSecret:              jwtSecret,
		JWTIssuer:              "dungeongate-auth",
		AccessTokenExpiration:  15 * time.Minute,
		RefreshTokenExpiration: 7 * 24 * time.Hour,
		MaxLoginAttempts:       3,
		LockoutDuration:        15 * time.Minute,
	}

	authService := auth.NewService(db, userService, *encryptor, authConfig)

	// Setup context for graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup gRPC server
	grpcServer := grpc.NewServer()
	proto.RegisterAuthServiceServer(grpcServer, authService)
	reflection.Register(grpcServer)

	// Get gRPC port from config or use default
	grpcPort := 8082 // default port
	if cfg.Server != nil && cfg.Server.GRPCPort > 0 {
		grpcPort = cfg.Server.GRPCPort
	} else if cfg.Server != nil && cfg.Server.Port > 0 {
		grpcPort = cfg.Server.Port // fallback to main port
	}

	// Start gRPC server
	grpcListener, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		log.Fatalf("Failed to listen on port %d: %v", grpcPort, err)
	}

	go func() {
		log.Printf("Starting Auth Service gRPC server on port %d", grpcPort)
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Printf("gRPC server error: %v", err)
		}
	}()

	// Get HTTP port from config or use default
	httpPort := 8081 // default port
	if cfg.Server != nil && cfg.Server.Port > 0 {
		httpPort = cfg.Server.Port
	}

	// Setup HTTP server for health checks
	httpServer := &http.Server{
		Addr: fmt.Sprintf(":%d", httpPort),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, "Auth Service OK")
				return
			}
			w.WriteHeader(http.StatusNotImplemented)
			fmt.Fprintf(w, "Auth Service - gRPC API available on port %d", grpcPort)
		}),
	}

	go func() {
		log.Printf("Starting Auth Service HTTP server on port %d", httpPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutting down gracefully...")

	// Shutdown gRPC server
	grpcServer.GracefulStop()

	// Shutdown HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	// Cancel context
	cancel()

	log.Println("Auth Service stopped")
}
