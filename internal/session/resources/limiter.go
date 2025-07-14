package resources

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ResourceType defines the type of resource being limited
type ResourceType int

const (
	ResourceTypeConnections ResourceType = iota
	ResourceTypePTYs
	ResourceTypeFileDescriptors
	ResourceTypeMemory
	ResourceTypeBandwidth
	ResourceTypeCPU
	ResourceTypeWorkItems
)

// Limit defines resource limits
type Limit struct {
	Type        ResourceType
	MaxValue    int64
	CurrentUsed int64
	ResetPeriod time.Duration
	LastReset   time.Time
	BurstLimit  int64 // Allow temporary burst above limit
}

// ResourceUsage tracks resource usage for a user/connection
type ResourceUsage struct {
	UserID      string
	ConnectionID string
	Usage       map[ResourceType]*UsageMetric
	LastUpdated time.Time
	CreatedAt   time.Time
}

// UsageMetric tracks specific resource usage
type UsageMetric struct {
	Current   int64
	Peak      int64
	Average   float64
	Samples   int64
	LastReset time.Time
}

// ResourceQuota defines quotas for a user or connection
type ResourceQuota struct {
	UserID           string
	ConnectionID     string
	MaxConnections   int64
	MaxPTYs         int64
	MaxBandwidth    int64 // bytes/sec
	MaxMemory       int64 // bytes
	MaxCPU          float64 // CPU cores
	MaxWorkItems    int64
	ExpiresAt       time.Time
	Priority        int // Higher priority gets more resources
}

// ResourceMetrics tracks resource limiter performance
type ResourceMetrics struct {
	TotalLimits     int64
	ActiveQuotas    int64
	ViolatedLimits  int64
	ThrottledUsers  int64
	ResourceUtilization map[ResourceType]float64
}

// ResourceLimiter manages resource limits and quotas
type ResourceLimiter struct {
	limits          map[ResourceType]*Limit
	usage           map[string]*ResourceUsage // keyed by user/connection ID
	quotas          map[string]*ResourceQuota // keyed by user ID
	metrics         *ResourceMetrics
	logger          *slog.Logger
	
	defaultQuota    *ResourceQuota
	cleanupInterval time.Duration
	maxIdleTime     time.Duration
	
	shutdownChan    chan struct{}
	
	mu              sync.RWMutex
	wg              sync.WaitGroup
}

// Config holds configuration for the resource limiter
type Config struct {
	DefaultQuota    *ResourceQuota
	CleanupInterval time.Duration
	MaxIdleTime     time.Duration
	Limits          map[ResourceType]*Limit
}

// DefaultConfig returns a default resource limiter configuration
func DefaultConfig() *Config {
	return &Config{
		DefaultQuota: &ResourceQuota{
			MaxConnections: 10,
			MaxPTYs:       5,
			MaxBandwidth:  10 * 1024 * 1024, // 10MB/s
			MaxMemory:     256 * 1024 * 1024, // 256MB
			MaxCPU:        0.5,               // 0.5 CPU cores
			MaxWorkItems:  100,
			ExpiresAt:     time.Now().Add(24 * time.Hour),
			Priority:      1,
		},
		CleanupInterval: 5 * time.Minute,
		MaxIdleTime:     30 * time.Minute,
		Limits: map[ResourceType]*Limit{
			ResourceTypeConnections: {
				Type:        ResourceTypeConnections,
				MaxValue:    1000,
				ResetPeriod: time.Hour,
				BurstLimit:  1200,
			},
			ResourceTypePTYs: {
				Type:        ResourceTypePTYs,
				MaxValue:    500,
				ResetPeriod: time.Hour,
				BurstLimit:  600,
			},
			ResourceTypeMemory: {
				Type:        ResourceTypeMemory,
				MaxValue:    8 * 1024 * 1024 * 1024, // 8GB
				ResetPeriod: time.Hour,
				BurstLimit:  10 * 1024 * 1024 * 1024, // 10GB
			},
			ResourceTypeBandwidth: {
				Type:        ResourceTypeBandwidth,
				MaxValue:    1024 * 1024 * 1024, // 1GB/s
				ResetPeriod: time.Second,
				BurstLimit:  2 * 1024 * 1024 * 1024, // 2GB/s
			},
		},
	}
}

