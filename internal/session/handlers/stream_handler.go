package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/dungeongate/internal/session/pools"
	"github.com/dungeongate/internal/session/resources"
)

// StreamHandler handles I/O streaming and spectating with resource tracking
type StreamHandler struct {
	resourceTracker *resources.ResourceTracker
	workerPool      *pools.WorkerPool
	logger          *slog.Logger

	// Metrics
	streamingSessions    *resources.GaugeMetric
	dataTransferred      *resources.CounterMetric
	spectatorConnections *resources.GaugeMetric
	streamingErrors      *resources.CounterMetric
	bandwidthUsage       *resources.HistogramMetric
}

// NewStreamHandler creates a new stream handler
func NewStreamHandler(
	resourceTracker *resources.ResourceTracker,
	workerPool *pools.WorkerPool,
	metricsRegistry *resources.MetricsRegistry,
	logger *slog.Logger,
) *StreamHandler {
	sh := &StreamHandler{
		resourceTracker: resourceTracker,
		workerPool:      workerPool,
		logger:          logger,
	}

	sh.initializeMetrics(metricsRegistry)
	return sh
}

// initializeMetrics sets up metrics for the stream handler
func (sh *StreamHandler) initializeMetrics(registry *resources.MetricsRegistry) {
	sh.streamingSessions = registry.RegisterGauge(
		"session_streaming_sessions_active",
		"Number of active streaming sessions",
		map[string]string{"handler": "stream"})

	sh.dataTransferred = registry.RegisterCounter(
		"session_streaming_bytes_total",
		"Total bytes transferred in streaming sessions",
		map[string]string{"handler": "stream", "direction": "unknown"})

	sh.spectatorConnections = registry.RegisterGauge(
		"session_spectator_connections_active",
		"Number of active spectator connections",
		map[string]string{"handler": "stream"})

	sh.streamingErrors = registry.RegisterCounter(
		"session_streaming_errors_total",
		"Total number of streaming errors",
		map[string]string{"handler": "stream"})

	sh.bandwidthUsage = registry.RegisterHistogram(
		"session_streaming_bandwidth_bytes_per_second",
		"Bandwidth usage for streaming sessions",
		[]float64{1000, 5000, 10000, 50000, 100000, 500000, 1000000, 5000000, 10000000},
		map[string]string{"handler": "stream"})
}

// HandleGameIO handles I/O streaming with resource tracking
func (sh *StreamHandler) HandleGameIO(ctx context.Context, conn *pools.Connection, sessionID string) error {
	startTime := time.Now()
	sh.streamingSessions.Inc()
	defer sh.streamingSessions.Dec()

	sh.logger.Info("Starting game I/O streaming",
		"session_id", sessionID,
		"connection_id", conn.ID,
		"user_id", conn.UserID)

	// Track this streaming operation
	if sh.resourceTracker != nil {
		// Note: These methods may need to be implemented in the resource tracker
		sh.logger.Debug("Would track streaming operation start",
			"session_id", sessionID,
			"connection_id", conn.ID)
	}

	defer func() {
		duration := time.Since(startTime)
		sh.logger.Info("Game I/O streaming ended",
			"session_id", sessionID,
			"connection_id", conn.ID,
			"duration", duration)

		if sh.resourceTracker != nil {
			sh.logger.Debug("Would track streaming operation end",
				"session_id", sessionID,
				"connection_id", conn.ID,
				"duration", duration)
		}
	}()

	// Create streaming work item
	work := &pools.WorkItem{
		Type:       pools.WorkTypeStreamManagement,
		Connection: conn,
		Handler:    sh.handleStreamingWork,
		Context:    ctx,
		Priority:   pools.PriorityHigh,
		QueuedAt:   time.Now(),
		Data:       sessionID,
	}

	// Submit to worker pool
	if err := sh.workerPool.Submit(work); err != nil {
		sh.logger.Error("Failed to submit streaming work",
			"error", err,
			"session_id", sessionID,
			"connection_id", conn.ID)
		sh.streamingErrors.Inc()
		return fmt.Errorf("failed to submit streaming work: %w", err)
	}

	return nil
}

// HandleSpectating handles spectator session management
func (sh *StreamHandler) HandleSpectating(ctx context.Context, conn *pools.Connection, sessionID string) error {
	startTime := time.Now()
	sh.spectatorConnections.Inc()
	defer sh.spectatorConnections.Dec()

	sh.logger.Info("Starting spectator session",
		"session_id", sessionID,
		"connection_id", conn.ID,
		"spectator_user", conn.UserID)

	defer func() {
		duration := time.Since(startTime)
		sh.logger.Info("Spectator session ended",
			"session_id", sessionID,
			"connection_id", conn.ID,
			"duration", duration)
	}()

	// For now, spectating is not implemented
	conn.SSHChannel.Write([]byte("Spectating functionality not yet implemented.\r\n"))
	conn.SSHChannel.Write([]byte("Press any key to return to menu...\r\n"))

	// Wait for any key press
	buffer := make([]byte, 1)
	_, err := conn.SSHChannel.Read(buffer)
	if err != nil {
		sh.logger.Debug("Error reading spectator input", "error", err)
	}

	return nil
}

// TrackDataTransfer tracks data transfer for bandwidth monitoring
func (sh *StreamHandler) TrackDataTransfer(connectionID string, bytesSent, bytesReceived int64) {
	sh.logger.Debug("Tracking data transfer",
		"connection_id", connectionID,
		"bytes_sent", bytesSent,
		"bytes_received", bytesReceived)

	// Update metrics
	if bytesSent > 0 {
		sh.dataTransferred.Add(float64(bytesSent))
	}
	if bytesReceived > 0 {
		sh.dataTransferred.Add(float64(bytesReceived))
	}

	// Calculate bandwidth usage (bytes per second)
	totalBytes := bytesSent + bytesReceived
	if totalBytes > 0 {
		sh.bandwidthUsage.Observe(float64(totalBytes))
	}
}

// handleStreamingWork processes streaming operations using worker pool
func (sh *StreamHandler) handleStreamingWork(ctx context.Context, conn *pools.Connection) error {
	sessionID, ok := conn.Context.Value("work_data").(string)
	if !ok {
		sh.logger.Error("Invalid streaming work data", "connection_id", conn.ID)
		sh.streamingErrors.Inc()
		return fmt.Errorf("invalid streaming work data")
	}

	sh.logger.Info("Processing streaming work",
		"session_id", sessionID,
		"connection_id", conn.ID)

	// This is where the actual streaming logic would be implemented
	// For now, this is a placeholder that simulates streaming work
	startTime := time.Now()

	// Simulate data transfer tracking
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Simulate tracking data transfer
				sh.TrackDataTransfer(conn.ID, 1024, 512) // Example: 1KB sent, 512B received
			}
		}
	}()

	// Wait for context cancellation or completion
	<-ctx.Done()

	duration := time.Since(startTime)
	sh.logger.Info("Streaming work completed",
		"session_id", sessionID,
		"connection_id", conn.ID,
		"duration", duration)

	return ctx.Err()
}

// GetStreamingMetrics returns current streaming metrics
func (sh *StreamHandler) GetStreamingMetrics() map[string]interface{} {
	return map[string]interface{}{
		"active_streaming_sessions": sh.streamingSessions.Get(),
		"active_spectator_connections": sh.spectatorConnections.Get(),
		"total_streaming_errors": sh.streamingErrors.Get(),
	}
}

// IsHealthy checks if the stream handler is healthy
func (sh *StreamHandler) IsHealthy() bool {
	// For now, always return true
	// In a real implementation, this would check various health indicators
	return true
}