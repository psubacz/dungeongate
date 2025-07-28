// Package config provides game configuration and path management for DungeonGate
package config

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// Manager is the main interface for game configuration management
type Manager struct {
	directoryManager *GameDirectoryManager
	validator        *GameConfigValidator
	cache            *GameConfigCache
	logger           *log.Logger
}

// NewManager creates a new configuration manager
func NewManager(baseDir string, logger *log.Logger) *Manager {
	return &Manager{
		directoryManager: NewGameDirectoryManager(baseDir, logger),
		validator:        NewGameConfigValidator(logger),
		cache:            NewGameConfigCache(time.Hour), // 1 hour cache expiry
		logger:           logger,
	}
}

// SetupGame sets up configuration for a new game
func (m *Manager) SetupGame(userID int, gameID string, options *GameSetupOptions) (*NetHackPaths, error) {
	// Check cache first
	cacheKey := m.generateCacheKey(userID, gameID)
	if cached, found := m.cache.GetConfig(cacheKey); found {
		m.logger.Printf("Using cached config for user %d, game %s", userID, gameID)
		return cached, nil
	}

	// Setup paths
	paths, err := m.directoryManager.SetupGamePaths(userID, gameID, options)
	if err != nil {
		return nil, err
	}

	// Validate configuration if requested
	if options.ValidatePaths {
		if err := m.validator.ValidateConfig(paths); err != nil {
			return nil, err
		}
	}

	// Cache the configuration
	m.cache.SetConfig(cacheKey, paths)

	m.logger.Printf("Successfully set up game config for user %d, game %s", userID, gameID)
	return paths, nil
}

// CleanupGame cleans up configuration for a game
func (m *Manager) CleanupGame(gameID string, options *GameCleanupOptions) error {
	return m.directoryManager.CleanupGame(gameID, options)
}

// ValidateConfig validates a game configuration
func (m *Manager) ValidateConfig(paths *NetHackPaths) error {
	return m.validator.ValidateConfig(paths)
}

// DetectSystemPaths detects NetHack system paths
func (m *Manager) DetectSystemPaths() (*NetHackSystemPaths, error) {
	detector := NewPathDetector()
	return detector.DetectNetHackPaths()
}

// GetUserDirs returns directory structure for a user
func (m *Manager) GetUserDirs(userID int) *UserGameDirs {
	return m.directoryManager.GetUserDirs(userID)
}

// CleanupExpiredTempDirs removes expired temporary directories
func (m *Manager) CleanupExpiredTempDirs(maxAge time.Duration) error {
	return m.directoryManager.CleanupExpiredTempDirs(maxAge)
}

// generateCacheKey generates a cache key for user and game
func (m *Manager) generateCacheKey(userID int, gameID string) string {
	return fmt.Sprintf("user_%d_game_%s", userID, gameID)
}

// GameConfigCache manages configuration caching
type GameConfigCache struct {
	cache         map[string]*CachedConfig
	cacheMutex    sync.RWMutex
	cacheExpiry   time.Duration
	evictionQueue []*CacheEntry
}

// NewGameConfigCache creates a new configuration cache
func NewGameConfigCache(expiry time.Duration) *GameConfigCache {
	gcc := &GameConfigCache{
		cache:       make(map[string]*CachedConfig),
		cacheExpiry: expiry,
	}

	// Start background cache cleanup
	go gcc.startCacheCleanup()

	return gcc
}

// GetConfig retrieves a configuration from cache
func (gcc *GameConfigCache) GetConfig(cacheKey string) (*NetHackPaths, bool) {
	gcc.cacheMutex.RLock()
	defer gcc.cacheMutex.RUnlock()

	cached, exists := gcc.cache[cacheKey]
	if !exists {
		return nil, false
	}

	// Check expiry
	if time.Now().After(cached.ExpiresAt) {
		return nil, false
	}

	// Update hit count
	cached.HitCount++

	return cached.Config, true
}

// SetConfig stores a configuration in cache
func (gcc *GameConfigCache) SetConfig(cacheKey string, config *NetHackPaths) {
	gcc.cacheMutex.Lock()
	defer gcc.cacheMutex.Unlock()

	now := time.Now()
	expiresAt := now.Add(gcc.cacheExpiry)

	gcc.cache[cacheKey] = &CachedConfig{
		Config:    config,
		CachedAt:  now,
		ExpiresAt: expiresAt,
		HitCount:  0,
	}

	// Add to eviction queue
	gcc.evictionQueue = append(gcc.evictionQueue, &CacheEntry{
		Key:       cacheKey,
		ExpiresAt: expiresAt,
	})
}

// startCacheCleanup starts the background cache cleanup routine
func (gcc *GameConfigCache) startCacheCleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		gcc.cleanupExpiredEntries()
	}
}

// cleanupExpiredEntries removes expired cache entries
func (gcc *GameConfigCache) cleanupExpiredEntries() {
	gcc.cacheMutex.Lock()
	defer gcc.cacheMutex.Unlock()

	now := time.Now()
	validEntries := make([]*CacheEntry, 0, len(gcc.evictionQueue))

	for _, entry := range gcc.evictionQueue {
		if now.After(entry.ExpiresAt) {
			// Remove from cache
			delete(gcc.cache, entry.Key)
		} else {
			// Keep valid entry
			validEntries = append(validEntries, entry)
		}
	}

	gcc.evictionQueue = validEntries
}

// GetCacheStats returns cache statistics
func (gcc *GameConfigCache) GetCacheStats() map[string]any {
	gcc.cacheMutex.RLock()
	defer gcc.cacheMutex.RUnlock()

	totalHits := int64(0)
	for _, cached := range gcc.cache {
		totalHits += cached.HitCount
	}

	return map[string]any{
		"total_entries": len(gcc.cache),
		"total_hits":    totalHits,
		"queue_size":    len(gcc.evictionQueue),
	}
}
