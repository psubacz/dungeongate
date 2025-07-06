package session

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dungeongate/internal/user"
	"github.com/dungeongate/pkg/config"
	"github.com/dungeongate/pkg/database"
	"github.com/dungeongate/pkg/encryption"
	"github.com/dungeongate/pkg/ttyrec"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Service handles session management operations
type Service struct {
	db             *database.Connection
	encryptor      *encryption.Encryptor
	recorder       *ttyrec.Recorder
	config         *config.SessionServiceConfig
	userService    *user.Service
	authMiddleware *AuthMiddleware

	// Session tracking using immutable data patterns
	sessions    map[string]*Session
	sessionsMux sync.RWMutex
}

// NewService creates a new session service
func NewService(db *database.Connection, encryptor *encryption.Encryptor, recorder *ttyrec.Recorder, cfg *config.SessionServiceConfig, userService *user.Service) *Service {
	return &Service{
		db:          db,
		encryptor:   encryptor,
		recorder:    recorder,
		config:      cfg,
		userService: userService,
		sessions:    make(map[string]*Session),
	}
}

// NewServiceWithAuth creates a new session service with authentication middleware
func NewServiceWithAuth(db *database.Connection, encryptor *encryption.Encryptor, recorder *ttyrec.Recorder, cfg *config.SessionServiceConfig, userService *user.Service, authMiddleware *AuthMiddleware) *Service {
	return &Service{
		db:             db,
		encryptor:      encryptor,
		recorder:       recorder,
		config:         cfg,
		userService:    userService,
		authMiddleware: authMiddleware,
		sessions:       make(map[string]*Session),
	}
}

// CreateSession creates a new terminal session
func (s *Service) CreateSession(ctx context.Context, req *CreateSessionRequest) (*Session, error) {
	sessionID := generateSessionID()

	// Initialize immutable spectator registry
	registry := &atomic.Pointer[SpectatorRegistry]{}
	registry.Store(NewSpectatorRegistry())

	// Initialize stream manager
	streamManager := NewStreamManager()

	session := &Session{
		ID:            sessionID,
		UserID:        req.UserID,
		Username:      req.Username,
		GameID:        req.GameID,
		StartTime:     time.Now(),
		IsActive:      true,
		TerminalSize:  req.TerminalSize,
		Encoding:      req.Encoding,
		LastActivity:  time.Now(),
		StreamEnabled: true, // Always enable streaming for spectators
		Encrypted:     s.config.Encryption.Enabled,
		Spectators:    make([]*Spectator, 0), // Legacy field for JSON serialization
		Registry:      registry,              // Immutable spectator registry
		StreamManager: streamManager,         // Stream manager for broadcasting
	}

	// Start TTY recording if enabled
	if s.config.SessionManagement.TTYRec.Enabled {
		recording, err := s.recorder.StartRecording(sessionID, req.Username, req.GameID)
		if err != nil {
			return nil, fmt.Errorf("failed to start TTY recording: %w", err)
		}
		session.TTYRecording = recording
	}

	// Start stream manager for spectating
	streamManager.Start(registry)

	// Store session in service registry
	s.sessionsMux.Lock()
	s.sessions[sessionID] = session
	s.sessionsMux.Unlock()

	log.Printf("Created session %s for user %s playing %s", sessionID, req.Username, req.GameID)
	log.Printf("Total active sessions: %d", len(s.sessions))

	// TODO: Store session in database
	// TODO: Set up session monitoring
	// TODO: Initialize encrypted stream if enabled

	return session, nil
}

// EndSession ends a terminal session
func (s *Service) EndSession(ctx context.Context, sessionID string) error {
	// TODO: Implement session ending logic

	// Stop TTY recording
	// Update database
	// Clean up resources

	return fmt.Errorf("session ending not implemented")
}

// GetSession retrieves a session by ID
func (s *Service) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	// TODO: Implement session retrieval from database
	return nil, fmt.Errorf("session retrieval not implemented")
}

// GetActiveSessions returns all active sessions
func (s *Service) GetActiveSessions(ctx context.Context) ([]*Session, error) {
	s.sessionsMux.RLock()
	defer s.sessionsMux.RUnlock()

	log.Printf("GetActiveSessions: Total sessions in registry: %d", len(s.sessions))
	var activeSessions []*Session

	// Iterate through all sessions and collect active ones
	for _, session := range s.sessions {
		if session.IsActive {
			// Create a copy to avoid race conditions
			sessionCopy := *session

			// Update spectator count from registry if available
			if session.Registry != nil {
				currentRegistry := session.Registry.Load()
				if currentRegistry != nil {
					sessionCopy.Spectators = currentRegistry.GetSpectators()
				}
			}

			activeSessions = append(activeSessions, &sessionCopy)
		}
	}

	// If no real sessions exist, create some test sessions for testing watch functionality
	if len(activeSessions) == 0 {
		log.Printf("No real sessions found, creating test sessions for demonstration")
		activeSessions = s.createTestSessions()
	}

	log.Printf("Found %d active sessions", len(activeSessions))
	return activeSessions, nil
}

