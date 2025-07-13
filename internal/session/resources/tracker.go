package resources

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// ConnectionInfo tracks information about a connection
type ConnectionInfo struct {
	ID           string
	UserID       string
	RemoteAddr   string
	StartTime    time.Time
	LastActivity time.Time
	BytesSent    int64
	BytesReceived int64
	State        string
	SessionID    string
}

// ResourceTracker tracks resource usage across connections and sessions
type ResourceTracker struct {
	connections     map[string]*ConnectionInfo
	userConnections map[string][]string // user_id -> connection_ids
	sessionResources map[string]*SessionResourceUsage
	
	totalConnections   int64
	activeConnections  int64
	totalBytesTransferred int64
	
	logger           *slog.Logger
	shutdownChan     chan struct{}
	
	mu               sync.RWMutex
	wg               sync.WaitGroup
}

// SessionResourceUsage tracks resource usage for a game session
type SessionResourceUsage struct {
	SessionID       string
	UserID          string
	ConnectionID    string
	GameID          string
	StartTime       time.Time
	LastActivity    time.Time
	CPUUsage        float64
	MemoryUsage     int64
	DiskUsage       int64
	NetworkUsage    int64
	PTYCount        int
	ProcessCount    int
	State           string
}

// TrackerMetrics provides metrics about resource tracking
type TrackerMetrics struct {
	TotalConnections      int64
	ActiveConnections     int64
	TotalBytesTransferred int64
	ActiveSessions        int64
	UniqueUsers           int64
	AverageSessionDuration time.Duration
	ConnectionsByState    map[string]int64
	TopUsers             []UserResourceSummary
}

// UserResourceSummary summarizes resource usage for a user
type UserResourceSummary struct {
	UserID          string
	ConnectionCount int
	TotalBytes      int64
	SessionCount    int
	LastActivity    time.Time
}

// NewResourceTracker creates a new resource tracker
func NewResourceTracker(logger *slog.Logger) *ResourceTracker {
	return &ResourceTracker{
		connections:      make(map[string]*ConnectionInfo),
		userConnections:  make(map[string][]string),
		sessionResources: make(map[string]*SessionResourceUsage),
		logger:           logger,
		shutdownChan:     make(chan struct{}),
	}
}

// Start starts the resource tracker
func (rt *ResourceTracker) Start(ctx context.Context) error {
	rt.logger.Info("Starting resource tracker")

	// Start monitoring routine
	rt.wg.Add(1)
	go rt.monitoringRoutine(ctx)

	// Start metrics collection
	rt.wg.Add(1)
	go rt.metricsRoutine(ctx)

	return nil
}

// Stop stops the resource tracker gracefully
func (rt *ResourceTracker) Stop(ctx context.Context) error {
	rt.logger.Info("Stopping resource tracker")

	// Signal shutdown
	close(rt.shutdownChan)

	// Wait for goroutines to finish
	done := make(chan struct{})
	go func() {
		rt.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		rt.logger.Info("Resource tracker stopped gracefully")
	case <-ctx.Done():
		rt.logger.Warn("Resource tracker stop timeout exceeded")
	}

	return nil
}

// TrackConnection starts tracking a new connection
func (rt *ResourceTracker) TrackConnection(connectionID, userID, remoteAddr string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	connInfo := &ConnectionInfo{
		ID:           connectionID,
		UserID:       userID,
		RemoteAddr:   remoteAddr,
		StartTime:    time.Now(),
		LastActivity: time.Now(),
		State:        "connected",
	}

	rt.connections[connectionID] = connInfo
	
	// Add to user connections
	rt.userConnections[userID] = append(rt.userConnections[userID], connectionID)

	atomic.AddInt64(&rt.totalConnections, 1)
	atomic.AddInt64(&rt.activeConnections, 1)

	rt.logger.Info("Started tracking connection",
		"connection_id", connectionID,
		"user_id", userID,
		"remote_addr", remoteAddr,
		"active_connections", atomic.LoadInt64(&rt.activeConnections))
}