// NewResourceLimiter creates a new resource limiter
func NewResourceLimiter(config *Config, logger *slog.Logger) (*ResourceLimiter, error) {
	if config == nil {
		config = DefaultConfig()
	}

	rl := &ResourceLimiter{
		limits:          config.Limits,
		usage:           make(map[string]*ResourceUsage),
		quotas:          make(map[string]*ResourceQuota),
		metrics:         &ResourceMetrics{
			ResourceUtilization: make(map[ResourceType]float64),
		},
		logger:          logger,
		defaultQuota:    config.DefaultQuota,
		cleanupInterval: config.CleanupInterval,
		maxIdleTime:     config.MaxIdleTime,
		shutdownChan:    make(chan struct{}),
	}

	// Initialize limits
	for resourceType, limit := range rl.limits {
		limit.LastReset = time.Now()
		logger.Info("Resource limit configured",
			"resource_type", resourceType.String(),
			"max_value", limit.MaxValue,
			"burst_limit", limit.BurstLimit,
			"reset_period", limit.ResetPeriod)
	}

	logger.Info("Created resource limiter",
		"limits_count", len(rl.limits),
		"cleanup_interval", config.CleanupInterval,
		"max_idle_time", config.MaxIdleTime)

	return rl, nil
}

// Start starts the resource limiter
func (rl *ResourceLimiter) Start(ctx context.Context) error {
	rl.logger.Info("Starting resource limiter")

	// Start cleanup routine
	rl.wg.Add(1)
	go rl.cleanupRoutine(ctx)

	// Start metrics collection
	rl.wg.Add(1)
	go rl.metricsRoutine(ctx)

	// Start limit reset routine
	rl.wg.Add(1)
	go rl.limitResetRoutine(ctx)

	return nil
}

// Stop stops the resource limiter gracefully
func (rl *ResourceLimiter) Stop(ctx context.Context) error {
	rl.logger.Info("Stopping resource limiter")

	// Signal shutdown
	close(rl.shutdownChan)

	// Wait for goroutines to finish
	done := make(chan struct{})
	go func() {
		rl.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		rl.logger.Info("Resource limiter stopped gracefully")
	case <-ctx.Done():
		rl.logger.Warn("Resource limiter stop timeout exceeded")
	}

	return nil
}

// CanExecute checks if a user can execute an action based on resource limits
func (rl *ResourceLimiter) CanExecute(userID string, action string) bool {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	// Get user quota
	quota, exists := rl.quotas[userID]
	if !exists {
		quota = rl.defaultQuota
		rl.logger.Debug("Using default quota for user", "user_id", userID)
	}

	// If no quota is set, allow execution (temporary fix for pool architecture)
	if quota == nil {
		rl.logger.Debug("No quota configured for user, allowing execution", "user_id", userID)
		return true
	}

	// Check if quota is expired
	if quota.ExpiresAt.Before(time.Now()) {
		rl.logger.Debug("User quota expired", "user_id", userID, "expired_at", quota.ExpiresAt)
		return false
	}

	// Get current usage
	usage, exists := rl.usage[userID]
	if !exists {
		// No usage tracked yet, allow execution
		rl.logger.Debug("No usage tracked for user, allowing execution", "user_id", userID)
		return true
	}

	// Check resource-specific limits based on action
	switch action {
	case "start_game", "connect_pty":
		return rl.checkPTYLimit(quota, usage)
	case "login", "register":
		return rl.checkConnectionLimit(quota, usage)
	case "stream_data":
		return rl.checkBandwidthLimit(quota, usage)
	default:
		return rl.checkGeneralLimits(quota, usage)
	}
}

