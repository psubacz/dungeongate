package handlers

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dungeongate/internal/session/pools"
	"github.com/dungeongate/internal/session/resources"
	"github.com/dungeongate/internal/session/client"
	"github.com/dungeongate/internal/session/menu"
)

// ServiceConfig holds configuration for the pool-based service
type ServiceConfig struct {
	// Pool configurations
	ConnectionPool *pools.Config
	WorkerPool     *pools.WorkerConfig
	PTYPool        *pools.PTYConfig
	Backpressure   *pools.BackpressureConfig

	// Resource management configuration
	ResourceManagement *resources.Config
	PoolMetrics        *resources.MetricsConfig

	// Feature flags for migration
	Migration struct {
		UsePoolBasedHandlers bool `yaml:"use_pool_based_handlers"`
		FallbackToLegacy     bool `yaml:"fallback_to_legacy"`
		
		Handlers struct {
			SessionHandler bool `yaml:"session_handler"`
			AuthHandler    bool `yaml:"auth_handler"`
			GameHandler    bool `yaml:"game_handler"`
			StreamHandler  bool `yaml:"stream_handler"`
			MenuHandler    bool `yaml:"menu_handler"`
		} `yaml:"handlers"`
	} `yaml:"migration"`
}

// InitializePoolBasedService initializes the session service with pool-based architecture
func InitializePoolBasedService(config *ServiceConfig, authClient *client.AuthClient, gameClient *client.GameClient, menuHandler *menu.MenuHandler, logger *slog.Logger) (*SessionHandler, error) {
	// Check if pool-based handlers are enabled
	if !config.Migration.UsePoolBasedHandlers {
		return nil, fmt.Errorf("pool-based handlers not enabled in configuration")
	}

	logger.Info("Initializing pool-based session service")

	// Initialize pools
	connectionPool, err := pools.NewConnectionPool(config.ConnectionPool, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	workerPool, err := pools.NewWorkerPool(config.WorkerPool, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create worker pool: %w", err)
	}

	ptyPool, err := pools.NewPTYPool(config.PTYPool, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create PTY pool: %w", err)
	}

	backpressure, err := pools.NewBackpressureManager(config.Backpressure, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create backpressure manager: %w", err)
	}

	// Initialize resource management
	resourceLimiter, err := resources.NewResourceLimiter(config.ResourceManagement, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource limiter: %w", err)
	}

	resourceTracker := resources.NewResourceTracker(logger)

	metricsRegistry := resources.NewMetricsRegistry(config.PoolMetrics, logger)

	// Create specialized handlers
	authHandler := NewAuthHandler(authClient, resourceLimiter, workerPool, metricsRegistry, logger)
	gameHandler := NewGameHandler(gameClient, ptyPool, resourceTracker, workerPool, metricsRegistry, logger)
	streamHandler := NewStreamHandler(resourceTracker, workerPool, metricsRegistry, logger)

	// Create session handler
	sessionHandler := NewSessionHandler(
		connectionPool, workerPool, ptyPool, backpressure,
		resourceLimiter, resourceTracker, metricsRegistry,
		authHandler, gameHandler, streamHandler, menuHandler,
		logger)

	logger.Info("Pool-based session service initialized successfully")
	return sessionHandler, nil
}

// StartPoolBasedService starts all pool components
func StartPoolBasedService(ctx context.Context, sessionHandler *SessionHandler) error {
	// Start all pools in order
	if err := sessionHandler.connectionPool.Start(ctx); err != nil {
		return fmt.Errorf("failed to start connection pool: %w", err)
	}

	if err := sessionHandler.workerPool.Start(ctx); err != nil {
		return fmt.Errorf("failed to start worker pool: %w", err)
	}

	if err := sessionHandler.ptyPool.Start(ctx); err != nil {
		return fmt.Errorf("failed to start PTY pool: %w", err)
	}

	if err := sessionHandler.backpressure.Start(ctx); err != nil {
		return fmt.Errorf("failed to start backpressure manager: %w", err)
	}

	if err := sessionHandler.resourceTracker.Start(ctx); err != nil {
		return fmt.Errorf("failed to start resource tracker: %w", err)
	}

	return nil
}

// ShutdownPoolBasedService gracefully shuts down all components
func ShutdownPoolBasedService(ctx context.Context, sessionHandler *SessionHandler) error {
	return sessionHandler.Shutdown(ctx)
}

// ExampleUsage shows how to integrate the pool-based architecture
func ExampleUsage() {
	/*
	// In cmd/session-service/main.go or similar:

	// Load configuration with pool settings
	config := &ServiceConfig{
		ConnectionPool: &pools.Config{
			MaxConnections: 1000,
			QueueSize:      100,
			QueueTimeout:   30 * time.Second,
			IdleTimeout:    300 * time.Second,
		},
		WorkerPool: &pools.WorkerConfig{
			MinWorkers:     10,
			MaxWorkers:     100,
			QueueSize:      1000,
			WorkerTimeout:  60 * time.Second,
		},
		// ... other pool configs
		Migration: Migration{
			UsePoolBasedHandlers: true,
			FallbackToLegacy:     false,
			Handlers: Handlers{
				SessionHandler: true,
				AuthHandler:    true,
				GameHandler:    true,
				StreamHandler:  true,
				MenuHandler:    true,
			},
		},
	}

	// Initialize existing clients and components
	authClient := client.NewAuthClient(...)
	gameClient := client.NewGameClient(...)
	menuHandler := menu.NewMenuHandler(...)

	// Initialize pool-based service
	sessionHandler, err := InitializePoolBasedService(config, authClient, gameClient, menuHandler, logger)
	if err != nil {
		log.Fatal("Failed to initialize pool-based service:", err)
	}

	// Start all components
	ctx := context.Background()
	if err := StartPoolBasedService(ctx, sessionHandler); err != nil {
		log.Fatal("Failed to start pool-based service:", err)
	}

	// Use sessionHandler.HandleNewConnection instead of old handler
	// Replace: oldHandler.HandleConnection(ctx, conn, config)
	// With:    sessionHandler.HandleNewConnection(ctx, conn, config)

	// Graceful shutdown
	defer func() {
		if err := ShutdownPoolBasedService(ctx, sessionHandler); err != nil {
			log.Error("Failed to shutdown pool-based service:", err)
		}
	}()
	*/
}