// UntrackConnection stops tracking a connection
func (rt *ResourceTracker) UntrackConnection(connectionID string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	connInfo, exists := rt.connections[connectionID]
	if !exists {
		rt.logger.Warn("Attempted to untrack non-existent connection", "connection_id", connectionID)
		return
	}

	// Remove from user connections
	userConnections := rt.userConnections[connInfo.UserID]
	for i, id := range userConnections {
		if id == connectionID {
			rt.userConnections[connInfo.UserID] = append(userConnections[:i], userConnections[i+1:]...)
			break
		}
	}

	// Clean up empty user entries
	if len(rt.userConnections[connInfo.UserID]) == 0 {
		delete(rt.userConnections, connInfo.UserID)
	}

	// Calculate session duration
	duration := time.Since(connInfo.StartTime)

	delete(rt.connections, connectionID)
	atomic.AddInt64(&rt.activeConnections, -1)

	rt.logger.Info("Stopped tracking connection",
		"connection_id", connectionID,
		"user_id", connInfo.UserID,
		"duration", duration,
		"bytes_sent", connInfo.BytesSent,
		"bytes_received", connInfo.BytesReceived,
		"active_connections", atomic.LoadInt64(&rt.activeConnections))
}

// UpdateConnectionActivity updates the last activity time for a connection
func (rt *ResourceTracker) UpdateConnectionActivity(connectionID string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if connInfo, exists := rt.connections[connectionID]; exists {
		connInfo.LastActivity = time.Now()
	}
}

// UpdateConnectionState updates the state of a connection
func (rt *ResourceTracker) UpdateConnectionState(connectionID, state string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if connInfo, exists := rt.connections[connectionID]; exists {
		oldState := connInfo.State
		connInfo.State = state
		connInfo.LastActivity = time.Now()

		rt.logger.Debug("Connection state changed",
			"connection_id", connectionID,
			"old_state", oldState,
			"new_state", state)
	}
}

// TrackDataTransfer tracks data transfer for a connection
func (rt *ResourceTracker) TrackDataTransfer(connectionID string, bytesSent, bytesReceived int64) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if connInfo, exists := rt.connections[connectionID]; exists {
		connInfo.BytesSent += bytesSent
		connInfo.BytesReceived += bytesReceived
		connInfo.LastActivity = time.Now()

		atomic.AddInt64(&rt.totalBytesTransferred, bytesSent+bytesReceived)

		rt.logger.Debug("Tracked data transfer",
			"connection_id", connectionID,
			"bytes_sent", bytesSent,
			"bytes_received", bytesReceived,
			"total_sent", connInfo.BytesSent,
			"total_received", connInfo.BytesReceived)
	}
}

// TrackSession starts tracking a game session
func (rt *ResourceTracker) TrackSession(sessionID, userID, connectionID, gameID string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	sessionUsage := &SessionResourceUsage{
		SessionID:    sessionID,
		UserID:       userID,
		ConnectionID: connectionID,
		GameID:       gameID,
		StartTime:    time.Now(),
		LastActivity: time.Now(),
		State:        "starting",
	}

	rt.sessionResources[sessionID] = sessionUsage

	// Link session to connection
	if connInfo, exists := rt.connections[connectionID]; exists {
		connInfo.SessionID = sessionID
	}

	rt.logger.Info("Started tracking session",
		"session_id", sessionID,
		"user_id", userID,
		"connection_id", connectionID,
		"game_id", gameID)
}

// UntrackSession stops tracking a game session
func (rt *ResourceTracker) UntrackSession(sessionID string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	sessionUsage, exists := rt.sessionResources[sessionID]
	if !exists {
		rt.logger.Warn("Attempted to untrack non-existent session", "session_id", sessionID)
		return
	}

	// Calculate session duration
	duration := time.Since(sessionUsage.StartTime)

	// Unlink from connection
	if connInfo, exists := rt.connections[sessionUsage.ConnectionID]; exists {
		connInfo.SessionID = ""
	}

	delete(rt.sessionResources, sessionID)

	rt.logger.Info("Stopped tracking session",
		"session_id", sessionID,
		"user_id", sessionUsage.UserID,
		"game_id", sessionUsage.GameID,
		"duration", duration,
		"cpu_usage", sessionUsage.CPUUsage,
		"memory_usage", sessionUsage.MemoryUsage,
		"disk_usage", sessionUsage.DiskUsage,
		"network_usage", sessionUsage.NetworkUsage)
}