// TrackResourceUsage tracks resource usage for a user/connection
func (rl *ResourceLimiter) TrackResourceUsage(userID, connectionID string, resourceType ResourceType, amount int64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	usageKey := rl.getUsageKey(userID, connectionID)
	usage, exists := rl.usage[usageKey]
	if !exists {
		usage = &ResourceUsage{
			UserID:       userID,
			ConnectionID: connectionID,
			Usage:        make(map[ResourceType]*UsageMetric),
			CreatedAt:    time.Now(),
		}
		rl.usage[usageKey] = usage
	}

	usage.LastUpdated = time.Now()

	// Update resource metric
	metric, exists := usage.Usage[resourceType]
	if !exists {
		metric = &UsageMetric{
			LastReset: time.Now(),
		}
		usage.Usage[resourceType] = metric
	}

	// Update current usage
	metric.Current += amount
	if metric.Current > metric.Peak {
		metric.Peak = metric.Current
	}

	// Update average
	metric.Samples++
	metric.Average = (metric.Average*float64(metric.Samples-1) + float64(metric.Current)) / float64(metric.Samples)

	rl.logger.Debug("Tracked resource usage",
		"user_id", userID,
		"connection_id", connectionID,
		"resource_type", resourceType.String(),
		"amount", amount,
		"current_total", metric.Current,
		"peak", metric.Peak,
		"average", metric.Average)

	// Update system-wide limits
	if limit, exists := rl.limits[resourceType]; exists {
		limit.CurrentUsed += amount
		rl.checkSystemLimit(resourceType, limit)
	}
}

// ReleaseResourceUsage releases resource usage for a user/connection
func (rl *ResourceLimiter) ReleaseResourceUsage(userID, connectionID string, resourceType ResourceType, amount int64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	usageKey := rl.getUsageKey(userID, connectionID)
	usage, exists := rl.usage[usageKey]
	if !exists {
		rl.logger.Warn("Attempted to release usage for non-existent user/connection",
			"user_id", userID,
			"connection_id", connectionID)
		return
	}

	metric, exists := usage.Usage[resourceType]
	if !exists {
		rl.logger.Warn("Attempted to release usage for non-tracked resource",
			"user_id", userID,
			"connection_id", connectionID,
			"resource_type", resourceType.String())
		return
	}

	// Update current usage
	metric.Current -= amount
	if metric.Current < 0 {
		metric.Current = 0
	}

	usage.LastUpdated = time.Now()

	rl.logger.Debug("Released resource usage",
		"user_id", userID,
		"connection_id", connectionID,
		"resource_type", resourceType.String(),
		"amount", amount,
		"current_total", metric.Current)

	// Update system-wide limits
	if limit, exists := rl.limits[resourceType]; exists {
		limit.CurrentUsed -= amount
		if limit.CurrentUsed < 0 {
			limit.CurrentUsed = 0
		}
	}
}

// SetUserQuota sets a custom quota for a user
func (rl *ResourceLimiter) SetUserQuota(userID string, quota *ResourceQuota) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	quota.UserID = userID
	rl.quotas[userID] = quota

	rl.logger.Info("Set user quota",
		"user_id", userID,
		"max_connections", quota.MaxConnections,
		"max_ptys", quota.MaxPTYs,
		"max_bandwidth", quota.MaxBandwidth,
		"max_memory", quota.MaxMemory,
		"expires_at", quota.ExpiresAt,
		"priority", quota.Priority)
}

// GetUserUsage returns current usage for a user
func (rl *ResourceLimiter) GetUserUsage(userID string) (*ResourceUsage, bool) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	// Find all usage entries for this user
	for _, usage := range rl.usage {
		if usage.UserID == userID {
			return usage, true
		}
	}

	return nil, false
}

// GetMetrics returns current resource limiter metrics
func (rl *ResourceLimiter) GetMetrics() ResourceMetrics {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	metrics := *rl.metrics
	metrics.TotalLimits = int64(len(rl.limits))
	metrics.ActiveQuotas = int64(len(rl.quotas))

	// Calculate resource utilization
	for resourceType, limit := range rl.limits {
		if limit.MaxValue > 0 {
			metrics.ResourceUtilization[resourceType] = float64(limit.CurrentUsed) / float64(limit.MaxValue)
		}
	}

	return metrics
}