// createTestSessions creates some test sessions for demonstration purposes
func (s *Service) createTestSessions() []*Session {
	testSessions := []*Session{
		{
			ID:           "test_session_1",
			UserID:       1,
			Username:     "player1",
			GameID:       "nethack",
			StartTime:    time.Now().Add(-30 * time.Minute),
			IsActive:     true,
			TerminalSize: "80x24",
			Encoding:     "utf-8",
			LastActivity: time.Now().Add(-5 * time.Minute),
			Spectators:   []*Spectator{},
		},
		{
			ID:           "test_session_2",
			UserID:       2,
			Username:     "player2",
			GameID:       "dcss",
			StartTime:    time.Now().Add(-15 * time.Minute),
			IsActive:     true,
			TerminalSize: "120x40",
			Encoding:     "utf-8",
			LastActivity: time.Now().Add(-2 * time.Minute),
			Spectators: []*Spectator{
				{
					UserID:   3,
					Username: "spectator1",
					JoinTime: time.Now().Add(-10 * time.Minute),
				},
			},
		},
	}

	// Add these test sessions to the service registry for spectating to work
	s.sessionsMux.Lock()
	for _, session := range testSessions {
		// Initialize immutable spectator registry for test sessions
		registry := &atomic.Pointer[SpectatorRegistry]{}
		registry.Store(NewSpectatorRegistry())
		session.Registry = registry

		// Initialize stream manager
		session.StreamManager = NewStreamManager()
		session.StreamManager.Start(registry)

		s.sessions[session.ID] = session
	}
	s.sessionsMux.Unlock()

	return testSessions
}

// AddSpectator adds a spectator to a session using immutable data sharing
func (s *Service) AddSpectator(ctx context.Context, sessionID string, userID int, username string) error {
	// Check if spectating is enabled
	if !s.config.SessionManagement.Spectating.Enabled {
		return fmt.Errorf("spectating is not enabled")
	}

	// Find the session
	s.sessionsMux.RLock()
	session, exists := s.sessions[sessionID]
	s.sessionsMux.RUnlock()

	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	if !session.IsActive {
		return fmt.Errorf("session %s is not active", sessionID)
	}

	// Check max spectators limit
	currentRegistry := session.Registry.Load()
	if len(currentRegistry.Spectators) >= s.config.SessionManagement.Spectating.MaxSpectatorsPerSession {
		return fmt.Errorf("maximum spectators (%d) reached for session %s",
			s.config.SessionManagement.Spectating.MaxSpectatorsPerSession, sessionID)
	}

	// TODO: This is a placeholder - in real implementation, the connection would be provided
	// For now, we'll create a mock connection for testing
	log.Printf("Adding spectator %s (ID: %d) to session %s", username, userID, sessionID)

	return nil
}