// UpdateSessionResources updates resource usage for a session
func (rt *ResourceTracker) UpdateSessionResources(sessionID string, cpuUsage float64, memoryUsage, diskUsage, networkUsage int64) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if sessionUsage, exists := rt.sessionResources[sessionID]; exists {
		sessionUsage.CPUUsage = cpuUsage
		sessionUsage.MemoryUsage = memoryUsage
		sessionUsage.DiskUsage = diskUsage
		sessionUsage.NetworkUsage = networkUsage
		sessionUsage.LastActivity = time.Now()

		rt.logger.Debug("Updated session resources",
			"session_id", sessionID,
			"cpu_usage", cpuUsage,
			"memory_usage", memoryUsage,
			"disk_usage", diskUsage,
			"network_usage", networkUsage)
	}
}

// UpdateSessionState updates the state of a session
func (rt *ResourceTracker) UpdateSessionState(sessionID, state string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if sessionUsage, exists := rt.sessionResources[sessionID]; exists {
		oldState := sessionUsage.State
		sessionUsage.State = state
		sessionUsage.LastActivity = time.Now()

		rt.logger.Debug("Session state changed",
			"session_id", sessionID,
			"old_state", oldState,
			"new_state", state)
	}
}

// GetConnectionInfo returns information about a connection
func (rt *ResourceTracker) GetConnectionInfo(connectionID string) (*ConnectionInfo, bool) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	connInfo, exists := rt.connections[connectionID]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid data races
	info := *connInfo
	return &info, true
}

// GetUserConnections returns all connections for a user
func (rt *ResourceTracker) GetUserConnections(userID string) []*ConnectionInfo {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	connectionIDs := rt.userConnections[userID]
	connections := make([]*ConnectionInfo, 0, len(connectionIDs))

	for _, connectionID := range connectionIDs {
		if connInfo, exists := rt.connections[connectionID]; exists {
			// Return a copy to avoid data races
			info := *connInfo
			connections = append(connections, &info)
		}
	}

	return connections
}

// GetSessionInfo returns information about a session
func (rt *ResourceTracker) GetSessionInfo(sessionID string) (*SessionResourceUsage, bool) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	sessionUsage, exists := rt.sessionResources[sessionID]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid data races
	usage := *sessionUsage
	return &usage, true
}

// GetMetrics returns current resource tracking metrics
func (rt *ResourceTracker) GetMetrics() TrackerMetrics {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	metrics := TrackerMetrics{
		TotalConnections:      atomic.LoadInt64(&rt.totalConnections),
		ActiveConnections:     atomic.LoadInt64(&rt.activeConnections),
		TotalBytesTransferred: atomic.LoadInt64(&rt.totalBytesTransferred),
		ActiveSessions:        int64(len(rt.sessionResources)),
		UniqueUsers:           int64(len(rt.userConnections)),
		ConnectionsByState:    make(map[string]int64),
		TopUsers:             make([]UserResourceSummary, 0),
	}

	// Calculate connections by state
	var totalDuration time.Duration
	var sessionCount int

	for _, connInfo := range rt.connections {
		metrics.ConnectionsByState[connInfo.State]++
		if connInfo.SessionID != "" {
			sessionCount++
			totalDuration += time.Since(connInfo.StartTime)
		}
	}

	// Calculate average session duration
	if sessionCount > 0 {
		metrics.AverageSessionDuration = totalDuration / time.Duration(sessionCount)
	}

	// Generate top users summary
	for userID, connectionIDs := range rt.userConnections {
		var totalBytes int64
		var lastActivity time.Time

		for _, connectionID := range connectionIDs {
			if connInfo, exists := rt.connections[connectionID]; exists {
				totalBytes += connInfo.BytesSent + connInfo.BytesReceived
				if connInfo.LastActivity.After(lastActivity) {
					lastActivity = connInfo.LastActivity
				}
			}
		}

		summary := UserResourceSummary{
			UserID:          userID,
			ConnectionCount: len(connectionIDs),
			TotalBytes:      totalBytes,
			LastActivity:    lastActivity,
		}

		// Count sessions for this user
		for _, sessionUsage := range rt.sessionResources {
			if sessionUsage.UserID == userID {
				summary.SessionCount++
			}
		}

		metrics.TopUsers = append(metrics.TopUsers, summary)
	}

	return metrics
}

