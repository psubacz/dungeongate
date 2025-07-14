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
	"github.com/dungeongate/internal/session/banner"
	"github.com/dungeongate/internal/session/handlers"
	"github.com/dungeongate/internal/session/pools"
	"github.com/dungeongate/internal/session/resources"
	"github.com/dungeongate/internal/session/client"
	"github.com/dungeongate/internal/session/menu"
	"github.com/dungeongate/pkg/config"
	"github.com/dungeongate/pkg/logging"
	"github.com/dungeongate/pkg/metrics"
)

var (
	version   string = "dev"
	buildTime string = "unknown"
	gitCommit string = "unknown"
)

// createPoolServiceConfig creates pool-based service configuration from session config
func createPoolServiceConfig(cfg *config.SessionServiceConfig) *handlers.ServiceConfig {
	return &handlers.ServiceConfig{
		ConnectionPool: &pools.Config{
			MaxConnections: 1000, // Default values, could be made configurable
			QueueSize:      100,
			QueueTimeout:   30 * time.Second,
			IdleTimeout:    300 * time.Second,
			DrainTimeout:   60 * time.Second,
			WorkerPoolSize: 50,  // This was missing!
			MaxPTYs:        500,
		},
		WorkerPool: &pools.WorkerConfig{
			PoolSize:        50,
			QueueSize:       1000,
			WorkerTimeout:   30 * time.Second,
			ShutdownTimeout: 10 * time.Second,
		},
		PTYPool: &pools.PTYConfig{
			MaxPTYs:         500,
			ReuseTimeout:    5 * time.Minute,
			CleanupInterval: 1 * time.Minute,
			FDLimit:         1024,
		},
		Backpressure: &pools.BackpressureConfig{
			Enabled:           true,
			CircuitBreaker:    true,
			LoadShedding:      true,
			FailureThreshold:  10,
			RecoveryTimeout:   60 * time.Second,
			QueueSize:         100,
			CPUThreshold:      0.8,
			MemoryThreshold:   0.9,
		},
		ResourceManagement: &resources.Config{
			// Basic configuration for now - can be expanded later
		},
		PoolMetrics: &resources.MetricsConfig{
			CollectionInterval: 10 * time.Second,
			ExportInterval:     30 * time.Second,
			RetentionPeriod:    24 * time.Hour,
			DefaultBuckets:     []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		Migration: struct {
			UsePoolBasedHandlers bool `yaml:"use_pool_based_handlers"`
			FallbackToLegacy     bool `yaml:"fallback_to_legacy"`
			
			Handlers struct {
				SessionHandler bool `yaml:"session_handler"`
				AuthHandler    bool `yaml:"auth_handler"`
				GameHandler    bool `yaml:"game_handler"`
				StreamHandler  bool `yaml:"stream_handler"`
				MenuHandler    bool `yaml:"menu_handler"`
			} `yaml:"handlers"`
		}{
			UsePoolBasedHandlers: true,
			FallbackToLegacy:     false,
			Handlers: struct {
				SessionHandler bool `yaml:"session_handler"`
				AuthHandler    bool `yaml:"auth_handler"`
				GameHandler    bool `yaml:"game_handler"`
				StreamHandler  bool `yaml:"stream_handler"`
				MenuHandler    bool `yaml:"menu_handler"`
			}{
				SessionHandler: true,
				AuthHandler:    true,
				GameHandler:    true,
				StreamHandler:  true,
				MenuHandler:    true,
			},
		},
	}
}

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
		sessionConfig.Menu.Banners.WatchMenu = cfg.Menu.Banners.WatchMenu
		sessionConfig.Menu.Banners.IdleMode = cfg.Menu.Banners.IdleMode
	}

	// Check if pool-based handlers are enabled via environment variable
	usePoolHandlers := os.Getenv("DUNGEONGATE_USE_POOL_HANDLERS") == "true"
	if usePoolHandlers {
		logger.Info("Starting with pool-based architecture")
		
		// Create service config for pool-based architecture
		poolServiceConfig := createPoolServiceConfig(cfg)
		logger.Info("Pool service config created",
			"max_connections", poolServiceConfig.ConnectionPool.MaxConnections,
			"worker_pool_size", poolServiceConfig.ConnectionPool.WorkerPoolSize,
			"worker_config_pool_size", poolServiceConfig.WorkerPool.PoolSize)
		
		// Create clients
		authClient, err := client.NewAuthClient(cfg.Services.AuthService, logger)
		if err != nil {
			logger.Error("Failed to create auth client", "error", err)
			os.Exit(1)
		}
		
		gameClient, err := client.NewGameClient(cfg.Services.GameService, logger)
		if err != nil {
			logger.Error("Failed to create game client", "error", err)
			os.Exit(1)
		}
		
		// Create menu handler - need to create banner manager first
		bannerConfig := &banner.BannerConfig{
			MainAnon:           cfg.Menu.Banners.MainAnon,
			MainUser:           cfg.Menu.Banners.MainUser,
			WatchMenu:          cfg.Menu.Banners.WatchMenu,
			IdleMode:           cfg.Menu.Banners.IdleMode,
			ServiceUnavailable: "", // Use default
		}
		bannerManager := banner.NewBannerManager(bannerConfig)
		
		// Create menu handler
		menuHandler := menu.NewMenuHandler(bannerManager, gameClient, authClient, logger)
		
		// Create server configuration for pool-based architecture
		serverConfig := &handlers.ServerConfig{
			SSH: handlers.SSHServerConfig{
				Address:     cfg.SSH.Host,
				Port:        cfg.SSH.Port,
				HostKeyPath: cfg.SSH.HostKeyPath,
				Banner:      "Welcome to DungeonGate Pool-Based Architecture!\r\n",
			},
			HTTP: handlers.HTTPServerConfig{
				Address: "0.0.0.0",
				Port:    cfg.Server.Port,
			},
			GRPC: handlers.GRPCServerConfig{
				Address: "0.0.0.0", 
				Port:    cfg.Server.Port + 1000, // Use different port for gRPC
			},
		}
		
		// Initialize pool-based service
		sessionHandler, err := handlers.InitializePoolBasedService(poolServiceConfig, authClient, gameClient, menuHandler, serverConfig, logger)
		if err != nil {
			logger.Error("Failed to initialize pool-based service", "error", err)
			if poolServiceConfig.Migration.FallbackToLegacy {
				logger.Info("Falling back to legacy handlers")
			} else {
				os.Exit(1)
			}
		} else {
			// Start pool-based service
			ctx := context.Background()
			if err := handlers.StartPoolBasedService(ctx, sessionHandler); err != nil {
				logger.Error("Failed to start pool-based service", "error", err)
				if poolServiceConfig.Migration.FallbackToLegacy {
					logger.Info("Falling back to legacy handlers")
				} else {
					os.Exit(1)
				}
			} else {
				logger.Info("Pool-based Session Service started successfully",
					"ssh_port", sessionConfig.SSH.Port,
					"http_port", sessionConfig.HTTP.Port,
					"grpc_port", sessionConfig.GRPC.Port,
					"max_connections", poolServiceConfig.ConnectionPool.MaxConnections,
				)
				
				// Wait for interrupt signal
				sigChan := make(chan os.Signal, 1)
				signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
				
				<-sigChan
				logger.Info("Shutting down pool-based service gracefully...")
				
				// Shutdown the pool-based service
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				
				if err := handlers.ShutdownPoolBasedService(shutdownCtx, sessionHandler); err != nil {
					logger.Error("Error during pool-based service shutdown", "error", err)
				}
				
				// Stop metrics server
				if cfg.Metrics != nil && cfg.Metrics.Enabled {
					if err := metricsRegistry.StopMetricsServer(shutdownCtx); err != nil {
						logger.Error("Error stopping metrics server", "error", err)
					}
				}
				
				logger.Info("Pool-based Session Service stopped")
				return
			}
		}
	}

	// Create stateless session service (legacy path)
	// DEPRECATED: This will be replaced by pool-based architecture
	// Set DUNGEONGATE_USE_POOL_HANDLERS=true to use the new architecture
	logger.Warn("Using legacy session service - this will be deprecated", 
		"migration_hint", "Set DUNGEONGATE_USE_POOL_HANDLERS=true to use pool-based architecture")
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