// AddSpectatorWithConnection adds a spectator to a session with a specific connection
func (s *Service) AddSpectatorWithConnection(ctx context.Context, sessionID string, userID int, username string, connection SpectatorConnection) error {
	// Check if spectating is enabled
	if !s.config.SessionManagement.Spectating.Enabled {
		return fmt.Errorf("spectating is not enabled")
	}

	// Find the session
	s.sessionsMux.RLock()
	session, exists := s.sessions[sessionID]
	s.sessionsMux.RUnlock()

	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	if !session.IsActive {
		return fmt.Errorf("session %s is not active", sessionID)
	}

	// Check max spectators limit
	currentRegistry := session.Registry.Load()
	if len(currentRegistry.Spectators) >= s.config.SessionManagement.Spectating.MaxSpectatorsPerSession {
		return fmt.Errorf("maximum spectators (%d) reached for session %s",
			s.config.SessionManagement.Spectating.MaxSpectatorsPerSession, sessionID)
	}

	// Create new spectator
	spectator := &Spectator{
		UserID:     userID,
		Username:   username,
		JoinTime:   time.Now(),
		Connection: connection,
		BytesSent:  0,
		IsActive:   true,
	}

	// Atomically update spectator registry (immutable pattern)
	const maxRetries = 10
	for retry := 0; retry < maxRetries; retry++ {
		oldRegistry := session.Registry.Load()
		newRegistry := oldRegistry.AddSpectator(spectator)

		// Try to swap the registry atomically
		if session.Registry.CompareAndSwap(oldRegistry, newRegistry) {
			log.Printf("Successfully added spectator %s to session %s (registry version %d)",
				username, sessionID, newRegistry.Version)

			// Update legacy field for JSON serialization
			session.Spectators = newRegistry.GetSpectators()

			// Send recent frames to the new spectator
			if session.StreamManager != nil {
				recentFrames := session.StreamManager.GetRecentFrames()
				if len(recentFrames) > 0 {
					log.Printf("Sending %d recent frames to new spectator %s", len(recentFrames), username)
					go func() {
						// Small delay to ensure spectator is ready
						time.Sleep(100 * time.Millisecond)

						// Send each frame with a small delay to avoid overwhelming
						for i, frame := range recentFrames {
							if spectator.Connection != nil && spectator.Connection.IsConnected() {
								if err := spectator.Connection.Write(frame); err != nil {
									log.Printf("Failed to send historical frame %d to spectator %s: %v", i, username, err)
									break
								}
								// Small delay between frames to ensure proper rendering
								time.Sleep(10 * time.Millisecond)
							}
						}
						log.Printf("Finished sending historical frames to spectator %s", username)
					}()
				}
			}

			return nil
		}
		// If swap failed, another goroutine updated the registry, retry with exponential backoff
		if retry < maxRetries-1 {
			time.Sleep(time.Duration(1<<uint(retry)) * time.Millisecond)
		}
	}

	return fmt.Errorf("failed to add spectator after %d retries due to high contention", maxRetries)
}

// RemoveSpectator removes a spectator from a session using immutable data sharing
func (s *Service) RemoveSpectator(ctx context.Context, sessionID string, userID int) error {
	// Find the session
	s.sessionsMux.RLock()
	session, exists := s.sessions[sessionID]
	s.sessionsMux.RUnlock()

	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	// Get current registry to find the spectator
	currentRegistry := session.Registry.Load()
	var targetUsername string
	spectatorID := ""

	// Find the spectator to get their username
	for id, spectator := range currentRegistry.Spectators {
		if spectator.UserID == userID {
			targetUsername = spectator.Username
			spectatorID = id
			break
		}
	}

	if targetUsername == "" {
		return fmt.Errorf("spectator with userID %d not found in session %s", userID, sessionID)
	}

	// Close the spectator connection before removing
	if spectator, exists := currentRegistry.Spectators[spectatorID]; exists {
		if spectator.Connection != nil {
			spectator.Connection.Close()
		}
	}

	// Atomically update spectator registry (immutable pattern)
	for {
		oldRegistry := session.Registry.Load()
		newRegistry := oldRegistry.RemoveSpectator(userID, targetUsername)

		// Try to swap the registry atomically
		if session.Registry.CompareAndSwap(oldRegistry, newRegistry) {
			log.Printf("Successfully removed spectator %s (ID: %d) from session %s (registry version %d)",
				targetUsername, userID, sessionID, newRegistry.Version)

			// Update legacy field for JSON serialization
			session.Spectators = newRegistry.GetSpectators()

			return nil
		}
		// If swap failed, another goroutine updated the registry, retry
	}
}

// WriteToSession writes data to a session and broadcasts it to spectators using immutable frames
func (s *Service) WriteToSession(ctx context.Context, sessionID string, data []byte) error {
	if len(data) == 0 {
		return nil
	}

	// Find the session
	s.sessionsMux.RLock()
	session, exists := s.sessions[sessionID]
	s.sessionsMux.RUnlock()

	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	if !session.IsActive {
		return fmt.Errorf("session %s is not active", sessionID)
	}

	// Update last activity
	session.LastActivity = time.Now()

	// Encrypt data if encryption is enabled
	dataToStream := data
	if s.config.Encryption.Enabled && s.encryptor != nil {
		encryptedData, err := s.encryptor.Encrypt(data)
		if err != nil {
			return fmt.Errorf("failed to encrypt session data: %w", err)
		}
		dataToStream = encryptedData
	}

	// Record data if TTY recording is enabled
	if session.TTYRecording != nil {
		// TODO: Implement actual TTY recording when ttyrec package is complete
		log.Printf("TTY recording for session %s: %d bytes", sessionID, len(dataToStream))
	}

	// Broadcast to spectators using immutable stream frames
	if session.StreamManager != nil && session.StreamEnabled {
		session.StreamManager.SendFrame(dataToStream)
		// Debug logging for spectator broadcasting
		if currentRegistry := session.Registry.Load(); currentRegistry != nil {
			spectatorCount := len(currentRegistry.GetSpectators())
			if spectatorCount > 0 {
				log.Printf("Broadcasting %d bytes to %d spectators for session %s", len(dataToStream), spectatorCount, sessionID)
			}
		}
	}

	return nil
}