// GetIdleConnections returns connections that have been idle for longer than the specified duration
func (rt *ResourceTracker) GetIdleConnections(idleDuration time.Duration) []string {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	var idleConnections []string
	now := time.Now()

	for connectionID, connInfo := range rt.connections {
		if now.Sub(connInfo.LastActivity) > idleDuration {
			idleConnections = append(idleConnections, connectionID)
		}
	}

	return idleConnections
}

// GetLongRunningSessions returns sessions that have been running longer than the specified duration
func (rt *ResourceTracker) GetLongRunningSessions(maxDuration time.Duration) []string {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	var longSessions []string
	now := time.Now()

	for sessionID, sessionUsage := range rt.sessionResources {
		if now.Sub(sessionUsage.StartTime) > maxDuration {
			longSessions = append(longSessions, sessionID)
		}
	}

	return longSessions
}

// Monitoring routines

// monitoringRoutine performs periodic monitoring and cleanup
func (rt *ResourceTracker) monitoringRoutine(ctx context.Context) {
	defer rt.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	rt.logger.Debug("Resource tracker monitoring routine started")

	for {
		select {
		case <-ticker.C:
			rt.performMonitoring()
		case <-ctx.Done():
			rt.logger.Debug("Resource tracker monitoring routine stopped due to context cancellation")
			return
		case <-rt.shutdownChan:
			rt.logger.Debug("Resource tracker monitoring routine stopped")
			return
		}
	}
}

// performMonitoring performs monitoring checks
func (rt *ResourceTracker) performMonitoring() {
	// Check for idle connections
	idleConnections := rt.GetIdleConnections(30 * time.Minute)
	if len(idleConnections) > 0 {
		rt.logger.Info("Found idle connections",
			"count", len(idleConnections),
			"idle_threshold", "30m")
	}

	// Check for long-running sessions
	longSessions := rt.GetLongRunningSessions(4 * time.Hour)
	if len(longSessions) > 0 {
		rt.logger.Info("Found long-running sessions",
			"count", len(longSessions),
			"duration_threshold", "4h")
	}

	// Log resource warnings if needed
	rt.checkResourceWarnings()
}

// checkResourceWarnings checks for resource usage warnings
func (rt *ResourceTracker) checkResourceWarnings() {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	activeConnections := atomic.LoadInt64(&rt.activeConnections)
	
	// Warning thresholds
	const (
		connectionWarningThreshold = 800  // 80% of 1000 max connections
		sessionWarningThreshold    = 400  // 80% of 500 max sessions
	)

	if activeConnections > connectionWarningThreshold {
		rt.logger.Warn("High connection count",
			"active_connections", activeConnections,
			"threshold", connectionWarningThreshold)
	}

	if len(rt.sessionResources) > sessionWarningThreshold {
		rt.logger.Warn("High session count",
			"active_sessions", len(rt.sessionResources),
			"threshold", sessionWarningThreshold)
	}
}

// metricsRoutine collects and logs resource tracking metrics
func (rt *ResourceTracker) metricsRoutine(ctx context.Context) {
	defer rt.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	rt.logger.Debug("Resource tracker metrics routine started")

	for {
		select {
		case <-ticker.C:
			rt.logMetrics()
		case <-ctx.Done():
			rt.logger.Debug("Resource tracker metrics routine stopped due to context cancellation")
			return
		case <-rt.shutdownChan:
			rt.logger.Debug("Resource tracker metrics routine stopped")
			return
		}
	}
}

// logMetrics logs current resource tracking metrics
func (rt *ResourceTracker) logMetrics() {
	metrics := rt.GetMetrics()

	rt.logger.Info("Resource tracker metrics",
		"total_connections", metrics.TotalConnections,
		"active_connections", metrics.ActiveConnections,
		"active_sessions", metrics.ActiveSessions,
		"unique_users", metrics.UniqueUsers,
		"total_bytes_transferred", metrics.TotalBytesTransferred,
		"average_session_duration", metrics.AverageSessionDuration.String())

	// Log connection states
	for state, count := range metrics.ConnectionsByState {
		if count > 0 {
			rt.logger.Debug("Connection state count",
				"state", state,
				"count", count)
		}
	}
}