// Helper methods

// getUsageKey generates a key for usage tracking
func (rl *ResourceLimiter) getUsageKey(userID, connectionID string) string {
	if connectionID != "" {
		return fmt.Sprintf("%s:%s", userID, connectionID)
	}
	return userID
}

// checkPTYLimit checks if user can create a new PTY
func (rl *ResourceLimiter) checkPTYLimit(quota *ResourceQuota, usage *ResourceUsage) bool {
	metric, exists := usage.Usage[ResourceTypePTYs]
	if !exists {
		return true // No usage yet
	}

	if metric.Current >= quota.MaxPTYs {
		rl.logger.Debug("PTY limit exceeded",
			"current", metric.Current,
			"limit", quota.MaxPTYs)
		return false
	}

	return true
}

// checkConnectionLimit checks if user can create a new connection
func (rl *ResourceLimiter) checkConnectionLimit(quota *ResourceQuota, usage *ResourceUsage) bool {
	metric, exists := usage.Usage[ResourceTypeConnections]
	if !exists {
		return true // No usage yet
	}

	if metric.Current >= quota.MaxConnections {
		rl.logger.Debug("Connection limit exceeded",
			"current", metric.Current,
			"limit", quota.MaxConnections)
		return false
	}

	return true
}

// checkBandwidthLimit checks if user can use more bandwidth
func (rl *ResourceLimiter) checkBandwidthLimit(quota *ResourceQuota, usage *ResourceUsage) bool {
	metric, exists := usage.Usage[ResourceTypeBandwidth]
	if !exists {
		return true // No usage yet
	}

	// For bandwidth, check rate over time
	now := time.Now()
	timeSinceReset := now.Sub(metric.LastReset)
	if timeSinceReset >= time.Second {
		// Reset bandwidth counter every second
		metric.Current = 0
		metric.LastReset = now
		return true
	}

	if metric.Current >= quota.MaxBandwidth {
		rl.logger.Debug("Bandwidth limit exceeded",
			"current", metric.Current,
			"limit", quota.MaxBandwidth)
		return false
	}

	return true
}

// checkGeneralLimits checks general resource limits
func (rl *ResourceLimiter) checkGeneralLimits(quota *ResourceQuota, usage *ResourceUsage) bool {
	// Check memory limit
	if memMetric, exists := usage.Usage[ResourceTypeMemory]; exists {
		if memMetric.Current >= quota.MaxMemory {
			rl.logger.Debug("Memory limit exceeded",
				"current", memMetric.Current,
				"limit", quota.MaxMemory)
			return false
		}
	}

	return true
}

// checkSystemLimit checks if system-wide limit is exceeded
func (rl *ResourceLimiter) checkSystemLimit(resourceType ResourceType, limit *Limit) {
	if limit.CurrentUsed > limit.MaxValue {
		rl.logger.Warn("System resource limit exceeded",
			"resource_type", resourceType.String(),
			"current", limit.CurrentUsed,
			"limit", limit.MaxValue,
			"burst_limit", limit.BurstLimit)

		rl.metrics.ViolatedLimits++

		// Check if we're exceeding burst limit
		if limit.CurrentUsed > limit.BurstLimit {
			rl.logger.Error("System resource burst limit exceeded",
				"resource_type", resourceType.String(),
				"current", limit.CurrentUsed,
				"burst_limit", limit.BurstLimit)
		}
	}
}

// Cleanup and monitoring routines

// cleanupRoutine performs periodic cleanup of old usage data
func (rl *ResourceLimiter) cleanupRoutine(ctx context.Context) {
	defer rl.wg.Done()

	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	rl.logger.Debug("Resource limiter cleanup routine started", "interval", rl.cleanupInterval)

	for {
		select {
		case <-ticker.C:
			rl.performCleanup()
		case <-ctx.Done():
			rl.logger.Debug("Resource limiter cleanup routine stopped due to context cancellation")
			return
		case <-rl.shutdownChan:
			rl.logger.Debug("Resource limiter cleanup routine stopped")
			return
		}
	}
}

