package session

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// HTTP handlers for the session service

// HTTP handler for session management
func handleSessionsHTTP(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGetSessions(service, w, r)
		case http.MethodPost:
			handleCreateSession(service, w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleGetSessions(service *Service, w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	sessions, err := service.GetActiveSessions(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

func handleCreateSession(service *Service, w http.ResponseWriter, r *http.Request) {
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	session, err := service.CreateSession(ctx, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

// Health check handler
func handleHealthCheck(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
			"service":   "session-service",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// Metrics handler
func handleMetrics(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metrics := service.GetMetrics()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metrics)
	}
}

// Spectate handler (simplified without WebSocket)
func handleSpectateHTTP(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse session ID from URL
		sessionID := r.URL.Query().Get("session_id")
		if sessionID == "" {
			http.Error(w, "session_id parameter required", http.StatusBadRequest)
			return
		}

		// Get user info from query params
		userIDStr := r.URL.Query().Get("user_id")
		username := r.URL.Query().Get("username")

		if username == "" {
			username = "anonymous"
		}

		userID := 0
		if userIDStr != "" {
			if id, err := strconv.Atoi(userIDStr); err == nil {
				userID = id
			}
		}

		// Add spectator
		ctx := context.Background()
		if err := service.AddSpectator(ctx, sessionID, userID, username); err != nil {
			http.Error(w, fmt.Sprintf("Failed to add spectator: %v", err), http.StatusInternalServerError)
			return
		}

		// Return success response
		response := map[string]interface{}{
			"status":     "success",
			"message":    "Spectator added successfully",
			"session_id": sessionID,
			"user_id":    userID,
			"username":   username,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}
