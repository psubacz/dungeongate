package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/dungeongate/internal/session/connection"
)

// HTTPServer provides HTTP API for session management
type HTTPServer struct {
	config      *HTTPConfig
	server      *http.Server
	connManager *connection.Manager
	logger      *slog.Logger
}

// HTTPConfig holds HTTP server configuration
type HTTPConfig struct {
	Address string
	Port    int
}

// NewHTTPServer creates a new HTTP server
func NewHTTPServer(config *HTTPConfig, connManager *connection.Manager, logger *slog.Logger) *HTTPServer {
	return &HTTPServer{
		config:      config,
		connManager: connManager,
		logger:      logger,
	}
}

// Start starts the HTTP server
func (h *HTTPServer) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// Register handlers
	mux.HandleFunc("/health", h.healthHandler)
	mux.HandleFunc("/stats", h.statsHandler)
	mux.HandleFunc("/connections", h.connectionsHandler)

	addr := fmt.Sprintf("%s:%d", h.config.Address, h.config.Port)
	h.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	h.logger.Info("HTTP server starting", "address", addr)

	go func() {
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			h.logger.Error("HTTP server error", "error", err)
		}
	}()

	return nil
}

// Stop stops the HTTP server
func (h *HTTPServer) Stop(ctx context.Context) error {
	if h.server != nil {
		h.logger.Info("HTTP server stopping")
		return h.server.Shutdown(ctx)
	}
	return nil
}

// healthHandler handles health check requests
func (h *HTTPServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status":  "healthy",
		"service": "session-service",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// statsHandler handles connection statistics requests
func (h *HTTPServer) statsHandler(w http.ResponseWriter, r *http.Request) {
	if h.connManager == nil {
		http.Error(w, "Connection manager not available", http.StatusInternalServerError)
		return
	}

	stats := h.connManager.GetStats()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// connectionsHandler handles connection listing requests
func (h *HTTPServer) connectionsHandler(w http.ResponseWriter, r *http.Request) {
	if h.connManager == nil {
		http.Error(w, "Connection manager not available", http.StatusInternalServerError)
		return
	}

	stats := h.connManager.GetStats()

	// Return a simplified connection list
	response := map[string]interface{}{
		"total_connections":  stats.Total,
		"active_connections": stats.Active,
		"note":               "Detailed stats available from Game Service",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
