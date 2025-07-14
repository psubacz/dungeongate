package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
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
	"github.com/dungeongate/pkg/logging"
	"github.com/dungeongate/pkg/metrics"
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
		configFile  = flag.String("config", "configs/auth-service.yaml", "Path to configuration file")
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

	// Load configuration first to get logging config
	cfg, err := config.LoadUserServiceConfig(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize standardized logging
	logger := logging.NewLoggerBasic("auth-service", cfg.Logging.Level, cfg.Logging.Format, cfg.Logging.Output)
	logger.Info("Starting DungeonGate Auth Service")

	// Initialize metrics registry
	metricsRegistry := metrics.NewRegistry("auth-service", version, buildTime, gitCommit, logger)

	// Start metrics server if enabled
	if cfg.Metrics != nil && cfg.Metrics.Enabled {
		go func() {
			if err := metricsRegistry.StartMetricsServer(cfg.Metrics.Port); err != nil {
				logger.Error("Failed to start metrics server", "error", err)
			}
		}()
		logger.Info("Metrics server starting", "port", cfg.Metrics.Port)
	}

	// Setup database
	db, err := database.NewConnection(cfg.Database)
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Setup encryption
	encryptor, err := encryption.New(&config.EncryptionConfig{
		Enabled:             true,
		Algorithm:           "AES-256-GCM",
		KeyRotationInterval: "24h",
	})
	if err != nil {
		logger.Error("Failed to initialize encryption", "error", err)
		os.Exit(1)
	}

	// Setup user service
	sessionCfg := config.GetDefaultDevelopmentConfig()
	userService, err := user.NewService(db, cfg, sessionCfg)
	if err != nil {
		logger.Error("Failed to create user service", "error", err)
		os.Exit(1)
	}

	// Generate JWT secret if not provided
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		logger.Info("JWT_SECRET not set, generating random secret (not recommended for production)")
		secretBytes := make([]byte, 32)
		if _, err := rand.Read(secretBytes); err != nil {
			logger.Error("Failed to generate JWT secret", "error", err)
			os.Exit(1)
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

	authService := auth.NewService(db, userService, *encryptor, authConfig, logger)

	// Setup context for graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup gRPC server with metrics interceptors
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(metricsRegistry.UnaryServerInterceptor()),
		grpc.StreamInterceptor(metricsRegistry.StreamServerInterceptor()),
	)
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
		logger.Error("Failed to listen on gRPC port", "port", grpcPort, "error", err)
		os.Exit(1)
	}

	go func() {
		logger.Info("Starting Auth Service gRPC server", "port", grpcPort)
		if err := grpcServer.Serve(grpcListener); err != nil {
			logger.Info("gRPC server error", "error", err)
		}
	}()

	// Get HTTP port from config or use default
	httpPort := 8081 // default port
	if cfg.Server != nil && cfg.Server.Port > 0 {
		httpPort = cfg.Server.Port
	}

	// Setup HTTP server for health checks
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"healthy","service":"auth-service","version":"%s"}`, version)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
		fmt.Fprintf(w, "Auth Service - gRPC API available on port %d", grpcPort)
	})

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", httpPort),
		Handler: metricsRegistry.HTTPMiddleware()(mux),
	}

	go func() {
		logger.Info("Starting Auth Service HTTP server", "port", httpPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Info("HTTP server error", "error", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	logger.Info("Shutting down gracefully...")

	// Shutdown gRPC server
	grpcServer.GracefulStop()

	// Shutdown HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Info("HTTP server shutdown error", "error", err)
	}

	// Stop metrics server
	if cfg.Metrics != nil && cfg.Metrics.Enabled {
		if err := metricsRegistry.StopMetricsServer(shutdownCtx); err != nil {
			logger.Error("Error stopping metrics server", "error", err)
		}
	}

	// Cancel context
	cancel()

	logger.Info("Auth Service stopped")
}