// performCleanup removes old usage data and expired quotas
func (rl *ResourceLimiter) performCleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	var cleanedUsage, cleanedQuotas int

	// Clean up old usage data
	for key, usage := range rl.usage {
		if now.Sub(usage.LastUpdated) > rl.maxIdleTime {
			delete(rl.usage, key)
			cleanedUsage++
		}
	}

	// Clean up expired quotas
	for userID, quota := range rl.quotas {
		if quota.ExpiresAt.Before(now) {
			delete(rl.quotas, userID)
			cleanedQuotas++
		}
	}

	if cleanedUsage > 0 || cleanedQuotas > 0 {
		rl.logger.Info("Performed cleanup",
			"cleaned_usage_entries", cleanedUsage,
			"cleaned_quotas", cleanedQuotas,
			"remaining_usage_entries", len(rl.usage),
			"remaining_quotas", len(rl.quotas))
	}
}

// metricsRoutine collects and logs resource metrics
func (rl *ResourceLimiter) metricsRoutine(ctx context.Context) {
	defer rl.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	rl.logger.Debug("Resource limiter metrics routine started")

	for {
		select {
		case <-ticker.C:
			rl.logMetrics()
		case <-ctx.Done():
			rl.logger.Debug("Resource limiter metrics routine stopped due to context cancellation")
			return
		case <-rl.shutdownChan:
			rl.logger.Debug("Resource limiter metrics routine stopped")
			return
		}
	}
}

// logMetrics logs current resource limiter metrics
func (rl *ResourceLimiter) logMetrics() {
	metrics := rl.GetMetrics()

	rl.logger.Info("Resource limiter metrics",
		"total_limits", metrics.TotalLimits,
		"active_quotas", metrics.ActiveQuotas,
		"violated_limits", metrics.ViolatedLimits,
		"throttled_users", metrics.ThrottledUsers)

	// Log resource utilization
	for resourceType, utilization := range metrics.ResourceUtilization {
		if utilization > 0 {
			rl.logger.Debug("Resource utilization",
				"resource_type", resourceType.String(),
				"utilization", fmt.Sprintf("%.2f%%", utilization*100))
		}
	}
}

// limitResetRoutine resets periodic limits
func (rl *ResourceLimiter) limitResetRoutine(ctx context.Context) {
	defer rl.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	rl.logger.Debug("Resource limiter reset routine started")

	for {
		select {
		case <-ticker.C:
			rl.resetPeriodicLimits()
		case <-ctx.Done():
			rl.logger.Debug("Resource limiter reset routine stopped due to context cancellation")
			return
		case <-rl.shutdownChan:
			rl.logger.Debug("Resource limiter reset routine stopped")
			return
		}
	}
}

// resetPeriodicLimits resets limits that have a reset period
func (rl *ResourceLimiter) resetPeriodicLimits() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	for resourceType, limit := range rl.limits {
		if limit.ResetPeriod > 0 && now.Sub(limit.LastReset) >= limit.ResetPeriod {
			oldUsed := limit.CurrentUsed
			limit.CurrentUsed = 0
			limit.LastReset = now

			if oldUsed > 0 {
				rl.logger.Debug("Reset periodic limit",
					"resource_type", resourceType.String(),
					"previous_usage", oldUsed,
					"reset_period", limit.ResetPeriod)
			}
		}
	}
}

// String method for ResourceType
func (rt ResourceType) String() string {
	switch rt {
	case ResourceTypeConnections:
		return "Connections"
	case ResourceTypePTYs:
		return "PTYs"
	case ResourceTypeFileDescriptors:
		return "FileDescriptors"
	case ResourceTypeMemory:
		return "Memory"
	case ResourceTypeBandwidth:
		return "Bandwidth"
	case ResourceTypeCPU:
		return "CPU"
	case ResourceTypeWorkItems:
		return "WorkItems"
	default:
		return "Unknown"
	}
}