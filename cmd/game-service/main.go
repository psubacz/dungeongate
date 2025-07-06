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

	"github.com/dungeongate/pkg/config"
	"github.com/dungeongate/pkg/database"
)

var (
	version   string = "dev"
	buildTime string = "unknown"
	gitCommit string = "unknown"
)

func main() {
	var (
		configFile  = flag.String("config", "configs/development/local.yaml", "Path to configuration file")
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

	// Load configuration
	cfg, err := config.LoadGameServiceConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Setup database
	db, err := database.NewConnection(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Setup context for graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup HTTP server
	httpServer := &http.Server{
		Addr: fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, "Game Service OK")
				return
			}
			w.WriteHeader(http.StatusNotImplemented)
			fmt.Fprintf(w, "Game Service - Implementation in progress")
		}),
	}

	go func() {
		log.Printf("Starting Game Service HTTP server on port %d", cfg.Server.Port)
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

	// Cancel context
	cancel()

	log.Println("Game Service stopped")

	// Prevent unused variable warnings
	_ = db
}