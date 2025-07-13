package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/dungeongate/internal/games/application"
	grpc_service "github.com/dungeongate/internal/games/infrastructure/grpc"
	"github.com/dungeongate/internal/games/infrastructure/repository"
	games_pb "github.com/dungeongate/pkg/api/games/v2"
	"github.com/dungeongate/pkg/config"
	"github.com/dungeongate/pkg/database"
	"github.com/dungeongate/pkg/logging"
	"github.com/dungeongate/pkg/metrics"
)

var (
	version   string = "dev"
	buildTime string = "unknown"
	gitCommit string = "unknown"
)

const serviceName = "game-service"

var logger *slog.Logger

func main() {
	var (
		configFile  = flag.String("config", "configs/game-service.yaml", "Path to configuration file")
		showVersion = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("DungeonGate Game Service\n")
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Build Time: %s\n", buildTime)
		fmt.Printf("Git Commit: %s\n", gitCommit)
		return
	}

	// Load configuration first to set up proper logging
	cfg, err := loadConfig(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logging with configuration
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
	logger = logging.NewLoggerBasic(serviceName, level, format, output)
	logger.Info("Starting Game Service", "version", version)

	// Initialize metrics registry
	metricsRegistry := metrics.NewRegistry(serviceName, version, buildTime, gitCommit, logger)

	// Start metrics server if enabled
	if cfg.Metrics != nil && cfg.Metrics.Enabled {
		go func() {
			if err := metricsRegistry.StartMetricsServer(cfg.Metrics.Port); err != nil {
				logger.Error("Failed to start metrics server", "error", err)
			}
		}()
		logger.Info("Metrics server starting", "port", cfg.Metrics.Port)
	}

	// Initialize database
	db, err := initializeDatabase(cfg)
	if err != nil {
		logger.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Initialize application services
	appServices := initializeApplicationServices(db, metricsRegistry)

	// Initialize gRPC server
	grpcServer := initializeGRPCServer(cfg, appServices, metricsRegistry)

	// Initialize HTTP server
	httpServer := initializeHTTPServer(cfg, appServices)

	// Start servers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start gRPC server
	go func() {
		if err := startGRPCServer(ctx, cfg, grpcServer); err != nil {
			logger.Error("gRPC server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Start HTTP server
	go func() {
		if err := startHTTPServer(ctx, cfg, httpServer); err != nil {
			logger.Error("HTTP server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	waitForShutdown(ctx, cancel, grpcServer, httpServer, metricsRegistry, cfg)
}

// loadConfig loads the service configuration
func loadConfig(configFile string) (*config.GameServiceConfig, error) {
	// Try the specified config file first
	if _, err := os.Stat(configFile); err == nil {
		return config.LoadGameServiceConfig(configFile)
	}

	// Try other common locations
	configPaths := []string{
		"./configs/game-service.yaml",
		"./configs/game-service.yaml",
		"/etc/dungeongate/game-service.yaml",
	}

	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			return config.LoadGameServiceConfig(path)
		}
	}

	// Use default configuration if no file found
	fmt.Fprintf(os.Stderr, "Warning: No configuration file found, using defaults\n")
	cfg := &config.GameServiceConfig{}
	return cfg, nil
}

// initializeDatabase initializes the database connection
func initializeDatabase(cfg *config.GameServiceConfig) (*database.Connection, error) {
	dbConfig := cfg.Database
	if dbConfig == nil {
		return nil, fmt.Errorf("database configuration is required")
	}

	db, err := database.NewConnection(dbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Run migrations if enabled
	// TODO: Add AutoMigrate field to DatabaseConfig or implement migration logic
	// if dbConfig.AutoMigrate {
	//	if err := database.RunMigrations(db, "migrations/games"); err != nil {
	//		log.Printf("Warning: failed to run migrations: %v", err)
	//	}
	// }

	return db, nil
}

// initializeDefaultGames adds default games to the repository for development
func initializeDefaultGames(gameService *application.GameService) {
	ctx := context.Background()

	// Add NetHack as a default game
	nethackReq := &application.CreateGameRequest{
		ID:               "nethack",
		Name:             "NetHack",
		ShortName:        "nh",
		Description:      "The classic dungeon exploration game",
		Category:         "roguelike",
		Tags:             []string{"roguelike", "classic", "dungeon"},
		Version:          "3.6.7",
		Difficulty:       7,
		BinaryPath:       "/opt/homebrew/bin/nethack",
		BinaryArgs:       []string{"-u", "${USERNAME}"},
		WorkingDirectory: "/opt/homebrew/Cellar/nethack/3.6.7/libexec",
		Environment: map[string]string{
			"TERM":           "xterm-256color",
			"USER":           "${USERNAME}",
			"HOME":           "/opt/homebrew/Cellar/nethack/3.6.7/libexec/${USERNAME}",
			"HACKDIR":        "/opt/homebrew/Cellar/nethack/3.6.7/libexec",
			"NETHACKDIR":     "/opt/homebrew/Cellar/nethack/3.6.7/libexec",
			"NETHACKOPTIONS": "@/opt/homebrew/Cellar/nethack/3.6.7/libexec/${USERNAME}.nethackrc",
		},
		CPULimit:                 "500m",
		MemoryLimit:              "256Mi",
		DiskLimit:                "1Gi",
		TimeoutSeconds:           14400, // 4 hours
		RunAsUser:                1000,
		RunAsGroup:               1000,
		ReadOnlyRootFilesystem:   false,
		AllowPrivilegeEscalation: false,
		NetworkIsolated:          true,
		BlockInternet:            true,
	}

	// Try to create the game, ignore errors if it already exists
	gameService.CreateGame(ctx, nethackReq)
}

// ApplicationServices holds all application services
type ApplicationServices struct {
	GameService    *application.GameService
	SessionService *application.SessionService
}

// initializeApplicationServices initializes all application services
func initializeApplicationServices(db *database.Connection, metricsRegistry *metrics.Registry) *ApplicationServices {
	// Initialize stub repositories for development
	gameRepo := repository.NewStubGameRepository()
	sessionRepo := repository.NewStubSessionRepository()
	saveRepo := repository.NewStubSaveRepository()
	eventRepo := repository.NewStubEventRepository()

	// Create unit of work
	uow := repository.NewStubUnitOfWork(gameRepo, sessionRepo, saveRepo, eventRepo)

	// Initialize application services
	gameService := application.NewGameService(gameRepo, sessionRepo, saveRepo, eventRepo, uow)
	sessionService := application.NewSessionService(sessionRepo, gameRepo, saveRepo, eventRepo, uow)

	// Add default games for development
	initializeDefaultGames(gameService)

	return &ApplicationServices{
		GameService:    gameService,
		SessionService: sessionService,
	}
}

// initializeGRPCServer initializes the gRPC server
func initializeGRPCServer(cfg *config.GameServiceConfig, appServices *ApplicationServices, metricsRegistry *metrics.Registry) *grpc.Server {
	server := grpc.NewServer()

	// Register health check service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(server, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// Register game service with slog logger
	gameServiceServer := grpc_service.NewGameServiceServer(cfg, appServices.GameService, appServices.SessionService, logger)
	games_pb.RegisterGameServiceServer(server, gameServiceServer)

	return server
}

// initializeHTTPServer initializes the HTTP server
func initializeHTTPServer(cfg *config.GameServiceConfig, appServices *ApplicationServices) *http.Server {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status": "healthy", "service": "%s", "version": "%s"}`, serviceName, version)
	})

	// Metrics endpoint
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement Prometheus metrics
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "# HELP game_service_up Service is up\n")
		fmt.Fprintf(w, "# TYPE game_service_up gauge\n")
		fmt.Fprintf(w, "game_service_up 1\n")
	})

	// Game management endpoints (REST API)
	mux.HandleFunc("/api/v1/games", handleGamesAPI(appServices.GameService))
	mux.HandleFunc("/api/v1/sessions", handleSessionsAPI(appServices.SessionService))

	return &http.Server{
		Addr:         fmt.Sprintf(":%d", getHTTPPort(cfg)),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// getHTTPPort returns the HTTP port from config or default
func getHTTPPort(cfg *config.GameServiceConfig) int {
	if cfg.Server != nil {
		return cfg.Server.Port
	}
	return 8084 // Default port
}

// getGRPCPort returns the gRPC port from config or default
func getGRPCPort(cfg *config.GameServiceConfig) int {
	if cfg.Server != nil {
		return cfg.Server.GRPCPort
	}
	return 50051 // Default port
}

// startGRPCServer starts the gRPC server
func startGRPCServer(ctx context.Context, cfg *config.GameServiceConfig, server *grpc.Server) error {
	addr := fmt.Sprintf(":%d", getGRPCPort(cfg))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	logger.Info("gRPC server starting", "address", addr)

	go func() {
		<-ctx.Done()
		logger.Info("Shutting down gRPC server...")
		server.GracefulStop()
	}()

	if err := server.Serve(listener); err != nil {
		return fmt.Errorf("gRPC server failed: %w", err)
	}

	return nil
}

// startHTTPServer starts the HTTP server
func startHTTPServer(ctx context.Context, cfg *config.GameServiceConfig, server *http.Server) error {
	logger.Info("HTTP server starting", "address", server.Addr)

	go func() {
		<-ctx.Done()
		logger.Info("Shutting down HTTP server...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("HTTP server shutdown error", "error", err)
		}
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server failed: %w", err)
	}

	return nil
}

// waitForShutdown waits for shutdown signals and gracefully shuts down servers
func waitForShutdown(ctx context.Context, cancel context.CancelFunc, grpcServer *grpc.Server, httpServer *http.Server, metricsRegistry *metrics.Registry, cfg *config.GameServiceConfig) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	logger.Info("Shutdown signal received, starting graceful shutdown...")

	cancel()

	// Stop metrics server
	if cfg.Metrics != nil && cfg.Metrics.Enabled {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := metricsRegistry.StopMetricsServer(shutdownCtx); err != nil {
			logger.Error("Error stopping metrics server", "error", err)
		}
	}

	// Wait a bit for servers to shut down gracefully
	time.Sleep(2 * time.Second)
	logger.Info("Game service shutdown complete")
}

// HTTP API handlers

// handleGamesAPI handles game-related API requests
func handleGamesAPI(gameService *application.GameService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if gameService == nil {
			http.Error(w, "Game service not initialized", http.StatusServiceUnavailable)
			return
		}

		switch r.Method {
		case http.MethodGet:
			// List games
			games, err := gameService.ListEnabledGames(r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			// TODO: Serialize games to JSON
			fmt.Fprintf(w, `{"games": [], "count": %d}`, len(games))

		case http.MethodPost:
			// Create game
			// TODO: Parse request body and create game
			http.Error(w, "Not implemented", http.StatusNotImplemented)

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleSessionsAPI handles session-related API requests
func handleSessionsAPI(sessionService *application.SessionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if sessionService == nil {
			http.Error(w, "Session service not initialized", http.StatusServiceUnavailable)
			return
		}

		switch r.Method {
		case http.MethodGet:
			// List sessions
			sessions, err := sessionService.ListActiveSessions(r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"sessions": [], "count": %d}`, len(sessions))

		case http.MethodPost:
			// Start session
			// TODO: Parse request body and start session
			http.Error(w, "Not implemented", http.StatusNotImplemented)

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