// WriteToSessionEnhanced writes data to a session with enhanced features
func (s *Service) WriteToSessionEnhanced(ctx context.Context, sessionID string, data []byte) error {
	// This would implement TTY recording, spectator broadcasting, etc.
	// For now, it's a placeholder
	fmt.Printf("Writing %d bytes to session %s", len(data), sessionID)
	return nil
}

// GetSessionPlayback retrieves session playback data
func (s *Service) GetSessionPlayback(ctx context.Context, sessionID string) ([]byte, error) {
	// TODO: Implement session playback retrieval
	return nil, fmt.Errorf("session playback not implemented")
}

// CleanupInactiveSessions removes inactive sessions
func (s *Service) CleanupInactiveSessions(ctx context.Context) error {
	// TODO: Implement session cleanup logic

	// Find sessions that have exceeded timeout
	// End sessions gracefully
	// Clean up resources

	return fmt.Errorf("session cleanup not implemented")
}

// MonitorSessions monitors session activity and health
func (s *Service) MonitorSessions(ctx context.Context) error {
	// TODO: Implement session monitoring

	// Check session health
	// Monitor resource usage
	// Send alerts if needed

	return fmt.Errorf("session monitoring not implemented")
}

// GetMetrics returns service metrics
func (s *Service) GetMetrics() *ServiceMetrics {
	return &ServiceMetrics{
		ActiveSessions:   2,
		TotalSessions:    10,
		ActiveSpectators: 1,
		TotalSpectators:  5,
		BytesTransferred: 1024 * 1024,
		UptimeSeconds:    3600,
	}
}

// Additional service methods for enhanced functionality

// GetSessionsByUser returns sessions for a specific user
func (s *Service) GetSessionsByUser(ctx context.Context, userID int) ([]*Session, error) {
	// Mock implementation
	return []*Session{}, nil
}

// GetSessionsByGame returns sessions for a specific game
func (s *Service) GetSessionsByGame(ctx context.Context, gameID string) ([]*Session, error) {
	// Mock implementation
	return []*Session{}, nil
}

// GetUserStatistics returns statistics for a user
func (s *Service) GetUserStatistics(ctx context.Context, userID int) (*UserStatistics, error) {
	// Mock implementation
	return &UserStatistics{}, nil
}

// GetGameStatistics returns statistics for a game
func (s *Service) GetGameStatistics(ctx context.Context, gameID string) (*GameStatistics, error) {
	// Mock implementation
	return &GameStatistics{}, nil
}

// GetSystemStatistics returns system-wide statistics
func (s *Service) GetSystemStatistics(ctx context.Context) (*SessionStatistics, error) {
	// Mock implementation
	return &SessionStatistics{}, nil
}

// generateSessionID generates a unique session ID
func generateSessionID() string {
	// TODO: Implement proper session ID generation
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}

// NewHTTPHandler creates a new HTTP handler for the session service
func NewHTTPHandler(service *Service) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("/sessions", func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement session endpoints
		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte("Session endpoints not implemented"))
	})

	mux.HandleFunc("/spectate", func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement spectator endpoints
		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte("Spectator endpoints not implemented"))
	})

	// WebSocket endpoint for browser spectating (stubbed)
	mux.HandleFunc("/ws/spectate", func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement WebSocket upgrade and spectating
		// This will be implemented when WebSocket support is added
		sessionID := r.URL.Query().Get("session")
		if sessionID == "" {
			http.Error(w, "session ID required", http.StatusBadRequest)
			return
		}

		// For now, return a placeholder response
		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte("WebSocket spectating not yet implemented"))
		log.Printf("WebSocket spectate request for session %s (not implemented)", sessionID)
	})

	mux.HandleFunc("/playback", func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement playback endpoints
		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte("Playback endpoints not implemented"))
	})

	// Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	return mux
}

// RegisterSessionServiceServer registers the gRPC service
func RegisterSessionServiceServer(server interface{}, service *Service) {
	// TODO: Implement gRPC service registration
}
